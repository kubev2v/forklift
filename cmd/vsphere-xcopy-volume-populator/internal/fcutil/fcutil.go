package fcutil

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseFCAdapter parses an ESX FC adapter ID in format "fc.WWNN:WWPN"
// and returns the WWNN and WWPN separately (unformatted hex strings).
//
// Example:
//   input: "fc.2000000000000001:2100000000000001"
//   output: wwnn="2000000000000001", wwpn="2100000000000001", err=nil
//
// The returned WWNN and WWPN are uppercase hex strings without formatting.
func ParseFCAdapter(fcID string) (wwnn, wwpn string, err error) {
	if !strings.HasPrefix(fcID, "fc.") {
		return "", "", fmt.Errorf("FC adapter ID %q doesn't start with 'fc.'", fcID)
	}

	// Remove "fc." prefix and split by ":"
	parts := strings.Split(fcID[3:], ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("FC adapter ID %q is not in expected fc.WWNN:WWPN format", fcID)
	}

	wwnn = strings.ToUpper(parts[0])
	wwpn = strings.ToUpper(parts[1])

	if len(wwnn) == 0 || len(wwpn) == 0 {
		return "", "", fmt.Errorf("FC adapter ID %q has empty WWNN or WWPN", fcID)
	}

	// Validate that WWN parts have even length (required for byte-pair formatting)
	if len(wwnn)%2 != 0 {
		return "", "", fmt.Errorf("WWNN %q has odd length", wwnn)
	}
	if len(wwpn)%2 != 0 {
		return "", "", fmt.Errorf("WWPN %q has odd length", wwpn)
	}

	// Validate hex format
	hexPattern := regexp.MustCompile(`^[0-9A-Fa-f]+$`)
	if !hexPattern.MatchString(wwnn) {
		return "", "", fmt.Errorf("WWNN %q contains non-hex characters", wwnn)
	}
	if !hexPattern.MatchString(wwpn) {
		return "", "", fmt.Errorf("WWPN %q contains non-hex characters", wwpn)
	}

	return wwnn, wwpn, nil
}

// FormatWWNWithColons formats a WWN hex string by inserting colons every 2 characters.
//
// Example:
//   input: "2100000000000001"
//   output: "21:00:00:00:00:00:00:01"
//
// The input should be an uppercase hex string with even length.
// If the input has odd length, the last character will be in its own segment.
func FormatWWNWithColons(wwn string) string {
	if len(wwn) == 0 {
		return ""
	}

	formatted := make([]string, 0, (len(wwn)+1)/2)
	for i := 0; i < len(wwn); i += 2 {
		end := i + 2
		if end > len(wwn) {
			end = len(wwn)
		}
		formatted = append(formatted, wwn[i:end])
	}
	return strings.Join(formatted, ":")
}

// NormalizeWWN removes all formatting characters (colons, dashes, spaces) and uppercases.
// This is useful for comparing WWNs from different sources that may use different formatting.
//
// Example:
//   input: "21:00:00:00:00:00:00:01"
//   output: "2100000000000001"
//
// Example:
//   input: "21-00-00-00-00-00-00-01"
//   output: "2100000000000001"
func NormalizeWWN(wwn string) string {
	cleaned := strings.ReplaceAll(wwn, ":", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	return strings.ToUpper(cleaned)
}

// ExtractAndFormatWWPN is a convenience function that extracts the WWPN from an
// ESX FC adapter ID and formats it with colons.
//
// This is the most common operation needed by storage backends.
//
// Example:
//   input: "fc.2000000000000001:2100000000000001"
//   output: "21:00:00:00:00:00:00:01"
func ExtractAndFormatWWPN(fcID string) (string, error) {
	_, wwpn, err := ParseFCAdapter(fcID)
	if err != nil {
		return "", err
	}
	return FormatWWNWithColons(wwpn), nil
}

// ExtractWWPN extracts the WWPN from an ESX FC adapter ID without formatting.
//
// Example:
//   input: "fc.2000000000000001:2100000000000001"
//   output: "2100000000000001"
func ExtractWWPN(fcID string) (string, error) {
	_, wwpn, err := ParseFCAdapter(fcID)
	return wwpn, err
}

// CompareWWNs compares two WWN strings, normalizing them first to ignore formatting differences.
// Returns true if the WWNs are equivalent.
//
// Example:
//   CompareWWNs("21:00:00:00:00:00:00:01", "2100000000000001") // returns true
//   CompareWWNs("21-00-00-00-00-00-00-01", "21:00:00:00:00:00:00:01") // returns true
func CompareWWNs(wwn1, wwn2 string) bool {
	return NormalizeWWN(wwn1) == NormalizeWWN(wwn2)
}
