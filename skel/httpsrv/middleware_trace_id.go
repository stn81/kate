package httpsrv

import (
	"context"

	"github.com/stn81/kate"
	"github.com/stn81/kate/log/ctxzap"
	"github.com/stn81/kate/traceid"
	"go.uber.org/zap"
)

func TraceID(h kate.ContextHandler) kate.ContextHandler {
	f := func(ctx context.Context, w kate.ResponseWriter, r *kate.Request) {
		var (
			traceID = r.Header.Get("X-Trace-ID")
			logger  = ctxzap.Extract(ctx)
		)

		if traceID == "" {
			traceID = traceid.New()
		}

		logger = logger.With(zap.String("trace_id", traceID))
		ctx = traceid.ToContext(ctx, traceID)
		ctx = ctxzap.ToContext(ctx, logger)
		h.ServeHTTP(ctx, w, r)
	}
	return kate.ContextHandlerFunc(f)
}
