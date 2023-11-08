package kate

import (
	"context"
	"github.com/stn81/kate/log"
	"net/http"

	"go.uber.org/zap"
)

// Recovery implements the recovery wrapper middleware
var Recovery = MiddlewareFunc(recoveryFunc)

func recoveryFunc(h ContextHandler) ContextHandler {
	f := func(ctx context.Context, w ResponseWriter, r *Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(http.StatusText(http.StatusInternalServerError)))
				log.GetLogger(ctx).Error("got panic", zap.Any("error", err), zap.Stack("stack"))
			}
		}()

		h.ServeHTTP(ctx, w, r)
	}
	return ContextHandlerFunc(f)
}
