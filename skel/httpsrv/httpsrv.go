package httpsrv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path"
	"sync"

	"github.com/cloudflare/tableflip"
	"github.com/stn81/kate"
	"github.com/stn81/kate/log"
	"github.com/stn81/kate/log/encoders/simple"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"__PACKAGE_NAME__/config"
)

var gService *httpService

type httpService struct {
	conf         config.HTTPConfig
	upgrader     *tableflip.Upgrader
	listener     net.Listener
	server       *http.Server
	wg           sync.WaitGroup
	logger       *zap.Logger
	accessLogger *zap.Logger
}

// Start the http service
func Start(upgrader *tableflip.Upgrader, logger *zap.Logger) {
	if gService != nil {
		panic("httpsrv start twice")
	}

	gService = &httpService{
		conf:     *config.HTTP,
		upgrader: upgrader,
		logger:   logger.Named("httpsrv"),
	}
	gService.start()
}

// Stop the http service
func Stop() {
	if gService != nil {
		gService.stop()
	}
}

func (s *httpService) start() {
	var err error

	s.initAccessLogger()

	// 定义中间件栈，可根据需要在下面追加
	c := kate.NewChain(
		kate.TraceId,
		kate.Logging(s.accessLogger),
		kate.Recovery,
	)

	// 注册Handler
	router := kate.NewRESTRouter(context.Background(), s.logger)
	router.SetMaxBodyBytes(s.conf.MaxBodyBytes)
	router.GET("/hello", c.Then(&HelloHandler{}))

	// 生成一个http.Server对象
	s.server = &http.Server{
		Addr:           s.conf.Addr,
		Handler:        router,
		ReadTimeout:    s.conf.ReadTimeout,
		WriteTimeout:   s.conf.WriteTimeout,
		MaxHeaderBytes: s.conf.MaxHeaderBytes,
	}

	if s.listener, err = s.upgrader.Listen("tcp", s.conf.Addr); err != nil {
		s.logger.Fatal("http listen failed",
			zap.String("addr", s.conf.Addr),
			zap.Error(err),
		)
	}

	s.wg.Add(1)
	go s.serve()
}

func (s *httpService) serve() {
	defer func() {
		s.wg.Done()
		s.logger.Info("http service stopped")
	}()

	s.logger.Info("http service started listening", zap.String("addr", s.conf.Addr))

	err := s.server.Serve(s.listener)
	switch {
	case errors.Is(err, http.ErrServerClosed):
	case err != nil:
		s.logger.Fatal("failed to serve http service", zap.Error(err))
	}
}

func (s *httpService) stop() {
	if err := s.server.Shutdown(context.TODO()); err != nil {
		s.logger.Error("http service shutdown failed", zap.Error(err))
	}
	s.wg.Wait()
}

func (s *httpService) initAccessLogger() {
	var (
		enc  = simple.NewEncoder()
		core = log.MustNewCoreWithLevelAbove(zapcore.DebugLevel, path.Join(config.Main.LogDir, s.conf.LogFile), enc)
	)

	if s.conf.LogSampler.Enabled {
		core = zapcore.NewSamplerWithOptions(
			core,
			s.conf.LogSampler.Tick,
			s.conf.LogSampler.First,
			s.conf.LogSampler.ThereAfter,
		)
	}

	opts := []zap.Option{
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCaller(),
	}

	s.accessLogger = zap.New(core, opts...)
}
