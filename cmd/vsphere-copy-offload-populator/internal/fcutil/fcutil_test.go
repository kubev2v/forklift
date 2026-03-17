package fcutil

import (
	"strings"
	"testing"
)

func TestParseFCAdapter(t *testing.T) {
	testCases := []struct {
		name          string
		fcID          string
		expectedWWNN  string
		expectedWWPN  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid FC adapter ID",
			fcID:         "fc.20000000C0A80ABC:21000000C0A80DEF",
			expectedWWNN: "20000000C0A80ABC",
			expectedWWPN: "21000000C0A80DEF",
			expectError:  false,
		},
		{
			name:         "valid with lowercase hex",
			fcID:         "fc.20000000c0a80abc:2a000000c0a80def",
			expectedWWNN: "20000000C0A80ABC",
			expectedWWPN: "2A000000C0A80DEF",
			expectError:  false,
		},
		{
			name:         "valid with mixed case",
			fcID:         "fc.AbCdEf0123456789:FeDcBa9876543210",
			expectedWWNN: "ABCDEF0123456789",
			expectedWWPN: "FEDCBA9876543210",
			expectError:  false,
		},
		{
			name:          "missing fc. prefix",
			fcID:          "20000000C0A80ABC:21000000C0A80DEF",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "invalid prefix",
			fcID:          "f.20000000C0A80ABC:21000000C0A80DEF",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "missing WWPN (no colon)",
			fcID:          "fc.20000000C0A80ABC",
			expectError:   true,
			errorContains: "not in expected fc.WWNN:WWPN format",
		},
		{
			name:          "empty WWPN",
			fcID:          "fc.20000000C0A80ABC:",
			expectError:   true,
			errorContains: "empty WWNN or WWPN",
		},
		{
			name:          "empty WWNN",
			fcID:          "fc.:21000000C0A80DEF",
			expectError:   true,
			errorContains: "empty WWNN or WWPN",
		},
		{
			name:          "odd length WWNN",
			fcID:          "fc.200000000000001:21000000C0A80DEF",
			expectError:   true,
			errorContains: "WWNN",
		},
		{
			name:          "odd length WWPN",
			fcID:          "fc.20000000C0A80ABC:210000000000001",
			expectError:   true,
			errorContains: "WWPN",
		},
		{
			name:          "non-hex characters in WWNN",
			fcID:          "fc.2000000Z00000ABC:21000000C0A80DEF",
			expectError:   true,
			errorContains: "non-hex",
		},
		{
			name:          "non-hex characters in WWPN",
			fcID:          "fc.20000000C0A80ABC:2100000G00000DEF",
			expectError:   true,
			errorContains: "non-hex",
		},
		{
			name:          "empty string",
			fcID:          "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "multiple colons",
			fcID:          "fc.20:00:00:00:C0:A8:0A:BC:21:00:00:00:C0:A8:0D:EF",
			expectError:   true,
			errorContains: "not in expected fc.WWNN:WWPN format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wwnn, wwpn, err := ParseFCAdapter(tc.fcID)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if wwnn != tc.expectedWWNN {
					t.Errorf("expected WWNN %q, but got %q", tc.expectedWWNN, wwnn)
				}
				if wwpn != tc.expectedWWPN {
					t.Errorf("expected WWPN %q, but got %q", tc.expectedWWPN, wwpn)
				}
			}
		})
	}
}

func TestFormatWWNWithColons(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "16 character WWN",
			input:    "21000000C0A80DEF",
			expected: "21:00:00:00:C0:A8:0D:EF", // NOSONAR
		},
		{
			name:     "different WWN",
			input:    "ABCDEF0123456789",
			expected: "AB:CD:EF:01:23:45:67:89", // NOSONAR
		},
		{
			name:     "all zeros",
			input:    "0000000000000000",
			expected: "00:00:00:00:00:00:00:00",
		},
		{
			name:     "odd length (edge case)",
			input:    "123456789",
			expected: "12:34:56:78:9",
		},
		{
			name:     "single character",
			input:    "A",
			expected: "A",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "two characters",
			input:    "AB",
			expected: "AB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatWWNWithColons(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, but got %q", tc.expected, result)
			}
		})
	}
}

