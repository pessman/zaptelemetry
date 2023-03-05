package logger

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	traceIDKey = "traceId"
	spanIDKey  = "spanId"
)

// confirm custom core meets zapcore.Core interface
var _ zapcore.Core = (*zapCtxCore)(nil)

type zapCtxCore struct {
	core    zapcore.Core
	context context.Context
}

// Check determines whether the supplied Entry should be logged (using the
// embedded LevelEnabler and possibly some extra logic). If the entry
// should be logged, the Core adds itself to the CheckedEntry and returns
// the result.
//
// Callers must use Check before calling Write.
func (zcc *zapCtxCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if zcc.Enabled(entry.Level) {
		return checked.AddCore(entry, zcc)
	}
	return zcc.core.Check(entry, checked)
}

// LevelEnabler decides whether a given logging level is enabled when logging a
// message.
//
// Enablers are intended to be used to implement deterministic filters;
// concerns like sampling are better implemented as a Core.
//
// Each concrete Level value implements a static LevelEnabler which returns
// true for itself and all higher logging levels. For example WarnLevel.Enabled()
// will return true for WarnLevel, ErrorLevel, DPanicLevel, PanicLevel, and
// FatalLevel, but return false for InfoLevel and DebugLevel.
func (zcc *zapCtxCore) Enabled(level zapcore.Level) bool {
	return zcc.core.Enabled(level)
}

// Sync flushes buffered logs (if any).
func (zcc *zapCtxCore) Sync() error {
	return zcc.core.Sync()
}

// With adds structured context to the Core.
func (zcc *zapCtxCore) With(fields []zapcore.Field) zapcore.Core {
	newCore := zcc.core.With(fields)
	return &zapCtxCore{
		core:    newCore,
		context: zcc.context,
	}
}

// Write serializes the Entry and any Fields supplied at the log site and
// writes them to their destination.
//
// If called, Write should always log the Entry and Fields; it should not
// replicate the logic of Check.
func (zcc *zapCtxCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if zcc.context == nil {
		return zcc.core.Write(entry, fields)
	}

	span := trace.SpanFromContext(zcc.context)
	if !span.IsRecording() {
		return nil
	}

	// add trace info to logging fields
	spanCtx := span.SpanContext()
	fields = append(fields, zap.String(traceIDKey, spanCtx.TraceID().String()))
	fields = append(fields, zap.String(spanIDKey, spanCtx.SpanID().String()))

	// add log events to span
	logSeverityAttrKey := attribute.Key("log.severity").String(entry.Level.String())
	logMessageAttrKey := attribute.Key("log.message").String(entry.Message)
	span.AddEvent("log", trace.WithAttributes(logSeverityAttrKey, logMessageAttrKey))
	if entry.Level >= zap.ErrorLevel {
		span.SetStatus(codes.Error, entry.Message)
	}

	return zcc.core.Write(entry, fields)
}
