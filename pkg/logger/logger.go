package logger

import (
	"context"
	"os"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Capture func(attrs ...zap.Field)

type Logger interface {
	Context(ctx context.Context) context.Context
	ContextWithCapture(ctx context.Context, operationName string) (context.Context, Capture)

	Debug(ctx context.Context, log string, fields ...zapcore.Field)
	Info(ctx context.Context, log string, fields ...zapcore.Field)
	Warn(ctx context.Context, log string, fields ...zapcore.Field)
	Error(ctx context.Context, log string, fields ...zapcore.Field)
}

var Module = fx.Provide(func() Logger {
	return New("debug")
})

// New constructs a new logger.
func New(level string) Logger {
	// write syncers
	stdoutSyncer := zapcore.Lock(os.Stdout)

	prodEncoderConfig := zap.NewProductionEncoderConfig()
	prodEncoderConfig.FunctionKey = "func"

	// tee core
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(prodEncoderConfig),
			stdoutSyncer,
			getLevel(level),
		),
	)

	// create log instance with AddCaller option.
	// AddCallerSkip option - skips stack trace where log called
	log := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	l := logger{}
	l.lg = log.Sugar()
	l.idGenerator = defaultIDGenerator()
	return &l
}

type logger struct {
	lg          *zap.SugaredLogger
	idGenerator IDGenerator
}

func getLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.DebugLevel
	}
}
