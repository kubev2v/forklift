package driver

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func Test_httpStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantOK   bool
	}{
		{
			name:   "nil error",
			err:    nil,
			wantOK: false,
		},
		{
			name:   "unrelated error",
			err:    errors.New("connection refused"),
			wantOK: false,
		},
		{
			name:     "winrm basic auth 401",
			err:      fmt.Errorf("http response error: 401 - invalid content type"),
			wantCode: http.StatusUnauthorized,
			wantOK:   true,
		},
		{
			name:     "winrm cert auth 401",
			err:      fmt.Errorf("http error 401: Unauthorized"),
			wantCode: http.StatusUnauthorized,
			wantOK:   true,
		},
		{
			name:     "403 forbidden",
			err:      fmt.Errorf("http response error: 403 - access denied"),
			wantCode: http.StatusForbidden,
			wantOK:   true,
		},
		{
			name:     "500 server error",
			err:      fmt.Errorf("http response error: 500 - internal"),
			wantCode: http.StatusInternalServerError,
			wantOK:   true,
		},
		{
			name:     "wrapped winrm error",
			err:      fmt.Errorf("WinRM command failed: %w", fmt.Errorf("http response error: 401 - invalid content type")),
			wantCode: http.StatusUnauthorized,
			wantOK:   true,
		},
		{
			name:     "double-wrapped",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("WinRM command failed: %w", fmt.Errorf("http response error: 401 - x"))),
			wantCode: http.StatusUnauthorized,
			wantOK:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, ok := httpStatus(tc.err)
			if ok != tc.wantOK {
				t.Errorf("httpStatus() ok = %v, want %v", ok, tc.wantOK)
			}
			if code != tc.wantCode {
				t.Errorf("httpStatus() code = %d, want %d", code, tc.wantCode)
			}
		})
	}
}

func TestWrapCommandError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantNil    bool
		wantIsAuth bool
	}{
		{
			name:    "nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:       "unrelated error passes through",
			err:        errors.New("timeout"),
			wantIsAuth: false,
		},
		{
			name:       "401 becomes ErrUnauthorized",
			err:        fmt.Errorf("http response error: 401 - invalid content type"),
			wantIsAuth: true,
		},
		{
			name:       "403 becomes ErrUnauthorized",
			err:        fmt.Errorf("http response error: 403 - forbidden"),
			wantIsAuth: true,
		},
		{
			name:       "500 does not become ErrUnauthorized",
			err:        fmt.Errorf("http response error: 500 - internal"),
			wantIsAuth: false,
		},
		{
			name:       "wrapped 401",
			err:        fmt.Errorf("WinRM command failed: %w", fmt.Errorf("http response error: 401 - x")),
			wantIsAuth: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := WrapCommandError(tc.err)
			if tc.wantNil {
				if result != nil {
					t.Fatalf("WrapCommandError(nil) = %v, want nil", result)
				}
				return
			}
			if result == nil {
				t.Fatal("WrapCommandError() returned nil for non-nil input")
			}
			isAuth := errors.Is(result, ErrUnauthorized)
			if isAuth != tc.wantIsAuth {
				t.Errorf("errors.Is(result, ErrUnauthorized) = %v, want %v (err=%v)", isAuth, tc.wantIsAuth, result)
			}
		})
	}
}
