package httpsrv

import (
	"context"
	"net/http"

	"github.com/stn81/kate"
	//kate:begin redis
	"github.com/stn81/kate/rdb"
	//kate:end redis

	//kate:begin mysql
	"github.com/stn81/kate/cmd/kate/template/service/model"
	//kate:end mysql
)

// LivenessHandler 恒 200：进程活着即健康，不依赖任何外部组件（k8s liveness 失败会触发重启，
// 依赖抖动不应连坐进程）。挂 /livez。
type LivenessHandler struct {
	kate.RESTHandler
}

func (h *LivenessHandler) ServeHTTP(ctx context.Context, w kate.ResponseWriter, r *kate.Request) {
	h.OkData(ctx, w, map[string]string{"status": "ok"})
}

// ReadinessHandler 是 readiness（挂 /readyz）：逐项 ping 依赖，任一失败返回真实 HTTP 503
//（k8s readiness / LB 摘流可直接用）。新增依赖时在此追加检查。
type ReadinessHandler struct {
	kate.RESTHandler
}

func (h *ReadinessHandler) ServeHTTP(ctx context.Context, w kate.ResponseWriter, r *kate.Request) {
	//kate:begin mysql
	if model.DB != nil {
		if err := model.DB.PingContext(ctx); err != nil {
			h.fail(ctx, w, "mysql", err)
			return
		}
	}
	//kate:end mysql
	//kate:begin redis
	if c := rdb.Get(); c != nil {
		if err := c.Ping(ctx).Err(); err != nil {
			h.fail(ctx, w, "redis", err)
			return
		}
	}
	//kate:end redis
	h.OkData(ctx, w, map[string]string{"status": "ok"})
}

// fail 以真实 HTTP 503 报告未就绪原因（body 仍是 errno envelope）。
func (h *ReadinessHandler) fail(ctx context.Context, w kate.ResponseWriter, component string, err error) {
	h.Error(ctx, w, kate.NewHTTPError(http.StatusServiceUnavailable, 503, component+": "+err.Error()))
}
