package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plan/utils", func() {
	DescribeTable("convert dev", func(dev string, number int) {
		Expect(GetDeviceNumber(dev)).Should(Equal(number))
	},
		Entry("sda", "/dev/sda", 1),
		Entry("sdb", "/dev/sdb", 2),
		Entry("sdz", "/dev/sdz", 26),
		Entry("sda1", "/dev/sda1", 1),
		Entry("sda5", "/dev/sda5", 1),
		Entry("sdb2", "/dev/sdb2", 2),
		Entry("sdza", "/dev/sdza", 26),
		Entry("sdzb", "/dev/sdzb", 26),
		Entry("sd", "/dev/sd", 0),
		Entry("test", "test", 0),
	)
})
