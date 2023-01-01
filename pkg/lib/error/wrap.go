package error

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
)

// Create a new wrapped error.
func New(m string, kvpair ...interface{}) error {
	return Wrap(
		errors.New(m),
		kvpair...)
}

// Wrap an error.
// Returns `err` when err is `nil` or *Error.
func Wrap(err error, kvpair ...interface{}) error {
	if err == nil {
		return err
	}
	if le, cast := err.(*Error); cast {
		le.append(kvpair)
		return le
	}
	bfr := make([]uintptr, 50)
	n := runtime.Callers(2, bfr[:])
	frames := runtime.CallersFrames(bfr[:n])
	stack := []string{""}
	for {
		f, hasNext := frames.Next()
		frame := fmt.Sprintf(
			"%s()\n\t%s:%d",
			f.Function,
			f.File,
			f.Line)
		stack = append(stack, frame)
		if !hasNext {
			break
		}
	}
	newError := &Error{
		stack:   stack,
		wrapped: err,
	}

	newError.append(kvpair)

	return newError
}

// Unwrap an error.
// Returns: the original error when not wrapped.
func Unwrap(err error) (out error) {
	if err == nil {
		return
	}
	out = err
	for {
		if wrapped, cast := out.(interface{ Unwrap() error }); cast {
			out = wrapped.Unwrap()
		} else {
			break
		}
	}

	return
}

// Error.
// Wraps a root cause error and captures
// the stack.
type Error struct {
	// Original error.
	wrapped error
	// Error description.
	description string
	// Context.
	context []interface{}
	// Stack.
	stack []string
}

// Error description.
func (e Error) Error() string {
	if len(e.description) > 0 {
		return e.causedBy(e.description, e.wrapped.Error())
	} else {
		return e.wrapped.Error()
	}
}

// Error stack trace.
// Format:
//
//	package.Function()
//	  file:line
//	package.Function()
//	  file:line
//	...
func (e Error) Stack() string {
	return strings.Join(e.stack, "\n")
}

// Get `context` key/value pairs.
func (e Error) Context() []interface{} {
	return e.context
}

// Unwrap the error.
func (e Error) Unwrap() error {
	return Unwrap(e.wrapped)
}

// Append context.
// And odd number of context is interpreted as:
// a description followed by an even number of key value pairs.
func (e *Error) append(kvpair []interface{}) {
	if len(kvpair) == 0 {
		return
	}
	fLen := float64(len(kvpair))
	odd := math.Mod(fLen, 2) != 0
	if description, cast := kvpair[0].(string); odd && cast {
		kvpair = kvpair[1:]
		if len(e.description) > 0 {
			e.description = e.causedBy(description, e.description)
		} else {
			e.description = description
		}
	}

	e.context = append(e.context, kvpair...)
}

// Build caused-by.
func (e *Error) causedBy(error, caused string) string {
	return fmt.Sprintf(
		"%s caused by: '%s'",
		error,
		caused)
}
