package kate

import (
	"context"
	"time"
)

func Timeout(timeout time.Duration) Middleware {
	mf := func(h ContextHandler) ContextHandler {
		f := func(ctx context.Context, w ResponseWriter, r *Request) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			h.ServeHTTP(ctx, w, r)
		}
		return ContextHandlerFunc(f)
	}
	return MiddlewareFunc(mf)
}
