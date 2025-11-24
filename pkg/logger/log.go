package logger

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func (l *logger) Context(ctx context.Context) context.Context {
	_, ok := ctx.Value(&logCtx).(*logContext)
	if ok {
		return ctx
	}

	lgCtx := newLogContext(l.idGenerator.NewLogID(ctx))
	ctx = context.WithValue(ctx, &logCtx, lgCtx)
	return ctx
}

func (l *logger) ContextWithCapture(ctx context.Context, operationName string) (context.Context, Capture) {
	lgCtx, ok := ctx.Value(&logCtx).(*logContext)
	if !ok {
		lgCtx = newLogContextWithOptions(l.idGenerator.NewLogID(ctx), withOperationName(operationName))
	}

	lgCtx = newLogContextWithOptions(lgCtx.LogID, withOperationName(operationName))
	ctx = context.WithValue(ctx, &logCtx, lgCtx)

	return ctx, l.captureContext(lgCtx)
}

func (l *logger) captureContext(logCtx *logContext) Capture {
	return func(attrs ...zap.Field) {
		l.lg.Desugar().With(attrs...).Info(logCtx.OperationName,
			zap.String(logIDKey, logCtx.LogID.String()),
			zap.String(durationKey, time.Since(time.Time(logCtx.StartTime)).String()),
		)
	}
}

func (l *logger) Debug(ctx context.Context, log string, fields ...zapcore.Field) {
	if ctx != nil {
		fields = append(fields, getAttrs(ctx)...)
	}
	l.lg.Desugar().Debug(log, fields...)
}

func (l *logger) Info(ctx context.Context, log string, fields ...zapcore.Field) {
	if ctx != nil {
		fields = append(fields, getAttrs(ctx)...)
	}
	l.lg.Desugar().Info(log, fields...)
}

func (l *logger) Warn(ctx context.Context, log string, fields ...zapcore.Field) {
	if ctx != nil {
		fields = append(fields, getAttrs(ctx)...)
	}
	l.lg.Desugar().Warn(log, fields...)
}

func (l *logger) Error(ctx context.Context, log string, fields ...zapcore.Field) {
	if ctx != nil {
		fields = append(fields, getAttrs(ctx)...)
	}
	l.lg.Desugar().Error(log, fields...)
}
