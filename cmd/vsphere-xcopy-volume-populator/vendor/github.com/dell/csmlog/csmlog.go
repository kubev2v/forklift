/*
 *
 * Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 */

package csmlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/metadata"
)

var globalLogInstance *CsmLog

type key int

func init() {
	// Initialize the global logger instance on package import
	if globalLogInstance == nil {
		// Read log format from env var, default to "text"
		logFormat := strings.ToLower(strings.TrimSpace(os.Getenv("CSI_LOG_FORMAT")))
		if logFormat == "" {
			fmt.Println("CSI_LOG_FORMAT is retrieved from applied CSM file, defaulted to 'json'")
			logFormat = "json"
		}

		// Read log level from env var, default to InfoLevel
		logLevelStr := strings.ToLower(strings.TrimSpace(os.Getenv("CSI_LOG_LEVEL")))
		logLevel := InfoLevel
		if logLevelStr != "" {
			if parsedLevel, err := ParseLevel(logLevelStr); err == nil {
				logLevel = parsedLevel
			} else {
				fmt.Printf("Invalid CSI_LOG_LEVEL value '%s', initializing to 'info' default: %v\n", logLevelStr, err)
			}
		} else {
			fmt.Println("CSI_LOG_LEVEL is retrieved from applied CSM file, defaulted to 'info'")
		}

		globalLogInstance = New(logFormat, logLevel, os.Stderr)
	}
}

const (
	// Key for request id in context
	RequestIDKey = "csi.requestid"

	// field for RequestID to be logged
	RequestIDField = "ReqID"

	contextLogFieldsKey key = iota
)

const (
	// Standard structured log field keys
	FieldComponent   = "component"
	FieldOperation   = "operation"
	FieldProtocol    = "protocol"
	FieldArrayID     = "array_id"
	FieldVolumeID    = "volume_id"
	FieldVolumeName  = "volume_name"
	FieldNodeID      = "node_id"
	FieldDurationMs  = "duration_ms"
	FieldError       = "error"
	FieldStagingPath = "staging_path"
	FieldTargetPath  = "target_path"
	FieldDevicePath  = "device_path"

	// Environment variable keys
	EnvLogFormat = "CSM_LOG_FORMAT"
	EnvLogLevel  = "CSM_LOG_LEVEL"
)

type Fields = map[string]interface{}

// cloneFields returns a shallow copy of fields.
//
// This is used to prevent accidental mutation of a Fields map that may be shared
// (for example, when stored on a context and used concurrently). Callers can
// safely add/override keys on the returned map without affecting the original.
func cloneFields(fields Fields) Fields {
	if len(fields) == 0 {
		return Fields{}
	}

	cloned := make(Fields, len(fields))
	for k, v := range fields {
		cloned[k] = v
	}
	return cloned
}

// CsmLog is a structured logger backed by zap that provides timestamp, automatic request ID
// extraction, and custom formatting standardized across the Container Storage Modules solution.
type CsmLog struct {
	sugar  *zap.SugaredLogger
	level  zap.AtomicLevel
	writer zapcore.WriteSyncer
	format string // "json" or "text"
	fields Fields
	mu     sync.RWMutex
}

// csmEncoderConfig returns the shared encoder config for both JSON and text output.
func csmEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "",
		MessageKey:     "msg",
		StacktraceKey:  "",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.RFC3339TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// csmTextEncoder produces the CSM bracket format:
// <UTC timestamp> [LEVEL] [ReqID=xxx] <message>
// [field1=val1] [field2=val2]
type csmTextEncoder struct {
	zapcore.Encoder
	cfg zapcore.EncoderConfig
}

func newCsmTextEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &csmTextEncoder{
		Encoder: zapcore.NewJSONEncoder(cfg), // internal buffer; we override EncodeEntry
		cfg:     cfg,
	}
}

func (e *csmTextEncoder) Clone() zapcore.Encoder {
	return &csmTextEncoder{
		Encoder: e.Encoder.Clone(),
		cfg:     e.cfg,
	}
}

