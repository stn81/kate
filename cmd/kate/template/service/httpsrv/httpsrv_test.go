package httpsrv

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stn81/kate"
	"go.uber.org/zap"

	"github.com/stn81/kate/cmd/kate/template/service/config"
)

// 全链路打 /hello：真实路由 + 中间件链（TraceId/Logging/Recovery/Timeout/CORS），
// 防"能编译但跑不起来"。不依赖任何可选组件，裁剪后仍成立。
func TestHelloRoute(t *testing.T) {
	if err := config.Load("../scripts/conf/dev.ini"); err != nil {
		t.Fatalf("load config: %v", err)
	}
	logger := zap.NewNop()
	s := &httpService{conf: *config.HTTP, logger: logger, accessLogger: logger}

	router := kate.NewRESTRouter(context.Background(), logger)
	router.SetMaxBodyBytes(s.conf.MaxBodyBytes)
	s.setupRoutes(router)

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatalf("GET /hello: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var result kate.Result
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("body not errno envelope: %v: %s", err, body)
	}
	if result.ErrNO != 0 || result.Data != "hello world" {
		t.Errorf("envelope = %+v, want errno 0 + hello world", result)
	}
}
