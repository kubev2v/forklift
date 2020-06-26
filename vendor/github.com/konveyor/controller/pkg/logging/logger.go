package logging

import (
	"fmt"
	"github.com/go-logr/logr"
	liberr "github.com/konveyor/controller/pkg/error"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage/names"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sync"
)

const (
	Stack = "stacktrace"
	Error = "error"
	None  = ""
)

//
// Protect the history.
// Cannot be part of Logger as logr interface requires
// some by-value method receivers.
var mutex sync.RWMutex

//
// Logger
// Delegates functionality to the wrapped `Real` logger.
// Provides:
//   - Prevents duplicate logging of the same error.
//   - Provides a `Trace()` method for convenience and brevity.
//   - Prevent spamming the log with `Conflict` errors.
type Logger struct {
	Real    logr.Logger
	history map[error]bool
	name    string
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
// Updates the generated correlation suffix in the name and
// clears the reported error history.
func (l *Logger) Reset() {
	mutex.Lock()
	defer mutex.Unlock()
	name := fmt.Sprintf("%s|", l.name)
	name = names.SimpleNameGenerator.GenerateName(name)
	l.Real = logf.Log.WithName(name)
	l.history = make(map[error]bool)
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
// Previously logged errors are ignored.
// `Conflict` errors are not logged.
func (l Logger) Error(err error, message string, kvpair ...interface{}) {
	if err == nil {
		return
	}
	mutex.Lock()
	defer mutex.Unlock()
	le, wrapped := err.(*liberr.Error)
	if wrapped {
		err = le.Unwrap()
		_, found := l.history[err]
		if found || errors.IsConflict(err) {
			return
		}
		kvpair = append(
			kvpair,
			Error,
			le.Error(),
			Stack,
			le.Stack())
		l.Real.Info(message, kvpair...)
		l.history[err] = true
	} else {
		_, found := l.history[err]
		if found || errors.IsConflict(err) {
			return
		}
		l.Real.Error(err, message, kvpair...)
		l.history[err] = true
	}
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
