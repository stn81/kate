package kate

import (
	"context"
	"github.com/stn81/kate/log"
	"github.com/stn81/kate/traceid"
	"go.uber.org/zap"
)

const HeaderTraceId = "X-Trace-Id"

func TraceId(h ContextHandler) ContextHandler {
	f := func(ctx context.Context, w ResponseWriter, r *Request) {
		var (
			traceId = r.Header.Get(HeaderTraceId)
			logger  = log.GetLogger(ctx)
		)

		if traceId == "" {
			traceId = traceid.New()
		}
		w.Header().Set(HeaderTraceId, traceId)

		logger = logger.With(zap.String("trace_id", traceId))
		ctx = traceid.ToContext(ctx, traceId)
		ctx = log.ToContext(ctx, logger)
		h.ServeHTTP(ctx, w, r)
	}
	return ContextHandlerFunc(f)
}
