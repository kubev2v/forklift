package pure

import (
	"testing"
)

func TestFcUidToWWN(t *testing.T) {
	testCases := []struct {
		name          string
		fcUid         string
		expectedWwn   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid fcUid",
			fcUid:       "fc.2020202020202020:2020202020202020",
			expectedWwn: "20:20:20:20:20:20:20:20",
			expectError: false,
		},
		{
			name:        "valid fcUid with no WWNP",
			fcUid:       "fc.2020202020202020",
			expectedWwn: "20:20:20:20:20:20:20:20",
			expectError: false,
		},
		{
			name:          "invalid prefix",
			fcUid:         "f.2020202020202020:2020202020202020",
			expectedWwn:   "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "invalid id",
			fcUid:         "fc.",
			expectedWwn:   "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
		{
			name:          "odd length wwn",
			fcUid:         "fc.12345:67890",
			expectedWwn:   "",
			expectError:   true,
			errorContains: "length isn't even",
		},
		{
			name:        "lowercase input",
			fcUid:       "fc.2a2b2c2d2a2b2c2d:2020202020202020",
			expectedWwn: "2A:2B:2C:2D:2A:2B:2C:2D",
			expectError: false,
		},
		{
			name:          "empty string",
			fcUid:         "",
			expectedWwn:   "",
			expectError:   true,
			errorContains: "doesn't start with 'fc.'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wwn, err := fcUidToWWN(tc.fcUid)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if wwn != tc.expectedWwn {
					t.Errorf("expected wwn %q, but got %q", tc.expectedWwn, wwn)
				}
			}
		})
	}
}
