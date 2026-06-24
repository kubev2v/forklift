package driver

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
)

// ErrUnauthorized indicates a WinRM authentication / authorization failure.
var ErrUnauthorized = errors.New("hyperv: unauthorized")

// winrmHTTPStatus extracts the numeric HTTP status from the winrm library's
// known error formats: "http response error: <code> - ..." and "http error <code>: ...".
var winrmHTTPStatus = regexp.MustCompile(`http (?:response )?error[:\s]+(\d{3})`)

// httpStatus extracts the HTTP status code from a WinRM error message.
func httpStatus(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	if m := winrmHTTPStatus.FindStringSubmatch(err.Error()); len(m) == 2 {
		if code, convErr := strconv.Atoi(m[1]); convErr == nil {
			return code, true
		}
	}
	return 0, false
}

// WrapCommandError inspects a WinRM command error.
func WrapCommandError(err error) error {
	if err == nil {
		return nil
	}
	if code, ok := httpStatus(err); ok {
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			return fmt.Errorf("%w: %w", ErrUnauthorized, err)
		}
	}
	return err
}
