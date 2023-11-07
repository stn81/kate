package traceid

import (
	"context"

	"github.com/stn81/kate/utils"
)

type ctxMarker struct{}

var ctxMarkerKey = &ctxMarker{}

func New() string {
	return utils.FastUuidStr()
}

func Extract(ctx context.Context) string {
	if traceId, ok := ctx.Value(ctxMarkerKey).(string); ok {
		return traceId
	}
	return ""
}

func ToContext(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, traceId)
}
