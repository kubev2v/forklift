package logging

import (
	"fmt"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	Stack = "stacktrace"
	Error = "error"
	None  = ""
)

//
// Logger
// Delegates functionality to the wrapped `Real` logger.
// Provides:
//   - Provides a `Trace()` method for convenience and brevity.
//   - Prevent spamming the log with `Conflict` errors.
//   - Handles wrapped errors.
type Logger struct {
	Real logr.Logger
	name string
}

//
// Get a named logger.
func WithName(name string) Logger {
	logger := Logger{
		Real: logf.Log.WithName(name),
		name: name,
	}
	logger.Reset()
	return logger
}

//
// Reset the logger.
// Updates the generated correlation suffix in the name.
func (l *Logger) Reset() {
	name := fmt.Sprintf("%s|", l.name)
	name = names.SimpleNameGenerator.GenerateName(name)
	l.Real = logf.Log.WithName(name)
}

//
// Set values.
func (l *Logger) SetValues(kvpair ...interface{}) {
	l.Real = l.Real.WithValues(kvpair...)
}

//
// Logs at info.
func (l Logger) Info(message string, kvpair ...interface{}) {
	l.Real.Info(message, kvpair...)
}

//
// Logs an error.
func (l Logger) Error(err error, message string, kvpair ...interface{}) {
	if err == nil {
		return
	}
	le, wrapped := err.(*liberr.Error)
	if wrapped {
		err = le.Unwrap()
		if k8serr.IsConflict(err) {
			return
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
	if err == nil || k8serr.IsConflict(err) {
		return
	}

	l.Real.Error(err, message, kvpair...)
}

//
// Logs an error without a description.
func (l Logger) Trace(err error, kvpair ...interface{}) {
	l.Error(err, None, kvpair...)
}

//
// Get whether logger is enabled.
func (l Logger) Enabled() bool {
	return l.Real.Enabled()
}

//
// Get logger with verbosity level.
func (l Logger) V(level int) logr.InfoLogger {
	return l.Real.V(level)
}

//
// Get logger with name.
func (l Logger) WithName(name string) logr.Logger {
	return Logger{
		Real: l.Real.WithName(name),
		name: l.name,
	}
}

//
// Get logger with values.
func (l Logger) WithValues(kvpair ...interface{}) logr.Logger {
	return Logger{
		Real: l.Real.WithValues(kvpair...),
		name: l.name,
	}
}
