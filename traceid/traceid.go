package traceid

import (
	"context"

	"github.com/stn81/kate/utils"
)

type ctxMarker struct{}

var ctxMarkerKey = &ctxMarker{}

func New() string {
	return utils.FastUUIDStr()
}

func Extract(ctx context.Context) string {
	if traceID, ok := ctx.Value(ctxMarkerKey).(string); ok {
		return traceID
	}
	return ""
}

func ToContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, traceID)
}
