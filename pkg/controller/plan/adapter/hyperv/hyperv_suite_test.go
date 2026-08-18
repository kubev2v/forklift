package hyperv

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHyperv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HyperV Suite")
}
