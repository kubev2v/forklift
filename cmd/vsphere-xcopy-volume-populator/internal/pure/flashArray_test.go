package pure

import (
	"strings"
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
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

func TestExtractSerialFromNAA(t *testing.T) {
	testCases := []struct {
		name           string
		naa            string
		expectedSerial string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid NAA with naa. prefix",
			naa:            "naa.624a9370abcd1234efgh5678",
			expectedSerial: "ABCD1234EFGH5678",
			expectError:    false,
		},
		{
			name:           "valid NAA without prefix",
			naa:            "624a9370abcd1234efgh5678",
			expectedSerial: "ABCD1234EFGH5678",
			expectError:    false,
		},
		{
			name:           "uppercase NAA",
			naa:            "NAA.624A9370ABCD1234EFGH5678",
			expectedSerial: "ABCD1234EFGH5678",
			expectError:    false,
		},
		{
			name:          "wrong provider ID",
			naa:           "naa.600a0980abcd1234efgh5678",
			expectError:   true,
			errorContains: "does not appear to be a Pure FlashArray device",
		},
		{
			name:          "empty serial",
			naa:           "naa.624a9370",
			expectError:   true,
			errorContains: "could not extract serial",
		},
		{
			name:          "empty string",
			naa:           "",
			expectError:   true,
			errorContains: "does not appear to be a Pure FlashArray device",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serial, err := extractSerialFromNAA(tc.naa)

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
				if serial != tc.expectedSerial {
					t.Errorf("expected serial %q, but got %q", tc.expectedSerial, serial)
				}
			}
		})
	}
}

func TestSupportsDiskType(t *testing.T) {
	clonner := &FlashArrayClonner{}

	testCases := []struct {
		name     string
		diskType populator.DiskType
		expected bool
	}{
		{
			name:     "supports RDM",
			diskType: populator.DiskTypeRDM,
			expected: true,
		},
		{
			name:     "supports VVol",
			diskType: populator.DiskTypeVVol,
			expected: true,
		},
		{
			name:     "supports VMDK",
			diskType: populator.DiskTypeVMDK,
			expected: true,
		},
		{
			name:     "unsupported type",
			diskType: populator.DiskType("unknown"),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := clonner.SupportsDiskType(tc.diskType)
			if result != tc.expected {
				t.Errorf("SupportsDiskType(%s) = %v, want %v", tc.diskType, result, tc.expected)
			}
		})
	}
}
