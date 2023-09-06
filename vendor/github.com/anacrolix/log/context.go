package log

import (
	"context"
)

var loggerContextKey interface{} = (*Logger)(nil)

func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

func ContextLogger(ctx context.Context) Logger {
	value := ctx.Value(loggerContextKey)
	if value == nil {
		return Default
	}
	return value.(Logger)
}
