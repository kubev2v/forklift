package logging

import (
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	"os"
	"strconv"
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

//
// Settings.
var Settings _Settings

func init() {
	Settings.Load()
}

//
// Logger factory.
var Factory Builder

func init() {
	Factory = &ZapBuilder{}
}

//
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

//
// Get a named logger.
func WithName(name string, kvpair ...interface{}) *Logger {
	l := &Logger{
		Real: Factory.New(),
		name: name,
	}
	l.Real = l.Real.WithValues(kvpair...)
	l.Real = l.Real.WithName(name)

	return l
}

//
// Logs at info.
func (l *Logger) Info(message string, kvpair ...interface{}) {
	if Settings.allowed(l.level) {
		l.Real.Info(message, kvpair...)
	}
}

//
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
		err = le.Unwrap()
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

//
// Logs an error without a description.
func (l *Logger) Trace(err error, kvpair ...interface{}) {
	l.Error(err, None, kvpair...)
}

//
// Get whether logger is enabled.
func (l *Logger) Enabled() bool {
	return l.Real.Enabled()
}

//
// Get logger with verbosity level.
func (l *Logger) V(level int) logr.InfoLogger {
	return &Logger{
		Real:  Factory.V(level, l.Real),
		name:  l.name,
		level: level,
	}
}

//
// Get logger with name.
func (l *Logger) WithName(name string) logr.Logger {
	return &Logger{
		Real:  l.Real.WithName(name),
		name:  name,
		level: l.level,
	}
}

//
// Get logger with values.
func (l *Logger) WithValues(kvpair ...interface{}) logr.Logger {
	return &Logger{
		Real:  l.Real.WithValues(kvpair...),
		name:  l.name,
		level: l.level,
	}
}

//
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

//
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

//
// The level is at (or above) the level setting.
func (r *_Settings) allowed(level int) bool {
	return r.Level >= level
}

//
// The level is at or above the debug threshold.
func (r *_Settings) atDebug(level int) bool {
	return level >= r.DebugThreshold
}