// from ECS01-331, format should be ex:
// 2025-08-25T14:23:05.123Z [INFO] [TraceID=abc123] [RequestID=req789] Volume created successfully
// In other words: Timestamp [LEVEL] [TraceID=id] [RequestID=id] [otherfields=data] .... <message>
func (e *csmTextEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// Delegate to the embedded JSON encoder to capture ALL fields
	// (including those accumulated via sugar.With). Then parse the JSON
	// and reformat into the CSM bracket text format.
	jsonBuf, err := e.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		return jsonBuf, err
	}

	// Parse the JSON to extract fields
	var m map[string]interface{}
	if jsonErr := json.Unmarshal(jsonBuf.Bytes(), &m); jsonErr != nil {
		// fallback: return the JSON as-is
		return jsonBuf, nil
	}
	jsonBuf.Free()

	pool := buffer.NewPool()
	buf := pool.Get()

	timestamp := entry.Time.UTC().Format(time.RFC3339)
	levelStr := csmLevelBracket(entry.Level)

	// extract ReqID
	reqID := ""
	if v, ok := m[RequestIDKey]; ok {
		reqID = fmt.Sprintf("[%s=%v]", RequestIDField, v)
	}

	// build log line
	buf.AppendString(timestamp)
	buf.AppendString(" ")
	buf.AppendString(levelStr)
	buf.AppendString(" ")
	// req ID goes before all other fields
	if reqID != "" {
		buf.AppendString(reqID)
		buf.AppendString(" ")
	}

	// append all other fields
	// skip standard keys when printing extra fields
	skip := map[string]bool{
		"time": true, "level": true, "msg": true, "logger": true,
		RequestIDKey: true,
	}
	fieldStr := ""
	for k, v := range m {
		if skip[k] {
			continue
		}
		fieldStr += fmt.Sprintf("[%s=%v] ", k, v)
	}
	if fieldStr != "" {
		buf.AppendString(fieldStr)
		buf.AppendString(" ") // space after all fields
	}

	// finally, append the log message itself, and end with a newline
	buf.AppendString(entry.Message)
	buf.AppendString("\n")

	return buf, nil
}

func csmLevelBracket(l zapcore.Level) string {
	switch l {
	case zapcore.InfoLevel:
		return " [INFO]"
	case zapcore.WarnLevel:
		return " [WARN]"
	case zapcore.ErrorLevel:
		return "[ERROR]"
	case zapcore.DebugLevel:
		return "[DEBUG]"
	case zapcore.DPanicLevel:
		return "[PANIC]"
	case zapcore.FatalLevel:
		return "[FATAL]"
	default:
		return fmt.Sprintf("[%s]", l.CapitalString())
	}
}

// buildCore creates a zapcore.Core from the current settings.
func buildCore(format string, level zap.AtomicLevel, writer zapcore.WriteSyncer) zapcore.Core {
	cfg := csmEncoderConfig()
	var encoder zapcore.Encoder
	if strings.EqualFold(format, "json") {
		encoder = zapcore.NewJSONEncoder(cfg)
	} else {
		encoder = newCsmTextEncoder(cfg)
	}
	return zapcore.NewCore(encoder, writer, level)
}

func GetLogger() *CsmLog {
	return globalLogInstance
}

// New creates a new CsmLog instance with the specified configuration.
// Parameters:
//   - format: Output format, "json" or "text". If empty, defaults to "text".
//   - level: Log level. If empty, defaults to InfoLevel.
//   - output: io.Writer for log output. If nil, defaults to os.Stderr.
//
// Returns a new CsmLog instance that can be configured independently.
func New(format string, level Level, output io.Writer) *CsmLog {
	if format == "" {
		format = "text"
	}

	zapLevel := csmLevelToZap(level)
	atomicLevel := zap.NewAtomicLevelAt(zapLevel)

	var writer zapcore.WriteSyncer
	if output != nil {
		writer = zapcore.Lock(zapcore.AddSync(output))
	} else {
		writer = zapcore.Lock(zapcore.AddSync(os.Stderr))
	}
	core := buildCore(format, atomicLevel, writer)
	logger := zap.New(core)

	return &CsmLog{
		sugar:  logger.Sugar(),
		level:  atomicLevel,
		writer: writer,
		format: format,
		fields: Fields{},
	}
}

// Level type — same iota values as before for backward compatibility
type Level uint32

