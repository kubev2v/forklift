package templateutil

import (
	"strings"
	"unicode"
)

// ToLower safely converts a string to lowercase, handling nil values.
func ToLower(s interface{}) string {
	if s == nil {
		return ""
	}
	if str, ok := s.(string); ok {
		return strings.ToLower(str)
	}
	return ""
}

// ToUpper safely converts a string to uppercase, handling nil values.
func ToUpper(s interface{}) string {
	if s == nil {
		return ""
	}
	if str, ok := s.(string); ok {
		return strings.ToUpper(str)
	}
	return ""
}

// Contains safely checks if a string contains a substring.
func Contains(s, substr interface{}) bool {
	if s == nil || substr == nil {
		return false
	}
	str, ok1 := s.(string)
	subStr, ok2 := substr.(string)
	if !ok1 || !ok2 {
		return false
	}
	return strings.Contains(str, subStr)
}

// Replace safely replaces occurrences of old with new in a string.
func Replace(s, old, new interface{}, n int) string {
	if s == nil || old == nil || new == nil {
		return ""
	}
	str, ok1 := s.(string)
	oldStr, ok2 := old.(string)
	newStr, ok3 := new.(string)
	if !ok1 || !ok2 || !ok3 {
		return ""
	}
	return strings.Replace(str, oldStr, newStr, n)
}

// Trim safely removes whitespace from both ends of a string, handling nil values.
func Trim(s interface{}) string {
	if s == nil {
		return ""
	}
	if str, ok := s.(string); ok {
		return strings.TrimSpace(str)
	}
	return ""
}

// TrimAll safely removes the specified cutset from both ends of a string.
func TrimAll(cutset, s interface{}) string {
	if s == nil || cutset == nil {
		return ""
	}
	str, ok1 := s.(string)
	cutsetStr, ok2 := cutset.(string)
	if !ok1 || !ok2 {
		return ""
	}
	return strings.Trim(str, cutsetStr)
}

// TrimSuffix safely removes a suffix from a string if present.
func TrimSuffix(suffix, s interface{}) string {
	if s == nil || suffix == nil {
		return ""
	}
	str, ok1 := s.(string)
	suffixStr, ok2 := suffix.(string)
	if !ok1 || !ok2 {
		return ""
	}
	return strings.TrimSuffix(str, suffixStr)
}

// TrimPrefix safely removes a prefix from a string if present.
func TrimPrefix(prefix, s interface{}) string {
	if s == nil || prefix == nil {
		return ""
	}
	str, ok1 := s.(string)
	prefixStr, ok2 := prefix.(string)
	if !ok1 || !ok2 {
		return ""
	}
	return strings.TrimPrefix(str, prefixStr)
}

// Title safely converts a string to title case.
func Title(s interface{}) string {
	if s == nil {
		return ""
	}
	if str, ok := s.(string); ok {
		b := []byte(str)
		previous := ' '
		for i := 0; i < len(b); i++ {
			if unicode.IsLetter(rune(b[i])) && unicode.IsSpace(previous) {
				b[i] = byte(unicode.ToUpper(rune(b[i])))
			}
			previous = rune(b[i])
		}
		return string(b)
	}
	return ""
}

// Repeat safely repeats a string n times.
func Repeat(count interface{}, s interface{}) string {
	if s == nil || count == nil {
		return ""
	}
	str, ok1 := s.(string)
	n, ok2 := count.(int)
	if !ok1 || !ok2 || n < 0 {
		return ""
	}
	return strings.Repeat(str, n)
}

// Substr safely extracts a substring from a string.
func Substr(start, end interface{}, s interface{}) string {
	if s == nil || start == nil || end == nil {
		return ""
	}
	str, ok1 := s.(string)
	startPos, ok2 := start.(int)
	endPos, ok3 := end.(int)
	if !ok1 || !ok2 || !ok3 {
		return ""
	}

	strLen := len(str)
	if startPos < 0 {
		startPos = 0
	}
	if endPos > strLen {
		endPos = strLen
	}
	if startPos > endPos {
		return ""
	}
	return str[startPos:endPos]
}

// Nospace safely removes all whitespace from a string.
func Nospace(s interface{}) string {
	if s == nil {
		return ""
	}
	if str, ok := s.(string); ok {
		return strings.ReplaceAll(strings.ReplaceAll(str, " ", ""), "\t", "")
	}
	return ""
}

// Trunc safely truncates a string.
func Trunc(length interface{}, s interface{}) string {
	if s == nil || length == nil {
		return ""
	}
	str, ok1 := s.(string)
	n, ok2 := length.(int)
	if !ok1 || !ok2 {
		return ""
	}

	strLen := len(str)
	if n < 0 {
		// Negative length means truncate from the beginning
		n = -n
		if n > strLen {
			return ""
		}
		return str[strLen-n:]
	}
	// Positive length means truncate from the end
	if n > strLen {
		return str
	}
	return str[:n]
}

// Initials safely extracts the first letter of each word in a string.
func Initials(s interface{}) string {
	if s == nil {
		return ""
	}
	if str, ok := s.(string); ok {
		words := strings.Fields(str)
		initials := ""
		for _, word := range words {
			if len(word) > 0 {
				initials += string(word[0])
			}
		}
		return initials
	}
	return ""
}

// HasPrefix safely checks if a string starts with a prefix.
func HasPrefix(prefix, s interface{}) bool {
	if s == nil || prefix == nil {
		return false
	}
	str, ok1 := s.(string)
	prefixStr, ok2 := prefix.(string)
	if !ok1 || !ok2 {
		return false
	}
	return strings.HasPrefix(str, prefixStr)
}

// HasSuffix safely checks if a string ends with a suffix.
func HasSuffix(suffix, s interface{}) bool {
	if s == nil || suffix == nil {
		return false
	}
	str, ok1 := s.(string)
	suffixStr, ok2 := suffix.(string)
	if !ok1 || !ok2 {
		return false
	}
	return strings.HasSuffix(str, suffixStr)
}
