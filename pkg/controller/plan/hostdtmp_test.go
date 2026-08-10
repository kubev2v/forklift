package plan

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestShouldWarnHostdTmp(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	g.Expect(shouldWarnHostdTmp(9)).To(gomega.BeFalse())
	g.Expect(shouldWarnHostdTmp(10)).To(gomega.BeTrue())
	g.Expect(shouldWarnHostdTmp(25)).To(gomega.BeTrue())
}
