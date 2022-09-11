package plan

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestKubevirt(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	id := "1234-5678"

	//Test all cases in name adjustments
	originalVmName := "----------------Vm!@#$%^&*()_+-Name/.,';[]-CorREct-<>123----------------------"
	newVmName := "vm-name-correct-123"
	g.Expect(changeVmName(originalVmName, id)).To(gomega.Equal(newVmName))

	//Test the case that the VM name is empty after all removals
	emptyVM := ".__."
	newVmNameFromId := "vm-1234-5678"
	g.Expect(changeVmName(emptyVM, id)).To(gomega.Equal(newVmNameFromId))
}
