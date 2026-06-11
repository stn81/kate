package httpsrv

import (
	"context"
	"net/http"

	"github.com/stn81/kate"
)

// OptionsHandler 统一应答 CORS 预检请求（与 CORS 中间件配合）。
type OptionsHandler struct {
	kate.BaseHandler
}

func (h *OptionsHandler) ServeHTTP(ctx context.Context, w kate.ResponseWriter, r *kate.Request) {
	w.WriteHeader(http.StatusOK)
}

// setupRoutes 注册所有路由。新增接口在此追加。
func (s *httpService) setupRoutes(router *kate.RESTRouter) {
	// cBase 是基础中间件链：TraceId / Logging / Recovery / Timeout / CORS。
	// 业务可在其上 Append 自己的中间件（如鉴权），生成新的链复用。
	cBase := kate.NewChain(
		kate.TraceId,
		kate.Logging(s.accessLogger),
		kate.Recovery,
		kate.Timeout(s.conf.HandleTimeout),
		kate.CORS(86400),
	)

	router.OPTIONS("/*path", cBase.Then(&OptionsHandler{}))
	// k8s 探针（HTTP 状态码语义）：livez 恒 200（不查依赖，liveness 失败会触发重启）；
	// readyz 逐项 ping 依赖，任一失败真实 503（摘流量不重启）。
	router.GET("/livez", cBase.Then(&LivenessHandler{}))
	router.GET("/readyz", cBase.Then(&ReadinessHandler{}))
	router.GET("/hello", cBase.Then(&HelloHandler{}))
}
