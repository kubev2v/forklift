package base

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const MaxDiskSerialLength = 20

// DiskSerial returns a serial number for a KubeVirt disk device.
// If sourceID is non-empty it is sanitized and truncated to
// MaxDiskSerialLength. Otherwise a deterministic 20-character hex
// serial is derived from vmID and diskKey.
func DiskSerial(sourceID, vmID string, diskKey int) string {
	if s := sanitizeSerial(sourceID); s != "" {
		return truncate(s, MaxDiskSerialLength)
	}
	return generateSerial(vmID, diskKey)
}

// sanitizeSerial removes characters that are unsafe for QEMU/libvirt
// disk serials, keeping only alphanumerics and hyphens.
func sanitizeSerial(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func generateSerial(vmID string, diskKey int) string {
	h := sha256.Sum256(fmt.Appendf(nil, "%s:%d", vmID, diskKey))
	return hex.EncodeToString(h[:])[:MaxDiskSerialLength]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