// A constant exposing all logging levels
var AllLevels = []Level{
	PanicLevel,
	FatalLevel,
	ErrorLevel,
	WarnLevel,
	InfoLevel,
	DebugLevel,
	TraceLevel,
}

const (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel Level = iota
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel
)

var levelNames = map[Level]string{
	PanicLevel: "panic",
	FatalLevel: "fatal",
	ErrorLevel: "error",
	WarnLevel:  "warn",
	InfoLevel:  "info",
	DebugLevel: "debug",
	TraceLevel: "trace",
}

var levelFromString = map[string]Level{
	"panic": PanicLevel,
	"fatal": FatalLevel,
	"error": ErrorLevel,
	"warn":  WarnLevel,
	"info":  InfoLevel,
	"debug": DebugLevel,
	"trace": TraceLevel,
}

func (level Level) String() string {
	if s, ok := levelNames[level]; ok {
		return s
	}
	return fmt.Sprintf("unknown level %d", level)
}

func csmLevelToZap(level Level) zapcore.Level {
	switch level {
	case PanicLevel:
		return zapcore.DPanicLevel
	case FatalLevel:
		return zapcore.FatalLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case DebugLevel, TraceLevel:
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}

func zapLevelToCsm(level zapcore.Level) Level {
	switch level {
	case zapcore.DPanicLevel:
		return PanicLevel
	case zapcore.FatalLevel:
		return FatalLevel
	case zapcore.ErrorLevel:
		return ErrorLevel
	case zapcore.WarnLevel:
		return WarnLevel
	case zapcore.InfoLevel:
		return InfoLevel
	case zapcore.DebugLevel:
		return DebugLevel
	default:
		return InfoLevel
	}
}

func parseZapLevel(s string) (zapcore.Level, error) {
	lower := strings.ToLower(strings.TrimSpace(s))
	if level, ok := levelFromString[lower]; ok {
		return csmLevelToZap(level), nil
	}
	return zapcore.InfoLevel, fmt.Errorf("unknown log level: %q, setting level to 'info'", s)
}

// Package-level convenience functions
// These delegate to the global logger instance for quick access
// Prevents users needing to call GetLogger every function
func SetLevel(level Level) {
	globalLogInstance.SetLevel(level)
}

func SetOutput(output io.Writer) {
	globalLogInstance.SetOutput(output)
}

func GetLevel() Level {
	return globalLogInstance.GetLevel()
}

func SetFormat(format string) {
	globalLogInstance.SetFormat(format)
}

func WithFields(fields Fields) *CsmLog {
	return globalLogInstance.WithFields(fields)
}

func WithContext(ctx context.Context) *CsmLog {
	return globalLogInstance.WithContext(ctx)
}

func Info(msg string) {
	globalLogInstance.Info(msg)
}

func Infof(format string, args ...interface{}) {
	globalLogInstance.Infof(format, args...)
}

func Warn(msg string) {
	globalLogInstance.Warn(msg)
}

func Warnf(format string, args ...interface{}) {
	globalLogInstance.Warnf(format, args...)
}

func Error(msg string) {
	globalLogInstance.Error(msg)
}

func Errorf(format string, args ...interface{}) {
	globalLogInstance.Errorf(format, args...)
}

func Debug(msg string) {
	globalLogInstance.Debug(msg)
}

func Debugf(format string, args ...interface{}) {
	globalLogInstance.Debugf(format, args...)
}

func Trace(msg string) {
	globalLogInstance.Trace(msg)
}

func Tracef(format string, args ...interface{}) {
	globalLogInstance.Tracef(format, args...)
}

func Panic(msg string) {
	globalLogInstance.Panic(msg)
}

func Panicf(format string, args ...interface{}) {
	globalLogInstance.Panicf(format, args...)
}

func Fatal(msg string) {
	globalLogInstance.Fatal(msg)
}

func Fatalf(format string, args ...interface{}) {
	globalLogInstance.Fatalf(format, args...)
}

// Instance methods
func (l *CsmLog) SetLevel(level Level) {
	l.level.SetLevel(csmLevelToZap(level))
}

func (l *CsmLog) GetLevel() Level {
	return zapLevelToCsm(l.level.Level())
}

func (l *CsmLog) SetOutput(output io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = zapcore.Lock(zapcore.AddSync(output))
	l.rebuildLogger()
}

func (l *CsmLog) SetFormat(format string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.format = format
	l.rebuildLogger()
}

func (l *CsmLog) WithContext(ctx context.Context) *CsmLog {
	fields := ExtractFieldsFromContext(ctx)
	log := l.WithFields(fields)
	return log
}

func (l *CsmLog) WithFields(fields Fields) *CsmLog {
	l.mu.RLock()
	currentFields := cloneFields(l.fields)
	format := l.format
	writer := l.writer
	level := l.level
	l.mu.RUnlock()

	merged := currentFields
	for k, v := range fields {
		merged[k] = v
	}
	core := buildCore(format, level, writer)
	return &CsmLog{
		sugar:  zap.New(core).Sugar().With(fieldsToArgs(merged)...),
		level:  level,
		writer: writer,
		format: format,
		fields: merged,
	}
}

// rebuildLogger recreates the zap logger with current settings. Must be called under mu lock.
func (l *CsmLog) rebuildLogger() {
	core := buildCore(l.format, l.level, l.writer)
	logger := zap.New(core)
	// re-apply accumulated fields
	if len(l.fields) > 0 {
		l.sugar = logger.Sugar().With(fieldsToArgs(l.fields)...)
	} else {
		l.sugar = logger.Sugar()
	}
}

func ParseLevel(level string) (Level, error) {
	lower := strings.ToLower(strings.TrimSpace(level))
	if l, ok := levelFromString[lower]; ok {
		return l, nil
	}
	return InfoLevel, fmt.Errorf("unknown log level: %q", level)
}

// fieldsToArgs converts a Fields map to alternating key-value args for sugar.With().
func fieldsToArgs(f Fields) []interface{} {
	args := make([]interface{}, 0, len(f)*2)
	for k, v := range f {
		args = append(args, k, v)
	}
	return args
}

// WithOperation returns a CsmLog with the FieldOperation pre-set.
func (l *CsmLog) WithOperation(name string) *CsmLog {
	return l.WithFields(Fields{FieldOperation: name})
}

// WithComponent returns a CsmLog with the FieldComponent pre-set.
func (l *CsmLog) WithComponent(name string) *CsmLog {
	return l.WithFields(Fields{FieldComponent: name})
}

// TrackDuration returns a CsmLog with FieldDurationMs computed from the given start time.
func (l *CsmLog) TrackDuration(start time.Time) *CsmLog {
	return l.WithFields(Fields{FieldDurationMs: time.Since(start).Milliseconds()})
}

// Log methods — delegate to zap SugaredLogger
func (l *CsmLog) Infof(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Infof(format, args...)
}

func (l *CsmLog) Info(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Info(msg)
}

func (l *CsmLog) Warnf(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Warnf(format, args...)
}

func (l *CsmLog) Warn(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Warn(msg)
}

func (l *CsmLog) Errorf(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Errorf(format, args...)
}

func (l *CsmLog) Error(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Error(msg)
}

// Trace maps to Debug level (zap has no native Trace)
func (l *CsmLog) Tracef(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Debugf(format, args...)
}

func (l *CsmLog) Trace(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Debug(msg)
}

func (l *CsmLog) Panicf(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.DPanicf(format, args...)
	panic(fmt.Sprintf(format, args...))
}

func (l *CsmLog) Panic(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.DPanic(msg)
	panic(msg)
}

func (l *CsmLog) Debugf(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Debugf(format, args...)
}

func (l *CsmLog) Debug(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Debug(msg)
}

func (l *CsmLog) Fatalf(format string, args ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Fatalf(format, args...)
}

func (l *CsmLog) Fatal(msg string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.sugar.Fatal(msg)
}

// ExtractFieldsFromContext extracts log fields and request IDs from the context.
func ExtractFieldsFromContext(ctx context.Context) Fields {
	if ctx == nil {
		return Fields{}
	}

	base, ok := ctx.Value(contextLogFieldsKey).(Fields)
	if !ok {
		base = Fields{}
	}

	m := cloneFields(base)

	headers, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqID, ok := headers[RequestIDKey]
		if ok && len(reqID) > 0 {
			m[RequestIDKey] = reqID[0]
		}
	}

	return m
}
