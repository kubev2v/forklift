package openstack

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("vSphere builder", func() {
	DescribeTable("should", func(os, version, distro, matchPreferenceName string) {
		Expect(getPreferenceOs(os, version, distro)).Should(Equal(matchPreferenceName))
	},
		Entry("rhel9", RHEL, "9", RHEL, "rhel.9"),
		Entry("centos stream 9", CentOS, "9", CentOS, "centos.stream9"),
		Entry("windows 11", Windows, "11", Windows, "windows.11.virtio"),
		Entry("windows2022", Windows, "2022", Windows, "windows.2k22.virtio"),
		Entry("ubuntu 22", Ubuntu, "22.04.3", Ubuntu, "ubuntu"),
	)
})
