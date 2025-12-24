package util

import "strings"

// ParseSMBSource converts provider URL to SMB source format.
// Input: smb://server/share, //server/share, or \\server\share (Windows UNC)
// Output: //server/share
func ParseSMBSource(providerURL string) string {
	// Normalize Windows UNC paths (\\server\share) to forward slashes
	source := strings.ReplaceAll(providerURL, "\\", "/")

	// Remove smb:// prefix if present
	source = strings.TrimPrefix(source, "smb://")
	source = strings.TrimPrefix(source, "smb:")

	// Ensure it starts with //
	if !strings.HasPrefix(source, "//") {
		source = "//" + source
	}
	return source
}
