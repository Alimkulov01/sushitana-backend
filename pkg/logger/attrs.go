package logger

import (
	"context"

	"go.uber.org/zap"
)

const (
	logIDKey    = "logID"
	durationKey = "duration"
	requestKey  = "request"
)

func getAttrs(ctx context.Context) []zap.Field {
	lgCtx, _ := ctx.Value(&logCtx).(*logContext)
	if lgCtx == nil {
		return nil
	}

	return lgCtx.ToFields()
}
