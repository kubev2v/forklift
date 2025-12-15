package ovfbase

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"
)

func TestGetResourceCapacity(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name        string
		capacity    int64
		units       string
		expected    int64
		expectError bool
	}{
		{"empty units", 10, "", 10, false},
		{"byte units", 20, "byte", 20, false},
		{"pow 10", 2, "byte * 2^10", 2048, false},
		{"pow 20", 1, "byte * 2^20", 1048576, false},
		{"missing pow", 2, "byte * 1024", 2048, false},
		{"malformed units", 100, "byte*2^", 0, true},
		{"no spaces", 1, "byte*2^20", 1048576, false},
		{"unsupported units", 10, "kilobytes", 0, true},
		{"unsupported operand", 1, "byte + 2^20", 0, true},
		{"diffrent base", 1, "byte * 3^10", 59049, false},
		{"uncommon format", 1, "byte * 2 * 2^10", 2048, false},
		{"uncommon format no spaces", 1, "byte*3*2^10", 3072, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := getResourceCapacity(testCase.capacity, testCase.units)
			if testCase.expectError {
				g.Expect(err).To(gomega.HaveOccurred(), fmt.Sprintf("expected an error for input: %v", testCase.units))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("did not expect an error for input: %v, but got: %v", testCase.units, err))
				g.Expect(result).To(gomega.Equal(testCase.expected), fmt.Sprintf("expected %v, but got %v", testCase.expected, result))
			}
		})
	}
}
