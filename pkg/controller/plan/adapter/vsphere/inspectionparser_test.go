package vsphere

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

func inspectionXML(name, distro, osinfo string) string {
	return fmt.Sprintf(`<v2v-inspection><operatingsystem><name>%s</name><distro>%s</distro><osinfo>%s</osinfo><arch>x86_64</arch></operatingsystem></v2v-inspection>`,
		name, distro, osinfo)
}

var _ = Describe("GetOperationSystemFromConfig", func() {
	DescribeTable("maps virt-v2v osinfo to a VMware guest id",
		func(name, distro, osinfo, expected string) {
			os, err := GetOperationSystemFromConfig(inspectionXML(name, distro, osinfo))
			Expect(err).NotTo(HaveOccurred())
			Expect(os).To(Equal(expected))
		},
		Entry("exact match", "linux", "rhel", "rhel9", "rhel9_64Guest"),
		Entry("minor version", "linux", "rhel", "rhel9.4", "rhel9_64Guest"),
		Entry("windows", "windows", "windows", "win2k16", "windows9Server64Guest"),
		Entry("SLES 12 service pack", "linux", "sles", "sles12sp5", "sles12_64Guest"),
		Entry("SLES 15 service pack, sle prefix", "linux", "sles", "sle15sp7", "sles15_64Guest"),
		Entry("SLES 15 service pack, sles prefix", "linux", "sles", "sles15sp5", "sles15_64Guest"),
		Entry("SLES 15 GA", "linux", "sles", "sle15", "sles15_64Guest"),
		Entry("SLES 16, sle prefix", "linux", "sles", "sle16", "sles16_64Guest"),
		Entry("SLES 16, minor version", "linux", "sles", "sles16.0", "sles16_64Guest"),
		Entry("openSUSE Leap", "linux", "opensuse", "opensuse15.6", "opensuse64Guest"),
		Entry("unknown linux", "linux", "unknown", "somelinux1", "genericLinuxGuest"),
		Entry("unknown os", "other", "unknown", "something", "otherGuest64"),
	)
})
