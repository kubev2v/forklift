package inspection

import "strings"

// isEncryptedDiskError checks if the error output indicates an encrypted disk
// Common patterns:
// - Direct encryption/LUKS mentions
// - QEMU errors about encrypted formats
// - Cipher-related errors
// - Passphrase requirements
func isEncryptedDiskError(output string) bool {
	// Convert to lowercase for case-insensitive matching
	lowerOutput := strings.ToLower(output)

	// Must contain "error" or "failed" to be considered an error condition
	hasError := strings.Contains(lowerOutput, "error") ||
		strings.Contains(lowerOutput, "failed") ||
		strings.Contains(lowerOutput, "could not open")

	if !hasError {
		return false
	}

	// Strong encryption indicators (specific to encryption) - check these FIRST
	// This ensures both virt-inspector and virt-v2v-inspector can detect encryption
	strongIndicators := []string{
		"encryption",            // Direct encryption mention
		"encrypted",             // Direct encrypted mention
		"luks",                  // LUKS encryption
		"unknown cipher",        // Cipher-related errors
		"requires a passphrase", // Passphrase requirement
		"dm-crypt",              // Device-mapper crypto
		"cryptsetup",            // Cryptsetup tool
		"crypto_",               // Crypto-related functions
		"cipher",                // Cipher operations
		"aes-",                  // AES encryption
		"encryption format",     // QEMU encryption format errors
		"encrypted disk",        // Direct mention
		"encrypted volume",      // Direct mention
	}

	// Check for strong indicators FIRST (before excluding access rights)
	for _, indicator := range strongIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}

	// Pattern: "Could not open backing image" + "VixDiskLib_Open" + "you do not have access rights"
	// This combination typically indicates encrypted disk, not just permissions
	if strings.Contains(lowerOutput, "could not open backing image") &&
		strings.Contains(lowerOutput, "vixdisklib_open") &&
		strings.Contains(lowerOutput, "you do not have access rights") {
		return true
	}

	// Pattern: "Requested export not available" + "VixDiskLib_Open" + "you do not have access rights"
	// This also indicates encrypted disk
	if strings.Contains(lowerOutput, "requested export not available") &&
		strings.Contains(lowerOutput, "vixdisklib_open") &&
		strings.Contains(lowerOutput, "you do not have access rights") {
		return true
	}

	// Exclude false positives - these are NOT encryption errors
	// Access rights/permissions errors from VDDK (but only if not combined with the patterns above)
	if strings.Contains(lowerOutput, "you do not have access rights") ||
		strings.Contains(lowerOutput, "access denied") ||
		strings.Contains(lowerOutput, "permission denied") {
		return false
	}

	// Pattern: QEMU reports "could not open backing image" with encryption context
	if strings.Contains(lowerOutput, "could not open backing image") {
		// Only if there's encryption context, not access rights issues
		if strings.Contains(lowerOutput, "encrypt") ||
			strings.Contains(lowerOutput, "luks") ||
			strings.Contains(lowerOutput, "cipher") {
			return true
		}
	}

	// Pattern: "invalid or incomplete multibyte" often appears with encrypted disks
	// when trying to read filesystem metadata that's actually encrypted data
	if strings.Contains(lowerOutput, "invalid or incomplete multibyte") ||
		strings.Contains(lowerOutput, "invalid multibyte") {
		return true
	}

	return false
}
