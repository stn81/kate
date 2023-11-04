package kate

import (
	"context"
	"net/http"

	"github.com/stn81/kate/log/ctxzap"
	"go.uber.org/zap"
)

// Recovery implements the recovery wrapper middleware
func Recovery(h ContextHandler) ContextHandler {
	f := func(ctx context.Context, w ResponseWriter, r *Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
				ctxzap.Extract(ctx).Error("got panic", zap.Any("error", err), zap.Stack("stack"))
			}
		}()

		h.ServeHTTP(ctx, w, r)
	}
	return ContextHandlerFunc(f)
}
