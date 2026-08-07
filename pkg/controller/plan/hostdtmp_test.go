package plan

import (
	"testing"

	"github.com/onsi/gomega"
)

func TestHostdTmpDiskWarnThreshold(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	g.Expect(defaultHostdTmpMB).To(gomega.Equal(500))
	g.Expect(hostdTmpDiskWarnThreshold).To(gomega.Equal(10))
	g.Expect(10 >= hostdTmpDiskWarnThreshold).To(gomega.BeTrue())
	g.Expect(9 >= hostdTmpDiskWarnThreshold).To(gomega.BeFalse())
}
