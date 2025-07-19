package vsphere

import (
	"fmt"
	"regexp"
	"strings"
)

// UUIDToVMwareSerial converts a UUID string to VMware serial format
// Input: "422c6a2a-5ea9-1083-39f3-3b140fffb444"
// Output: "VMware-42 2c 6a 2a 5e a9 10 83-39 f3 3b 14 0f ff b4 44"
func UUIDToVMwareSerial(uuid string) string {
	// Validate UUID format using regex
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	if !uuidRegex.MatchString(uuid) {
		// Fallback to original UUID if it doesn't match the expected format
		return uuid
	}

	// Remove hyphens from UUID
	cleanUUID := strings.ReplaceAll(uuid, "-", "")

	// Convert to lowercase for consistency
	cleanUUID = strings.ToLower(cleanUUID)

	// Insert spaces every 2 characters
	var spacedUUID strings.Builder
	for i := 0; i < len(cleanUUID); i += 2 {
		if i > 0 {
			spacedUUID.WriteString(" ")
		}
		spacedUUID.WriteString(cleanUUID[i : i+2])
	}

	// Split the spaced string into two parts
	spacedString := spacedUUID.String()
	firstPart := spacedString[:23]  // "42 2c 6a 2a 5e a9 10 83"
	secondPart := spacedString[24:] // "39 f3 3b 14 0f ff b4 44"

	// Combine with VMware prefix and separator
	return fmt.Sprintf("VMware-%s-%s", firstPart, secondPart)
}
