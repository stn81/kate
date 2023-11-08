package log

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxMarker struct{}

var (
	ctxMarkerKey = &ctxMarker{}
	nullLogger   = zap.NewNop()
)

// GetLogger retrieve the logger in context.
func GetLogger(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(ctxMarkerKey).(*zap.Logger); ok {
		return logger
	}

	return nullLogger
}

// ToContext adds the zap.Logger to the context for extraction later.
func ToContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxMarkerKey, logger)
}

// With append logging fields to context
func With(ctx context.Context, fields ...zapcore.Field) context.Context {
	return ToContext(ctx, GetLogger(ctx).With(fields...))
}
