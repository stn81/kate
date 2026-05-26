package httpsrv

import (
	"context"
	"github.com/stn81/kate"
)

type HelloHandler struct {
	kate.BaseHandler
}

func (h *HelloHandler) ServeHTTP(ctx context.Context, w kate.ResponseWriter, r *kate.Request) {
	h.OkData(ctx, w, "hello world")
}
