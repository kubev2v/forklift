package vsphere

import (
	"testing"

	utils "github.com/konveyor/forklift-controller/pkg/controller/plan/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVsphere(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "vSphere Suite")
}

func TestGetDeviceNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"/dev/sda", 1},
		{"/dev/sdb", 2},
		{"/dev/sdz", 26},
		{"/dev/sda1", 1},
		{"/dev/sda5", 1},
		{"/dev/sdb2", 2},
		{"/dev/sdza", 26},
		{"/dev/sdzb", 26},
		{"/dev/sd", 0},
		{"test", 0},
	}

	for _, test := range tests {
		result := utils.GetDeviceNumber(test.input)
		if result != test.expected {
			t.Errorf("For input '%s', expected %d, but got %d", test.input, test.expected, result)
		}
	}
}
