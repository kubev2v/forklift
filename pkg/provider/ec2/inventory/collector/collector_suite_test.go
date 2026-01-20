package collector

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// forkliftFailHandler calls ginkgo.Fail with printing the additional information
func forkliftFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	Fail(message, callerSkip...)
}

func TestCollector(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(forkliftFailHandler)
	RunSpecs(t, "EC2 collector")
}
