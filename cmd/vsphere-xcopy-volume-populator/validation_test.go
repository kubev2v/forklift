package main

import (
	"strings"
	"testing"
)

func TestValidateStorageAuthentication(t *testing.T) {
	testCases := []struct {
		name          string
		token         string
		username      string
		password      string
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:        "token only - should be valid",
			token:       "my-api-token",
			username:    "",
			password:    "",
			expectError: false,
			description: "Token-based authentication without username/password",
		},
		{
			name:        "username and password only - should be valid",
			token:       "",
			username:    "admin",
			password:    "secret",
			expectError: false,
			description: "Username/password authentication without token",
		},
		{
			name:        "both token and credentials - token takes precedence (valid)",
			token:       "my-api-token",
			username:    "admin",
			password:    "secret",
			expectError: false,
			description: "When both are provided, token is used",
		},
		{
			name:          "no credentials - should be invalid",
			token:         "",
			username:      "",
			password:      "",
			expectError:   true,
			errorContains: "either STORAGE_TOKEN or both STORAGE_USERNAME and STORAGE_PASSWORD must be provided",
			description:   "No authentication credentials provided",
		},
		{
			name:          "username without password - should be invalid",
			token:         "",
			username:      "admin",
			password:      "",
			expectError:   true,
			errorContains: "either STORAGE_TOKEN or both STORAGE_USERNAME and STORAGE_PASSWORD must be provided",
			description:   "Incomplete username/password credentials",
		},
		{
			name:          "password without username - should be invalid",
			token:         "",
			username:      "",
			password:      "secret",
			expectError:   true,
			errorContains: "either STORAGE_TOKEN or both STORAGE_USERNAME and STORAGE_PASSWORD must be provided",
			description:   "Incomplete username/password credentials",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStorageAuthentication(tc.token, tc.username, tc.password)

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error but got none", tc.description)
				} else if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("%s: expected error to contain %q, but got %q",
						tc.description, tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.description, err)
				}
			}
		})
	}
}

func TestTokenPrecedence(t *testing.T) {
	// Test that when both token and username/password are provided, validation succeeds
	// (because token takes precedence)
	err := validateStorageAuthentication("my-token", "user", "pass")
	if err != nil {
		t.Errorf("expected no error when both token and username/password are provided, got: %v", err)
	}
}

func TestEmptyToken(t *testing.T) {
	// Test that empty token falls back to username/password validation
	err := validateStorageAuthentication("", "user", "pass")
	if err != nil {
		t.Errorf("expected no error with empty token and valid username/password, got: %v", err)
	}

	// Empty token with missing credentials should fail
	err = validateStorageAuthentication("", "", "")
	if err == nil {
		t.Error("expected error with empty token and no username/password")
	}
}
