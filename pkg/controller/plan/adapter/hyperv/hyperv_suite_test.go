package hyperv

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHyperV(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HyperV Suite")
}
