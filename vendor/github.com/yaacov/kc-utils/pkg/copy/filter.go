package copy

import (
	"fmt"
	"strings"
)

// normalizeDiskPath strips a snapshot delta suffix (-NNNNNN.vmdk → .vmdk)
// so lease backing names match base paths from inventory / source_disks.
func normalizeDiskPath(path string) string {
	const suffix = ".vmdk"
	if !strings.HasSuffix(path, suffix) {
		return path
	}
	prefix := path[:len(path)-len(suffix)]
	if len(prefix) >= 7 && prefix[len(prefix)-7] == '-' && isSixDigits(prefix[len(prefix)-6:]) {
		return prefix[:len(prefix)-7] + suffix
	}
	return path
}

func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < 6; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// FilterDiskURLs returns lease disks matching sources, in source order.
// Paths are compared after normalizeDiskPath. Lease disks not listed are skipped.
// Empty sources copies every lease disk in lease order.
func FilterDiskURLs(lease []DiskURL, sources []string) ([]DiskURL, error) {
	if len(sources) == 0 {
		out := make([]DiskURL, len(lease))
		copy(out, lease)
		return out, nil
	}
	byPath := make(map[string]DiskURL, len(lease))
	for _, d := range lease {
		key := normalizeDiskPath(d.DiskPath)
		if key == "" {
			continue
		}
		if _, exists := byPath[key]; !exists {
			byPath[key] = d
		}
	}

	selected := make([]DiskURL, 0, len(sources))
	for _, src := range sources {
		key := normalizeDiskPath(strings.TrimSpace(src))
		if key == "" {
			return nil, fmt.Errorf("empty source disk path in source_disks")
		}
		d, ok := byPath[key]
		if !ok {
			return nil, fmt.Errorf("source disk %q not found in NFC lease", src)
		}
		selected = append(selected, d)
	}
	return selected, nil
}

// SplitDiskPath splits a comma-separated disk path string into individual paths.
func SplitDiskPath(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var paths []string
	for _, part := range strings.Split(raw, ",") {
		if path := strings.TrimSpace(part); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
