package inspection

import (
	"strings"
	"unicode/utf8"
)

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
	// These must be specific enough to avoid false positives in verbose debug
	// output (-v -x / LIBGUESTFS_DEBUG=1), which contains kernel boot messages
	// ("Key type encrypted registered"), TLS cipher config ("cipher list
	// ECDHE+AESGCM"), and kernel module loading ("crypto_engine.ko"). Single
	// words like "encrypted", "cipher", "crypto_", "aes-" are too broad.
	strongIndicators := []string{
		"luks",
		"unknown cipher",
		"requires a passphrase",
		"dm-crypt",
		"cryptsetup",
		"encryption format",
		"encrypted disk",
		"encrypted volume",
		"disk encryption",
		"is encrypted",
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

const maxErrorSummaryLen = 500

// extractErrorSummary pulls the salient error lines from virt-v2v-inspector
// stderr. With -v -x and LIBGUESTFS_DEBUG=1, stderr can be megabytes of debug
// output; this returns only the lines that name the actual failure. Returns ""
// when no error lines are found.
func extractErrorSummary(stderr string) string {
	if stderr == "" {
		return ""
	}

	markers := []string{
		"virt-v2v: error:",
		"virt-v2v-inspector: error:",
		"libguestfs: error:",
		"nbdkit: vddk",
		"vixdisklib",
		"nfc error",
	}
	errorIndicators := []string{"error", "failed", "failure", "cannot", "unable"}

	var errorLines []string
	seen := make(map[string]bool)
	for _, line := range splitOutputLines(stderr) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, marker := range markers {
			if strings.Contains(lower, marker) && containsAny(lower, errorIndicators) && !seen[trimmed] {
				errorLines = append(errorLines, trimmed)
				seen[trimmed] = true
				break
			}
		}
	}

	if len(errorLines) > 0 {
		return truncateRuneSafe(strings.Join(errorLines, "; "), maxErrorSummaryLen)
	}

	// Fallback: last three non-empty lines (error output is typically near the end).
	lines := splitOutputLines(stderr)
	start := 0
	if len(lines) > 3 {
		start = len(lines) - 3
	}
	var tail []string
	for _, l := range lines[start:] {
		if t := strings.TrimSpace(l); t != "" {
			tail = append(tail, t)
		}
	}
	if len(tail) == 0 {
		return ""
	}
	return truncateRuneSafe(strings.Join(tail, "; "), maxErrorSummaryLen)
}

func splitOutputLines(output string) []string {
	return strings.FieldsFunc(output, func(r rune) bool {
		return r == '\r' || r == '\n'
	})
}

func containsAny(s string, values []string) bool {
	for _, value := range values {
		if strings.Contains(s, value) {
			return true
		}
	}
	return false
}

func truncateRuneSafe(s string, max int) string {
	if len(s) <= max {
		return s
	}
	t := s[:max]
	for len(t) > 0 && !utf8.ValidString(t) {
		t = t[:len(t)-1]
	}
	return t + "..."
}
