package primera3par

import (
	"testing"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/stretchr/testify/assert"
)

func TestExtractSerialFromNAA(t *testing.T) {
	tests := []struct {
		name          string
		naa           string
		expected      string
		expectError   bool
		errorContains string
	}{
		{
			name:     "valid NAA with naa. prefix",
			naa:      "naa.60002ac0000000000000001a00028af4",
			expected: "0000000000000001A00028AF4",
		},
		{
			name:     "valid NAA without prefix",
			naa:      "60002ac0000000000000001a00028af4",
			expected: "0000000000000001A00028AF4",
		},
		{
			name:     "uppercase NAA",
			naa:      "NAA.60002AC0000000000000001A00028AF4",
			expected: "0000000000000001A00028AF4",
		},
		{
			name:          "wrong provider ID",
			naa:           "naa.624a93700000000000001234",
			expectError:   true,
			errorContains: "does not appear to be a 3PAR device",
		},
		{
			name:          "empty serial after provider ID",
			naa:           "naa.60002ac",
			expectError:   true,
			errorContains: "could not extract serial",
		},
		{
			name:          "empty string",
			naa:           "",
			expectError:   true,
			errorContains: "does not appear to be a 3PAR device",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serial, err := extractSerialFromNAA(tc.naa)
			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, serial)
			}
		})
	}
}

// clonnerWithVolumes creates a Primera3ParClonner backed by a mock client pre-loaded with volumes
func clonnerWithVolumes(volumes []Volume) *Primera3ParClonner {
	mock := NewMockPrimera3ParClient()
	for _, v := range volumes {
		mock.Volumes[v.Name] = populator.LUN{
			Name:         v.Name,
			SerialNumber: v.WWN,
		}
	}
	return &Primera3ParClonner{client: mock}
}

func TestFindVolumeByVVolID(t *testing.T) {
	volumes := []Volume{
		{Id: 1, Name: ".mgmtdata", WWN: "AAAA1111BBBB2222"},
		{Id: 2, Name: "vv-abcdef01234567890abcdef012345678", WWN: "CCCC3333DDDD4444"},
		{Id: 3, Name: "other-volume", WWN: "EEEE5555FFFF6666"},
	}

	tests := []struct {
		name        string
		vvolID      string
		expected    string
		expectError bool
	}{
		{
			name:     "match by WWN",
			vvolID:   "rfc4122.cccc3333-dddd-4444",
			expected: "vv-abcdef01234567890abcdef012345678",
		},
		{
			name:     "match by name containing ID",
			vvolID:   "rfc4122.abcdef01-2345-6789-0abc-def012345678",
			expected: "vv-abcdef01234567890abcdef012345678",
		},
		{
			name:        "no match",
			vvolID:      "rfc4122.00000000-0000-0000-0000-000000000000",
			expectError: true,
		},
		{
			name:     "without rfc4122 prefix",
			vvolID:   "cccc3333-dddd-4444",
			expected: "vv-abcdef01234567890abcdef012345678",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clonner := clonnerWithVolumes(volumes)
			result, err := clonner.findVolumeByVVolID(tc.vvolID)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestResolveRDMToLUN(t *testing.T) {
	volumes := []Volume{
		{Id: 1, Name: "source-vol-1", WWN: "0000000000000001A00028AF4"},
		{Id: 2, Name: "source-vol-2", WWN: "AABBCCDD11223344"},
	}

	tests := []struct {
		name        string
		deviceName  string
		expected    string
		expectError bool
	}{
		{
			name:       "resolve by NAA with prefix",
			deviceName: "naa.60002ac0000000000000001a00028af4",
			expected:   "source-vol-1",
		},
		{
			name:       "resolve by NAA uppercase",
			deviceName: "NAA.60002AC0000000000000001A00028AF4",
			expected:   "source-vol-1",
		},
		{
			name:        "non-3PAR NAA falls back to findVolumeByDeviceName and fails",
			deviceName:  "naa.624a93700000000000009999",
			expectError: true,
		},
		{
			name:       "fallback matches by WWN substring",
			deviceName: "naa.500AABBCCDD11223344",
			expected:   "source-vol-2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clonner := clonnerWithVolumes(volumes)
			lun, err := clonner.resolveRDMToLUN(tc.deviceName)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, lun.Name)
			}
		})
	}
}

func TestFindVolumeByDeviceName(t *testing.T) {
	volumes := []Volume{
		{Id: 1, Name: "vol-abc", WWN: "AABBCCDD11223344"},
		{Id: 2, Name: "vol-xyz", WWN: "EEFF00112233AABB"},
	}

	tests := []struct {
		name        string
		deviceName  string
		expected    string
		expectError bool
	}{
		{
			name:       "match by WWN substring in device name",
			deviceName: "some-path-aabbccdd11223344-lun",
			expected:   "vol-abc",
		},
		{
			name:       "match by full NAA",
			deviceName: "naa.60002acaabbccdd11223344",
			expected:   "vol-abc",
		},
		{
			name:        "no match",
			deviceName:  "naa.60002ac9999999999999999",
			expectError: true,
		},
		{
			name:       "case insensitive match",
			deviceName: "NAA.60002ACEEFF00112233AABB",
			expected:   "vol-xyz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clonner := clonnerWithVolumes(volumes)
			lun, err := clonner.findVolumeByDeviceName(tc.deviceName)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, lun.Name)
			}
		})
	}
}
