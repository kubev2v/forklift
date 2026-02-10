package builder

import (
	"testing"
)

func TestFindClosest_ExactMatch(t *testing.T) {
	candidates := []instanceTypeCandidate{
		{Name: "u1.small", Kind: "VirtualMachineClusterInstancetype", VCPUs: 1, MemoryMi: 2048},
		{Name: "u1.medium", Kind: "VirtualMachineClusterInstancetype", VCPUs: 2, MemoryMi: 4096},
		{Name: "u1.large", Kind: "VirtualMachineClusterInstancetype", VCPUs: 2, MemoryMi: 8192},
		{Name: "u1.xlarge", Kind: "VirtualMachineClusterInstancetype", VCPUs: 4, MemoryMi: 16384},
	}

	result := findClosest(candidates, 2, 8192)
	if result.Name != "u1.large" {
		t.Errorf("expected u1.large, got %s", result.Name)
	}
}

func TestFindClosest_NearestMatch(t *testing.T) {
	candidates := []instanceTypeCandidate{
		{Name: "u1.small", Kind: "VirtualMachineClusterInstancetype", VCPUs: 1, MemoryMi: 2048},
		{Name: "u1.medium", Kind: "VirtualMachineClusterInstancetype", VCPUs: 2, MemoryMi: 4096},
		{Name: "u1.large", Kind: "VirtualMachineClusterInstancetype", VCPUs: 2, MemoryMi: 8192},
		{Name: "u1.xlarge", Kind: "VirtualMachineClusterInstancetype", VCPUs: 4, MemoryMi: 16384},
	}

	// Looking for 3 vCPUs with 6000 MiB - should be closest to u1.large (2/8192)
	// or u1.medium (2/4096) - depends on normalized distance
	result := findClosest(candidates, 3, 6000)
	// Normalized distance for u1.medium: sqrt((-1/3)^2 + (-2000/6000)^2) ≈ sqrt(0.111 + 0.111) ≈ 0.471
	// Normalized distance for u1.large: sqrt((-1/3)^2 + (2192/6000)^2) ≈ sqrt(0.111 + 0.134) ≈ 0.494
	// Normalized distance for u1.xlarge: sqrt((1/3)^2 + (10384/6000)^2) ≈ sqrt(0.111 + 2.996) ≈ 1.763
	// So u1.medium is closest
	if result.Name != "u1.medium" {
		t.Errorf("expected u1.medium for (3, 6000), got %s", result.Name)
	}
}

func TestFindClosest_SingleCandidate(t *testing.T) {
	candidates := []instanceTypeCandidate{
		{Name: "only-one", Kind: "VirtualMachineClusterInstancetype", VCPUs: 4, MemoryMi: 8192},
	}

	result := findClosest(candidates, 2, 4096)
	if result.Name != "only-one" {
		t.Errorf("expected only-one, got %s", result.Name)
	}
}

func TestFindClosest_ZeroTarget(t *testing.T) {
	// Edge case: target values are zero
	candidates := []instanceTypeCandidate{
		{Name: "u1.small", Kind: "VirtualMachineClusterInstancetype", VCPUs: 1, MemoryMi: 512},
		{Name: "u1.large", Kind: "VirtualMachineClusterInstancetype", VCPUs: 8, MemoryMi: 32768},
	}

	result := findClosest(candidates, 0, 0)
	// With zero targets, normalization uses 1 as denominator
	// u1.small: sqrt(1^2 + 512^2) ≈ 512
	// u1.large: sqrt(8^2 + 32768^2) ≈ 32768
	if result.Name != "u1.small" {
		t.Errorf("expected u1.small for zero target, got %s", result.Name)
	}
}

func TestIsNoMatchError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "no matches for kind",
			err:      &testError{"no matches for kind \"Template\" in version \"template.openshift.io/v1\""},
			expected: true,
		},
		{
			name:     "no match for kind",
			err:      &testError{"no match for kind \"Template\""},
			expected: true,
		},
		{
			name:     "server could not find",
			err:      &testError{"the server could not find the requested resource"},
			expected: true,
		},
		{
			name:     "other error",
			err:      &testError{"connection refused"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNoMatchError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
