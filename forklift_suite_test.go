package forklift_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestForklift(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forklift Suite")
}
