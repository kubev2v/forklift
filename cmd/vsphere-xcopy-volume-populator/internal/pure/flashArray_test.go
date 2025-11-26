package pure

import (
	"strings"
	"testing"
)

func TestFcUIDToWWPN(t *testing.T) {
	testCases := []struct {
		name          string
		fcUid         string
		expectedWwpn  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid fcUid",
			fcUid:        "fc.2020202020202020:2121212121212121",
			expectedWwpn: "21:21:21:21:21:21:21:21",
			expectError:  false,
		},
		{
			name:          "missing WWPN",
			fcUid:         "fc.2020202020202020",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "not in expected fc.WWNN:WWPN format",
		},
		{
			name:          "invalid prefix",
			fcUid:         "f.2020202020202020:2121212121212121",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "invalid format",
			fcUid:         "fc.2020202020202020:",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "empty WWNN or WWPN",
		},
		{
			name:          "odd length wwpn",
			fcUid:         "fc.2020202020202020:12345",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "odd length",
		},
		{
			name:         "lowercase input",
			fcUid:        "fc.2020202020202020:2a2b2c2d2e2f2021",
			expectedWwpn: "2A:2B:2C:2D:2E:2F:20:21", // NOSONAR
			expectError:  false,
		},
		{
			name:          "empty string",
			fcUid:         "",
			expectedWwpn:  "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wwpn, err := fcUIDToWWPN(tc.fcUid)

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
				if wwpn != tc.expectedWwpn {
					t.Errorf("expected wwpn %q, but got %q", tc.expectedWwpn, wwpn)
				}
			}
		})
	}
}
