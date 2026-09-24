package ovfbase

import (
	"fmt"
	"testing"

	"github.com/onsi/gomega"
)

func TestDiskCapacityMB(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tests := []struct {
		name        string
		capacity    int64
		units       string
		expected    int64
		expectError bool
	}{
		{
			name:     "raw bytes (1nisim-rhel9-efi)",
			capacity: 10737418240,
			units:    "byte",
			expected: 10240,
		},
		{
			name:     "gibibytes (ameen-opensuse-btrfs)",
			capacity: 30,
			units:    "byte * 2^30",
			expected: 30720,
		},
		{
			name:     "byte * 2^20",
			capacity: 1,
			units:    "byte * 2^20",
			expected: 1,
		},
		{
			name:        "unsupported units",
			capacity:    10,
			units:       "kilobytes",
			expectError: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := diskCapacityMB(testCase.capacity, testCase.units)
			if testCase.expectError {
				g.Expect(err).To(gomega.HaveOccurred(), fmt.Sprintf("expected an error for input: %v", testCase.units))
			} else {
				g.Expect(err).ToNot(gomega.HaveOccurred(), fmt.Sprintf("did not expect an error for input: %v, but got: %v", testCase.units, err))
				g.Expect(result).To(gomega.Equal(testCase.expected), fmt.Sprintf("expected %v, but got %v", testCase.expected, result))
			}
		})
	}
}
