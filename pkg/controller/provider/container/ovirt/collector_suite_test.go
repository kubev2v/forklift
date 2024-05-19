package ovirt

import (
	"testing"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// forkliftFailHandler call ginkgo.Fail with printing the additional information
func forkliftFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	ginkgo.Fail(message, callerSkip...)
}

func TestTests(t *testing.T) {
	defer ginkgo.GinkgoRecover()
	RegisterFailHandler(forkliftFailHandler)
	ginkgo.RunSpecs(t, "ovirt collector")
}
