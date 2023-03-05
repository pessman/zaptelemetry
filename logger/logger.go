package logger

import (
	"context"

	"go.uber.org/zap"
)

// New creates new zaptelemetry logger
func New(ctx context.Context) *zap.Logger {
	l, _ := zap.NewProduction()
	return zap.New(&zapCtxCore{core: l.Core(), context: ctx})
}
