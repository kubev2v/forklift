package inspection

import "strings"

// isEncryptedDiskError returns (true, reason) when the output looks like an
// encrypted-disk failure, or (false, "") otherwise. The reason string names
// the specific pattern that matched, which is useful for distinguishing real
// decryption failures from false-positives in libguestfs debug output.
func isEncryptedDiskError(output string) (bool, string) {
	lowerOutput := strings.ToLower(output)

	// Require at least one generic error signal before doing anything else.
	hasError := strings.Contains(lowerOutput, "error") ||
		strings.Contains(lowerOutput, "failed") ||
		strings.Contains(lowerOutput, "could not open")
	if !hasError {
		return false, ""
	}

	// Strong encryption indicators checked before access-rights exclusions.
	strongIndicators := []string{
		"encryption",
		"encrypted",
		"luks",
		"unknown cipher",
		"requires a passphrase",
		"dm-crypt",
		"cryptsetup",
		"crypto_",
		"cipher",
		"aes-",
		"encryption format",
		"encrypted disk",
		"encrypted volume",
	}
	for _, indicator := range strongIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true, indicator
		}
	}

	// "Could not open backing image" + VixDiskLib + access rights → likely encrypted.
	if strings.Contains(lowerOutput, "could not open backing image") &&
		strings.Contains(lowerOutput, "vixdisklib_open") &&
		strings.Contains(lowerOutput, "you do not have access rights") {
		return true, "could not open backing image + vixdisklib_open + access rights"
	}

	// "Requested export not available" + VixDiskLib + access rights → likely encrypted.
	if strings.Contains(lowerOutput, "requested export not available") &&
		strings.Contains(lowerOutput, "vixdisklib_open") &&
		strings.Contains(lowerOutput, "you do not have access rights") {
		return true, "requested export not available + vixdisklib_open + access rights"
	}

	// Exclude plain access-rights errors that are not encryption-related.
	if strings.Contains(lowerOutput, "you do not have access rights") ||
		strings.Contains(lowerOutput, "access denied") ||
		strings.Contains(lowerOutput, "permission denied") {
		return false, ""
	}

	// "Could not open backing image" with any encryption context.
	if strings.Contains(lowerOutput, "could not open backing image") {
		if strings.Contains(lowerOutput, "encrypt") ||
			strings.Contains(lowerOutput, "luks") ||
			strings.Contains(lowerOutput, "cipher") {
			return true, "could not open backing image + encryption context"
		}
	}

	// Garbled multibyte data often means the tool is reading encrypted raw bytes.
	if strings.Contains(lowerOutput, "invalid or incomplete multibyte") ||
		strings.Contains(lowerOutput, "invalid multibyte") {
		return true, "invalid multibyte sequence"
	}

	return false, ""
}
