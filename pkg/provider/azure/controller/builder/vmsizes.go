package builder

import (
	"fmt"
	"strings"
)

// vmSizeSpec defines resource allocation for an Azure VM size.
type vmSizeSpec struct {
	vcpus     int32
	memoryMiB int64
}

var vmSizeSpecs = map[string]vmSizeSpec{
	"Standard_B1s":     {1, 1024},
	"Standard_B1ms":    {1, 2048},
	"Standard_B2s":     {2, 4096},
	"Standard_B2ms":    {2, 8192},
	"Standard_B4ms":    {4, 16384},
	"Standard_B8ms":    {8, 32768},
	"Standard_D2s_v3":  {2, 8192},
	"Standard_D4s_v3":  {4, 16384},
	"Standard_D8s_v3":  {8, 32768},
	"Standard_D16s_v3": {16, 65536},
	"Standard_D32s_v3": {32, 131072},
	"Standard_D48s_v3": {48, 196608},
	"Standard_D64s_v3": {64, 262144},
	"Standard_E2s_v3":  {2, 16384},
	"Standard_E4s_v3":  {4, 32768},
	"Standard_E8s_v3":  {8, 65536},
	"Standard_E16s_v3": {16, 131072},
	"Standard_E32s_v3": {32, 262144},
	"Standard_E64s_v3": {64, 524288},
	"Standard_F2s_v2":  {2, 4096},
	"Standard_F4s_v2":  {4, 8192},
	"Standard_F8s_v2":  {8, 16384},
	"Standard_F16s_v2": {16, 32768},
	"Standard_F32s_v2": {32, 65536},
	"Standard_F64s_v2": {64, 131072},
}

// mapVMSize extracts resource allocation from the Azure VM size string.
// Falls back to parsing the numeric portion of the size name if not in the known map.
func (r *Builder) mapVMSize(vmSize string) (vcpus int32, memoryMiB int64) {
	if spec, ok := vmSizeSpecs[vmSize]; ok {
		return spec.vcpus, spec.memoryMiB
	}

	// Try to parse the number from size (e.g., "Standard_D2s_v5" -> 2 cores)
	vcpus = 2
	memoryMiB = 8192

	parts := strings.Split(vmSize, "_")
	for _, part := range parts {
		for i, c := range part {
			if c >= '0' && c <= '9' {
				numStr := ""
				for j := i; j < len(part) && part[j] >= '0' && part[j] <= '9'; j++ {
					numStr += string(part[j])
				}
				var n int
				if _, err := fmt.Sscanf(numStr, "%d", &n); err == nil && n > 0 && n <= 128 {
					vcpus = int32(n)
					memoryMiB = int64(n) * 4096
				}
				break
			}
		}
		if vcpus != 2 {
			break
		}
	}

	r.log.V(1).Info("Mapped VM size", "size", vmSize, "vcpus", vcpus, "memoryMiB", memoryMiB)
	return
}
