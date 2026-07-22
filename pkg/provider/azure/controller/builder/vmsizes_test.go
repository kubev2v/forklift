package builder

import (
	"testing"

	"github.com/kubev2v/forklift/pkg/lib/logging"
)

func TestMapVMSize_KnownSizes(t *testing.T) {
	b := &Builder{log: logging.WithName("test")}

	tests := []struct {
		size      string
		wantCPU   int32
		wantMemMB int64
	}{
		{"Standard_B1s", 1, 1024},
		{"Standard_B2ms", 2, 8192},
		{"Standard_D2s_v3", 2, 8192},
		{"Standard_D16s_v3", 16, 65536},
		{"Standard_E64s_v3", 64, 524288},
		{"Standard_F4s_v2", 4, 8192},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			cpu, mem := b.mapVMSize(tt.size)
			if cpu != tt.wantCPU {
				t.Errorf("mapVMSize(%q) cpu = %d, want %d", tt.size, cpu, tt.wantCPU)
			}
			if mem != tt.wantMemMB {
				t.Errorf("mapVMSize(%q) mem = %d, want %d", tt.size, mem, tt.wantMemMB)
			}
		})
	}
}

func TestMapVMSize_ParsesFallback(t *testing.T) {
	b := &Builder{log: logging.WithName("test")}

	tests := []struct {
		size    string
		wantCPU int32
	}{
		{"Standard_D4s_v5", 4},
		{"Standard_E8s_v5", 8},
		{"Standard_D32as_v4", 32},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			cpu, mem := b.mapVMSize(tt.size)
			if cpu != tt.wantCPU {
				t.Errorf("mapVMSize(%q) cpu = %d, want %d", tt.size, cpu, tt.wantCPU)
			}
			expectedMem := int64(tt.wantCPU) * 4096
			if mem != expectedMem {
				t.Errorf("mapVMSize(%q) mem = %d, want %d", tt.size, mem, expectedMem)
			}
		})
	}
}

func TestMapVMSize_UnknownFallsToDefault(t *testing.T) {
	b := &Builder{log: logging.WithName("test")}

	cpu, mem := b.mapVMSize("SomeWeirdSize")
	if cpu != 2 {
		t.Errorf("mapVMSize(unknown) cpu = %d, want 2", cpu)
	}
	if mem != 8192 {
		t.Errorf("mapVMSize(unknown) mem = %d, want 8192", mem)
	}
}
