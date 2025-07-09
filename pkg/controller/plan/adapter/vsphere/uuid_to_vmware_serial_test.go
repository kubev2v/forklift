package vsphere

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UUIDToVMwareSerial", func() {
	DescribeTable("should convert valid UUIDs to VMware serial format",
		func(input string, expected string) {
			result := UUIDToVMwareSerial(input)
			Expect(result).To(Equal(expected))
		},
		Entry("standard UUID lowercase",
			"422c6a2a-5ea9-1083-39f3-3b140fffb444",
			"VMware-42 2c 6a 2a 5e a9 10 83-39 f3 3b 14 0f ff b4 44"),
		Entry("standard UUID uppercase",
			"422C6A2A-5EA9-1083-39F3-3B140FFFB444",
			"VMware-42 2c 6a 2a 5e a9 10 83-39 f3 3b 14 0f ff b4 44"),
		Entry("standard UUID mixed case",
			"422c6A2a-5eA9-1083-39F3-3b140FFFb444",
			"VMware-42 2c 6a 2a 5e a9 10 83-39 f3 3b 14 0f ff b4 44"),
		Entry("UUID with all zeros",
			"00000000-0000-0000-0000-000000000000",
			"VMware-00 00 00 00 00 00 00 00-00 00 00 00 00 00 00 00"),
		Entry("UUID with all F's",
			"ffffffff-ffff-ffff-ffff-ffffffffffff",
			"VMware-ff ff ff ff ff ff ff ff-ff ff ff ff ff ff ff ff"),
		Entry("UUID with numbers only",
			"12345678-1234-1234-1234-123456789012",
			"VMware-12 34 56 78 12 34 12 34-12 34 12 34 56 78 90 12"),
		Entry("UUID with letters only",
			"abcdefab-cdef-abcd-efab-cdefabcdefab",
			"VMware-ab cd ef ab cd ef ab cd-ef ab cd ef ab cd ef ab"),
	)

	DescribeTable("should return original string for invalid UUIDs",
		func(input string, expected string) {
			result := UUIDToVMwareSerial(input)
			Expect(result).To(Equal(expected))
		},
		Entry("empty string", "", ""),
		Entry("too short UUID",
			"422c6a2a-5ea9-1083-39f3",
			"422c6a2a-5ea9-1083-39f3"),
		Entry("too long UUID",
			"422c6a2a-5ea9-1083-39f3-3b140fffb444-extra",
			"422c6a2a-5ea9-1083-39f3-3b140fffb444-extra"),
		Entry("UUID without hyphens",
			"422c6a2a5ea9108339f33b140fffb444",
			"422c6a2a5ea9108339f33b140fffb444"),
		Entry("UUID with invalid characters",
			"422c6a2a-5ea9-1083-39f3-3b140fffb44g",
			"422c6a2a-5ea9-1083-39f3-3b140fffb44g"),
		Entry("UUID with wrong hyphen positions",
			"422c6a2a5-ea9-1083-39f3-3b140fffb444",
			"422c6a2a5-ea9-1083-39f3-3b140fffb444"),
		Entry("random string",
			"not-a-uuid",
			"not-a-uuid"),
		Entry("UUID with spaces",
			"422c6a2a-5ea9-1083-39f3- 3b140fffb444",
			"422c6a2a-5ea9-1083-39f3- 3b140fffb444"),
		Entry("partial UUID",
			"422c6a2a-5ea9",
			"422c6a2a-5ea9"),
	)

	Context("edge cases", func() {
		It("should handle maximum valid hex values", func() {
			uuid := "ffffffff-ffff-ffff-ffff-ffffffffffff"
			result := UUIDToVMwareSerial(uuid)
			Expect(result).To(Equal("VMware-ff ff ff ff ff ff ff ff-ff ff ff ff ff ff ff ff"))
		})

		It("should handle minimum valid hex values", func() {
			uuid := "00000000-0000-0000-0000-000000000000"
			result := UUIDToVMwareSerial(uuid)
			Expect(result).To(Equal("VMware-00 00 00 00 00 00 00 00-00 00 00 00 00 00 00 00"))
		})
	})
})
