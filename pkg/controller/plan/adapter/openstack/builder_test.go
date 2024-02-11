package openstack

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("vSphere builder", func() {
	table.DescribeTable("should", func(os, version, distro, matchPreferenceName string) {
		Expect(getPreferenceOs(os, version, distro)).Should(Equal(matchPreferenceName))
	},
		table.Entry("rhel9", RHEL, "9", RHEL, "rhel.9"),
		table.Entry("centos stream 9", CentOS, "9", CentOS, "centos.stream9"),
		table.Entry("windows 11", Windows, "11", Windows, "windows.11.virtio"),
		table.Entry("windows2022", Windows, "2022", Windows, "windows.2k22.virtio"),
		table.Entry("ubuntu 22", Ubuntu, "22.04.3", Ubuntu, "ubuntu"),
	)
})
