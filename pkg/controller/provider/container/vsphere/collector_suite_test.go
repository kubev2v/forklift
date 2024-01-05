package vsphere

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// forkliftFailHandler call ginkgo.Fail with printing the additional information
func forkliftFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	Fail(message, callerSkip...)
}

func TestTests(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(forkliftFailHandler)
	RunSpecs(t, "vSphere controller")
}
