package resolver

import "strings"

// EnsureScheme prepends "https://" to host if it has no URL scheme.
// This lets a single secret's STORAGE_HOSTNAME work for both plugins that
// require a scheme (csiVolumeImport) and plugins/drivers that accept a bare
// host (xcopy → Trident's ManagementLIF).
func EnsureScheme(host string) string {
	if host == "" || strings.Contains(host, "://") {
		return host
	}
	return "https://" + host
}
