package logging

import (
	"context"
	"os"

	"go.uber.org/zap/zapcore"

	"go.uber.org/zap"
)

type loggerContextKey struct{}

func DefaultLogger(options ...zap.Option) (*zap.Logger, error) {
	if os.Getenv("DEV_LOGGING") == "true" {
		return zap.NewDevelopment(options...)
	}

	return zap.NewProduction(options...)
}

func WithDefaultLogger(ctx context.Context, options ...zap.Option) (context.Context, error) {
	logger, err := DefaultLogger(options...)
	if err != nil {
		return ctx, err
	}
	return WithLogger(ctx, logger), nil
}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

func FromContext(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(loggerContextKey{}).(*zap.Logger)
	if !ok {
		var err error
		logger, err = DefaultLogger()
		if err != nil {
			return zap.NewNop()
		}
	}

	return logger
}

// FieldOnLevel only returns a field if the logger level is at least `level`.
// The function depends on the assumption that the logger level is not going to change in runtime (only set on startup).
func FieldOnLevel(ctx context.Context, level zapcore.Level, field zap.Field) zap.Field {
	if !FromContext(ctx).Core().Enabled(level) {
		return zap.Skip()
	}

	return field
}
