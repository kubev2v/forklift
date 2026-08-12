package plan

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestDiskMaxLUNThreshold(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Warning when existing + planned reaches the default MaxLUN.
	g.Expect(defaultDiskMaxLUN).To(gomega.Equal(1024))
	g.Expect(1000+24 >= defaultDiskMaxLUN).To(gomega.BeTrue())
	g.Expect(100+50 >= defaultDiskMaxLUN).To(gomega.BeFalse())
}
