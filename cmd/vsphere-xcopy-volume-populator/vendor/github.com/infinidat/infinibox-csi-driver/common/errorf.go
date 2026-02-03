package common

import (
	"fmt"
	"runtime"
)

func Errorf(format string, a ...interface{}) error {
	_, file, line, ok := runtime.Caller(1) // '1' skips the current function
	if !ok {
		file = "???"
		line = 0
	}
	// Format the message with location info
	format = fmt.Sprintf("%s:%d: %s", file, line, format)
	return fmt.Errorf(format, a...)
}
