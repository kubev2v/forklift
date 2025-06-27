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
	newVmName := "vm--name.is-correct-123"
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
}
