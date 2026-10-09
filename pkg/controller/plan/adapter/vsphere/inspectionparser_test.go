package vsphere

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