func TestNormalizeWWN(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "WWN with colons",
			input:    "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			expected: "21000000C0A80DEF",
		},
		{
			name:     "WWN with dashes",
			input:    "21-00-00-00-C0-A8-0D-EF", // NOSONAR
			expected: "21000000C0A80DEF",
		},
		{
			name:     "WWN with spaces",
			input:    "21 00 00 00 C0 A8 0D EF", // NOSONAR
			expected: "21000000C0A80DEF",
		},
		{
			name:     "WWN with mixed formatting",
			input:    "21:00-00 00:C0-A8 0D:EF", // NOSONAR
			expected: "21000000C0A80DEF",
		},
		{
			name:     "lowercase input",
			input:    "abcdef0123456789",
			expected: "ABCDEF0123456789",
		},
		{
			name:     "already normalized",
			input:    "21000000C0A80DEF",
			expected: "21000000C0A80DEF",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeWWN(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, but got %q", tc.expected, result)
			}
		})
	}
}

func TestExtractAndFormatWWPN(t *testing.T) {
	testCases := []struct {
		name          string
		fcID          string
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid FC adapter ID",
			fcID:        "fc.20000000C0A80ABC:21000000C0A80DEF",
			expected:    "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			expectError: false,
		},
		{
			name:        "lowercase input",
			fcID:        "fc.20000000c0a80abc:abcdef0123456789",
			expected:    "AB:CD:EF:01:23:45:67:89", // NOSONAR
			expectError: false,
		},
		{
			name:          "invalid format",
			fcID:          "fc.20000000C0A80ABC",
			expectError:   true,
			errorContains: "not in expected",
		},
		{
			name:          "odd length WWPN",
			fcID:          "fc.20000000C0A80ABC:210000000000001",
			expectError:   true,
			errorContains: "odd length",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExtractAndFormatWWPN(tc.fcID)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("expected %q, but got %q", tc.expected, result)
				}
			}
		})
	}
}

func TestExtractWWPN(t *testing.T) {
	testCases := []struct {
		name          string
		fcID          string
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid FC adapter ID",
			fcID:        "fc.20000000C0A80ABC:21000000C0A80DEF",
			expected:    "21000000C0A80DEF",
			expectError: false,
		},
		{
			name:        "lowercase input becomes uppercase",
			fcID:        "fc.20000000c0a80abc:abcdef0123456789",
			expected:    "ABCDEF0123456789",
			expectError: false,
		},
		{
			name:          "invalid format",
			fcID:          "20000000C0A80ABC:21000000C0A80DEF",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExtractWWPN(tc.fcID)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("expected error to contain %q, but got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tc.expected {
					t.Errorf("expected %q, but got %q", tc.expected, result)
				}
			}
		})
	}
}

func TestCompareWWNs(t *testing.T) {
	testCases := []struct {
		name     string
		wwn1     string
		wwn2     string
		expected bool
	}{
		{
			name:     "identical formatted WWNs",
			wwn1:     "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			wwn2:     "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			expected: true,
		},
		{
			name:     "formatted vs unformatted",
			wwn1:     "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			wwn2:     "21000000C0A80DEF",
			expected: true,
		},
		{
			name:     "colon vs dash formatting",
			wwn1:     "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			wwn2:     "21-00-00-00-C0-A8-0D-EF",
			expected: true,
		},
		{
			name:     "lowercase vs uppercase",
			wwn1:     "abcdef0123456789",
			wwn2:     "AB:CD:EF:01:23:45:67:89", // NOSONAR
			expected: true,
		},
		{
			name:     "different WWNs",
			wwn1:     "21:00:00:00:C0:A8:0D:EF", // NOSONAR
			wwn2:     "21:00:00:00:C0:A8:0D:FF", // NOSONAR
			expected: false,
		},
		{
			name:     "empty strings",
			wwn1:     "",
			wwn2:     "",
			expected: true,
		},
		{
			name:     "one empty",
			wwn1:     "21000000C0A80DEF",
			wwn2:     "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CompareWWNs(tc.wwn1, tc.wwn2)
			if result != tc.expected {
				t.Errorf("expected %v, but got %v for comparing %q and %q",
					tc.expected, result, tc.wwn1, tc.wwn2)
			}
		})
	}
}
