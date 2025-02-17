package plan

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestVmNameHandler(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	//Test all cases in name adjustments
	originalVmName := "----------------Vm!@#$%^&*()_+-Name/.is,';[]-CorREct-<>123----------------------"
	newVmName := "vm--name.is-correct-123"
	g.Expect(changeVmName(originalVmName)).To(gomega.Equal(newVmName))

	//Test the case that the VM name is empty after all removals
	emptyVM := ".__."
	newVmNameFromId := "vm-"
	g.Expect(changeVmName(emptyVM)).To(gomega.ContainSubstring(newVmNameFromId))
}
