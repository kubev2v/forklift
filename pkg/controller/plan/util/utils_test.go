package util

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Helper function to check if VM name is a valid DNS1123 subdomain
func validateVmName(name string) bool {
	return len(validation.IsDNS1123Subdomain(name)) == 0
}

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

	Context("VM Name Handler", func() {
		It("should handle all cases in name adjustments", func() {
			originalVmName := "----------------Vm!@#$%^&*()_+-Name/.is,';[]-CorREct-<>123----------------------"
			newVmName := "vm-name.is-correct-123"
			changedName := SanitizeLabel(originalVmName)
			Expect(changedName).To(Equal(newVmName))
			Expect(validateVmName(changedName)).To(BeTrue(), "Changed name should match DNS1123 subdomain format")
		})

		It("should handle the case that the VM name is empty after all removals", func() {
			emptyVM := ".__."
			newVmNameFromId := "vm-"
			changedEmptyName := SanitizeLabel(emptyVM)
			Expect(changedEmptyName).To(ContainSubstring(newVmNameFromId))
			Expect(validateVmName(changedEmptyName)).To(BeTrue(), "Changed name from empty should match DNS1123 subdomain format")
		})

		It("should handle multiple consecutive dots", func() {
			multiDotVM := "mtv.func.-.rhel.-...8.8"
			expectedMultiDotResult := "mtv.func.rhel.8.8"
			changedMultiDotName := SanitizeLabel(multiDotVM)
			Expect(changedMultiDotName).To(Equal(expectedMultiDotResult))
			Expect(validateVmName(changedMultiDotName)).To(BeTrue(), "Changed name with multiple dots should match DNS1123 subdomain format")

			multiDotVM2 := ".....mtv.func..-...............rhel.-...8.8"
			expectedMultiDotResult2 := "mtv.func.rhel.8.8"
			changedMultiDotName2 := SanitizeLabel(multiDotVM2)
			Expect(changedMultiDotName2).To(Equal(expectedMultiDotResult2))
			Expect(validateVmName(changedMultiDotName2)).To(BeTrue(), "Changed name with multiple leading dots should match DNS1123 subdomain format")
		})

		It("should convert spaces to dashes", func() {
			spaceVM := "vm with spaces in name"
			expectedSpaceResult := "vm-with-spaces-in-name"
			changedSpaceName := SanitizeLabel(spaceVM)
			Expect(changedSpaceName).To(Equal(expectedSpaceResult))
			Expect(validateVmName(changedSpaceName)).To(BeTrue(), "Changed name with spaces should match DNS1123 subdomain format")
		})

		It("should convert + signs to dashes", func() {
			plusVM := "vm+with+plus+signs"
			expectedPlusResult := "vm-with-plus-signs"
			changedPlusName := SanitizeLabel(plusVM)
			Expect(changedPlusName).To(Equal(expectedPlusResult))
			Expect(validateVmName(changedPlusName)).To(BeTrue(), "Changed name with plus signs should match DNS1123 subdomain format")
		})

		It("should remove multiple consecutive dashes", func() {
			multipleDashVM := "vm---with----multiple-----dashes"
			expectedMultipleDashResult := "vm-with-multiple-dashes"
			changedMultipleDashName := SanitizeLabel(multipleDashVM)
			Expect(changedMultipleDashName).To(Equal(expectedMultipleDashResult))
			Expect(validateVmName(changedMultipleDashName)).To(BeTrue(), "Changed name with multiple dashes should match DNS1123 subdomain format")
		})

		It("should handle complex case with spaces, plus signs, and multiple dashes", func() {
			complexVM := "vm   +++with   ---mixed+++   ---characters"
			expectedComplexResult := "vm-with-mixed-characters"
			changedComplexName := SanitizeLabel(complexVM)
			Expect(changedComplexName).To(Equal(expectedComplexResult))
			Expect(validateVmName(changedComplexName)).To(BeTrue(), "Changed name with mixed special characters should match DNS1123 subdomain format")
		})

		It("should convert * (asterisk) to dashes", func() {
			asteriskVM := "vm*with*asterisk*characters"
			expectedAsteriskResult := "vm-with-asterisk-characters"
			changedAsteriskName := SanitizeLabel(asteriskVM)
			Expect(changedAsteriskName).To(Equal(expectedAsteriskResult))
			Expect(validateVmName(changedAsteriskName)).To(BeTrue(), "Changed name with asterisk should match DNS1123 subdomain format")
		})

		It("should trim names longer than NameMaxLength", func() {
			const labelMax = validation.DNS1123LabelMaxLength

			long := strings.Repeat("a", labelMax+10)
			changed := SanitizeLabel(long)
			Expect(changed).To(HaveLen(labelMax))
			Expect(validateVmName(changed)).To(BeTrue())
		})
	})
})
