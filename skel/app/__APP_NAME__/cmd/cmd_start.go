package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/cloudflare/tableflip"
	"github.com/spf13/cobra"
	"github.com/stn81/kate/app"
	"github.com/stn81/kate/log"
	"github.com/stn81/kate/log/encoders/simple"
	"github.com/stn81/kate/rdb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"__PACKAGE_NAME__/config"
	"__PACKAGE_NAME__/grpcsrv"
	"__PACKAGE_NAME__/httpsrv"
	"__PACKAGE_NAME__/model"
	"__PACKAGE_NAME__/profiling"
)

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start server",
		Run:   startCmdFunc,
	}
	return cmd
}

func startCmdFunc(_ *cobra.Command, _ []string) {
	_ = os.Chdir(app.GetHomeDir())

	// load config
	if err := config.Load(GlobalFlags.ConfigFile); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load config failed: file=%s, error=%v\n", GlobalFlags.ConfigFile, err)
	}

	logger := initLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to flush log: %v", err)
		}
	}()

	// update pid
	if err := app.UpdatePidFile(config.Main.PidFile); err != nil {
		logger.Fatal("update pid failed", zap.Error(err))
	}

	app.LogVersion(logger)

	defer func() {
		if r := recover(); r != nil {
			logger.Fatal("panic", zap.Any("error", r), zap.Stack("stack"))
		}

		app.RemovePidFile()
		logger.Info("server stopped")
	}()

	// setup profiling
	if config.Profiling.Enabled {
		profiling.Start(config.Profiling.Port, logger)
	}

	logger.Info("server starting")

	rdb.Init(config.Redis.Config)
	defer rdb.Uninit()

	model.Init(logger)

	// setup upgrader to support zero-downtime upgrade/restart
	upgrader, err := tableflip.New(tableflip.Options{
		PIDFile:        app.GetPidFile(),
		UpgradeTimeout: time.Minute * 10,
	})
	if err != nil {
		logger.Fatal("failed to create upgrader", zap.Error(err))
	}
	defer upgrader.Stop()

	grpcsrv.Start(upgrader, logger)
	defer grpcsrv.Stop()

	httpsrv.Start(upgrader, logger)
	defer httpsrv.Stop()

	logger.Info("server started")

	waitForShutdown(upgrader, logger)
}

func initLogger() *zap.Logger {
	enc := simple.NewEncoder()
	core := zapcore.NewTee(
		log.MustNewCoreWithLevelAbove(zapcore.DebugLevel, path.Join(config.Main.LogDir, "__APP_NAME__.all.log"), enc),
		log.MustNewCoreWithLevelOnly(zapcore.DebugLevel, path.Join(config.Main.LogDir, "__APP_NAME__.debug.log"), enc),
		log.MustNewCoreWithLevelOnly(zapcore.InfoLevel, path.Join(config.Main.LogDir, "__APP_NAME__.info.log"), enc),
		log.MustNewCoreWithLevelOnly(zapcore.WarnLevel, path.Join(config.Main.LogDir, "__APP_NAME__.warn.log"), enc),
		log.MustNewCoreWithLevelOnly(zapcore.ErrorLevel, path.Join(config.Main.LogDir, "__APP_NAME__.error.log"), enc),
		log.MustNewCoreWithLevelOnly(zapcore.FatalLevel, path.Join(config.Main.LogDir, "__APP_NAME__.fatal.log"), enc),
	)

	opts := []zap.Option{
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCaller(),
	}

	logger := zap.New(core, opts...)
	zap.ReplaceGlobals(logger)

	return logger
}

func waitForShutdown(upgrader *tableflip.Upgrader, logger *zap.Logger) {
	defer func() {
		logger.Info("server shutting down ...")
	}()

	if err := upgrader.Ready(); err != nil {
		logger.Fatal("upgrader ready failed", zap.Error(err))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(
		sigCh,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGTTIN,
	)

	for {
		select {
		case sig := <-sigCh:
			logger.Info("got signal", zap.Any("signal", sig))

			switch sig {
			case syscall.SIGINT:
				return
			case syscall.SIGQUIT:
				return
			case syscall.SIGTERM:
				return
			case syscall.SIGHUP:
				err := upgrader.Upgrade()
				if err != nil {
					logger.Error("upgrade failed", zap.Error(err))
				}
			}
		case <-upgrader.Exit():
			logger.Info("upgrader exit")
			return
		}
	}
}
