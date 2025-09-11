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
			fcUid:       "fc.20000025b5120030:20000025b56a0030",
			expectedWwn: "20:00:00:25:B5:12:00:30",
			expectError: false,
		},
		{
			name:          "invalid prefix",
			fcUid:         "f.20000025b5120030:20000025b56a0030",
			expectedWwn:   "",
			expectError:   true,
			errorContains: "doesn't strat with 'fc.'",
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
			fcUid:       "fc.20000025b5120030:20000025b56a0030",
			expectedWwn: "20:00:00:25:B5:12:00:30",
			expectError: false,
		},
		{
			name:          "empty string",
			fcUid:         "",
			expectedWwn:   "",
			expectError:   true,
			errorContains: "doesn't strat with 'fc.'",
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
