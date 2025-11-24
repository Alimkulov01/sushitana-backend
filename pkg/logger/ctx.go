package logger

import (
	"bytes"
	"encoding/hex"
	"time"

	"go.uber.org/zap"
)

type (
	logCtxKey struct{}
)

var logCtx logCtxKey

type StartTime time.Time

type LogID [8]byte

func (lid LogID) String() string {
	return hex.EncodeToString(lid[:])
}

var nilLogID = LogID{}

func (lid LogID) IsValid() bool {
	return !bytes.Equal(lid[:], nilLogID[:])
}

type logContext struct {
	StartTime     StartTime
	RequestID     string
	OperationName string
	LogID         LogID
}

func (lgCtx *logContext) ToFields() []zap.Field {
	if lgCtx == nil {
		return nil
	}

	//nolint:mnd // guide go slice cap
	attrs := make([]zap.Field, 0, 2)
	attrs = append(attrs, zap.String(logIDKey, lgCtx.LogID.String()))

	if lgCtx.RequestID != "" {
		attrs = append(attrs, zap.String(requestKey, lgCtx.RequestID))
	}
	return attrs
}

type logContextOptions struct {
	ServiceName   string
	RequestID     string
	OperationName string
}

type logContextOption func(*logContextOptions)

//nolint:unused // future method
func withRequestID(requestID string) logContextOption {
	return func(o *logContextOptions) {
		o.RequestID = requestID
	}
}

func withOperationName(operationName string) logContextOption {
	return func(o *logContextOptions) {
		o.OperationName = operationName
	}
}

func newLogContext(logID LogID) *logContext {
	return newLogContextWithOptions(logID)
}

func newLogContextWithOptions(logID LogID, opts ...logContextOption) *logContext {
	options := &logContextOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return &logContext{
		LogID:         logID,
		RequestID:     options.RequestID,
		OperationName: options.OperationName,
		StartTime:     StartTime(time.Now()),
	}
}
