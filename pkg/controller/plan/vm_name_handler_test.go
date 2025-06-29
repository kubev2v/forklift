package plan

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestVmNameHandler(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Helper function to check if VM name is a valid DNS1123 subdomain
	validateVmName := func(name string) bool {
		return len(validation.IsDNS1123Subdomain(name)) == 0
	}

	//Test all cases in name adjustments
	originalVmName := "----------------Vm!@#$%^&*()_+-Name/.is,';[]-CorREct-<>123----------------------"
	newVmName := "vm-name.is-correct-123"
	changedName := changeVmName(originalVmName)
	g.Expect(changedName).To(gomega.Equal(newVmName))
	g.Expect(validateVmName(changedName)).To(gomega.BeTrue(), "Changed name should match DNS1123 subdomain format")

	//Test the case that the VM name is empty after all removals
	emptyVM := ".__."
	newVmNameFromId := "vm-"
	changedEmptyName := changeVmName(emptyVM)
	g.Expect(changedEmptyName).To(gomega.ContainSubstring(newVmNameFromId))
	g.Expect(validateVmName(changedEmptyName)).To(gomega.BeTrue(), "Changed name from empty should match DNS1123 subdomain format")

	//Test handling of multiple consecutive dots
	multiDotVM := "mtv.func.-.rhel.-...8.8"
	expectedMultiDotResult := "mtv.func.rhel.8.8"
	changedMultiDotName := changeVmName(multiDotVM)
	g.Expect(changedMultiDotName).To(gomega.Equal(expectedMultiDotResult))
	g.Expect(validateVmName(changedMultiDotName)).To(gomega.BeTrue(), "Changed name with multiple dots should match DNS1123 subdomain format")

	multiDotVM2 := ".....mtv.func..-...............rhel.-...8.8"
	expectedMultiDotResult2 := "mtv.func.rhel.8.8"
	changedMultiDotName2 := changeVmName(multiDotVM2)
	g.Expect(changedMultiDotName2).To(gomega.Equal(expectedMultiDotResult2))
	g.Expect(validateVmName(changedMultiDotName2)).To(gomega.BeTrue(), "Changed name with multiple leading dots should match DNS1123 subdomain format")

	//Test conversion of spaces to dashes
	spaceVM := "vm with spaces in name"
	expectedSpaceResult := "vm-with-spaces-in-name"
	changedSpaceName := changeVmName(spaceVM)
	g.Expect(changedSpaceName).To(gomega.Equal(expectedSpaceResult))
	g.Expect(validateVmName(changedSpaceName)).To(gomega.BeTrue(), "Changed name with spaces should match DNS1123 subdomain format")

	//Test conversion of + signs to dashes
	plusVM := "vm+with+plus+signs"
	expectedPlusResult := "vm-with-plus-signs"
	changedPlusName := changeVmName(plusVM)
	g.Expect(changedPlusName).To(gomega.Equal(expectedPlusResult))
	g.Expect(validateVmName(changedPlusName)).To(gomega.BeTrue(), "Changed name with plus signs should match DNS1123 subdomain format")

	//Test removal of multiple consecutive dashes
	multipleDashVM := "vm---with----multiple-----dashes"
	expectedMultipleDashResult := "vm-with-multiple-dashes"
	changedMultipleDashName := changeVmName(multipleDashVM)
	g.Expect(changedMultipleDashName).To(gomega.Equal(expectedMultipleDashResult))
	g.Expect(validateVmName(changedMultipleDashName)).To(gomega.BeTrue(), "Changed name with multiple dashes should match DNS1123 subdomain format")

	//Test complex case with spaces, plus signs, and multiple dashes
	complexVM := "vm   +++with   ---mixed+++   ---characters"
	expectedComplexResult := "vm-with-mixed-characters"
	changedComplexName := changeVmName(complexVM)
	g.Expect(changedComplexName).To(gomega.Equal(expectedComplexResult))
	g.Expect(validateVmName(changedComplexName)).To(gomega.BeTrue(), "Changed name with mixed special characters should match DNS1123 subdomain format")

	//Test conversion of * (asterisk) to dashes
	asteriskVM := "vm*with*asterisk*characters"
	expectedAsteriskResult := "vm-with-asterisk-characters"
	changedAsteriskName := changeVmName(asteriskVM)
	g.Expect(changedAsteriskName).To(gomega.Equal(expectedAsteriskResult))
	g.Expect(validateVmName(changedAsteriskName)).To(gomega.BeTrue(), "Changed name with asterisk should match DNS1123 subdomain format")
}
