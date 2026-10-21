package vsphere

import (
	"testing"
)

func TestGetBootDiskIndexFromInspection(t *testing.T) {
	tests := []struct {
		name      string
		xml       string
		expected  int
		expectErr bool
	}{
		{
			name: "boot disk at index 1",
			xml: `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <disks>
    <disk index='0'>
      <boot-order>2</boot-order>
    </disk>
    <disk index='1'>
      <boot-order>1</boot-order>
    </disk>
  </disks>
  <operatingsystem>
    <osinfo>rhel8.8</osinfo>
  </operatingsystem>
</v2v-inspection>`,
			expected:  1,
			expectErr: false,
		},
		{
			name: "boot disk at index 0",
			xml: `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <disks>
    <disk index='0'>
      <boot-order>1</boot-order>
    </disk>
    <disk index='1'>
      <boot-order>2</boot-order>
    </disk>
  </disks>
  <operatingsystem>
    <osinfo>rhel8.8</osinfo>
  </operatingsystem>
</v2v-inspection>`,
			expected:  0,
			expectErr: false,
		},
		{
			name: "no boot order present",
			xml: `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <disks>
    <disk index='0'>
    </disk>
    <disk index='1'>
    </disk>
  </disks>
  <operatingsystem>
    <osinfo>rhel8.8</osinfo>
  </operatingsystem>
</v2v-inspection>`,
			expected:  -1,
			expectErr: false,
		},
		{
			name: "no disks section",
			xml: `<?xml version='1.0' encoding='utf-8'?>
<v2v-inspection>
  <operatingsystem>
    <osinfo>rhel8.8</osinfo>
  </operatingsystem>
</v2v-inspection>`,
			expected:  -1,
			expectErr: false,
		},
		{
			name:      "invalid XML",
			xml:       "not valid xml",
			expected:  -1,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			index, err := GetBootDiskIndexFromInspection(tc.xml)
			if tc.expectErr && err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if index != tc.expected {
				t.Fatalf("Expected boot disk index %d, got %d", tc.expected, index)
			}
		})
	}
}
