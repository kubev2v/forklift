package logging

import (
	"os"
	"strconv"

	"github.com/go-logr/logr"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

const (
	Stack = "stacktrace"
	Error = "error"
	None  = ""
)
const (
	EnvDevelopment = "LOG_DEVELOPMENT"
	EnvLevel       = "LOG_LEVEL"
)

// Settings.
var Settings _Settings

func init() {
	Settings.Load()
}

// Logger factory.
var Factory Builder

func init() {
	Factory = &ZapBuilder{}
}

// Logger
// Delegates functionality to the wrapped `Real` logger.
// Provides:
//   - Provides a `Trace()` method for convenience and brevity.
//   - Handles wrapped errors.
type Logger struct {
	// Real (wrapped) logger.
	Real logr.Logger
	// Name.
	name string
	// Level.
	level int
}

type LevelLogger interface {
	Info(msg string, kv ...interface{})
	Enabled() bool
	Error(err error, msg string, kv ...interface{})
	WithValues(kv ...interface{}) LevelLogger
	WithName(name string) LevelLogger
	V(level int) LevelLogger
	Trace(err error, kvpair ...interface{})
}

type levelLoggerImpl struct {
	real  logr.Logger
	level int
}

func (l *levelLoggerImpl) Info(msg string, kv ...interface{}) {
	if Settings.allowed(l.level) {
		l.real.Info(msg, kv...)
	}
}

func (l *levelLoggerImpl) Enabled() bool {
	return l.real.Enabled()
}

func (l *levelLoggerImpl) Error(err error, msg string, kv ...interface{}) {
	l.real.Error(err, msg, kv...)
}

func (l *levelLoggerImpl) V(level int) LevelLogger {
	return &levelLoggerImpl{
		real:  l.real.V(level),
		level: level,
	}
}

func (l *levelLoggerImpl) WithValues(kv ...interface{}) LevelLogger {
	return &levelLoggerImpl{
		real:  l.real.WithValues(kv...),
		level: l.level,
	}
}

func (l *levelLoggerImpl) WithName(name string) LevelLogger {
	return &levelLoggerImpl{
		real:  l.real.WithName(name),
		level: l.level,
	}
}

func (l *levelLoggerImpl) Trace(err error, kvpair ...interface{}) {
	l.real.Error(err, None, kvpair...)
}

// Logs at info.
func (l *Logger) Info(level int, message string, kvpair ...interface{}) {
	if Settings.allowed(l.level) {
		l.Real.Info(message, kvpair...)
	}
}

func WithName(name string, kvpair ...interface{}) LevelLogger {
	l := &Logger{
		Real: Factory.New(),
		name: name,
	}
	l.Real = l.Real.WithValues(kvpair...)
	l.Real = l.Real.WithName(name)

	return &levelLoggerImpl{
		real:  l.Real,
		level: 0,
	}
}

// Logs an error.
func (l *Logger) Error(err error, message string, kvpair ...interface{}) {
	if err == nil {
		return
	}
	if !Settings.allowed(l.level) {
		return
	}
	le, wrapped := err.(*liberr.Error)
	if wrapped {
		if context := le.Context(); context != nil {
			context = append(
				context,
				kvpair...)
			kvpair = context
		}
		kvpair = append(
			kvpair,
			Error,
			le.Error(),
			Stack,
			le.Stack())

		l.Real.Info(message, kvpair...)
		return
	}
	if wErr, wrapped := err.(interface {
		Unwrap() error
	}); wrapped {
		err = wErr.Unwrap()
	}
	if err == nil {
		return
	}

	l.Real.Error(err, message, kvpair...)
}

// Logs an error without a description.
func (l *Logger) Trace(err error, kvpair ...interface{}) {
	l.Error(err, None, kvpair...)
}

// Get whether logger is enabled.
func (l *Logger) Enabled(level int) bool {
	return l.Real.Enabled()
}

// Get logger with verbosity level.
func (l *Logger) V(level int) *Logger {
	return &Logger{
		Real:  Factory.V(level, l.Real),
		name:  l.name,
		level: level,
	}
}

// Package settings.
type _Settings struct {
	// Debug threshold.
	// Level determines when the real
	// debug logger is used.
	DebugThreshold int
	// Development configuration.
	Development bool
	// Info level threshold.
	// Higher level increases verbosity.
	Level int
}

// Determine development logger.
func (r *_Settings) Load() {
	r.DebugThreshold = 4
	if s, found := os.LookupEnv(EnvDevelopment); found {
		bv, err := strconv.ParseBool(s)
		if err == nil {
			r.Development = bv
		}
	}
	if s, found := os.LookupEnv(EnvLevel); found {
		n, err := strconv.ParseInt(s, 10, 8)
		if err == nil {
			r.Level = int(n)
		}
	}
}

// The level is at (or above) the level setting.
func (r *_Settings) allowed(level int) bool {
	return r.Level >= level
}

// The level is at or above the debug threshold.
func (r *_Settings) atDebug(level int) bool {
	return level >= r.DebugThreshold
}
