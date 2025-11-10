package output

import (
	"fmt"
	"regexp"
	"strings"
)

// ANSI color codes
const (
	Reset       = "\033[0m"
	BoldText    = "\033[1m"
	RedColor    = "\033[31m"
	GreenColor  = "\033[32m"
	YellowColor = "\033[33m"
	BlueColor   = "\033[34m"
	PurpleColor = "\033[35m"
	CyanColor   = "\033[36m"
	White       = "\033[37m"
	BoldRed     = "\033[1;31m"
	BoldGreen   = "\033[1;32m"
	BoldYellow  = "\033[1;33m"
	BoldBlue    = "\033[1;34m"
)

// ansiRegex is a regular expression that matches ANSI color escape codes
var ansiRegex = regexp.MustCompile("\033\\[[0-9;]*m")

// Bold returns a bold-formatted string
func Bold(text string) string {
	return ColorizedString(text, BoldText)
}

// ColorizedString returns a string with the specified color applied
func ColorizedString(text string, color string) string {
	return color + text + Reset
}

// Yellow returns a yellow-colored string
func Yellow(text string) string {
	return ColorizedString(text, YellowColor)
}

// Green returns a green-colored string
func Green(text string) string {
	return ColorizedString(text, GreenColor)
}

// Red returns a red-colored string
func Red(text string) string {
	return ColorizedString(text, RedColor)
}

// Blue returns a blue-colored string
func Blue(text string) string {
	return ColorizedString(text, BlueColor)
}

// Cyan returns a cyan-colored string
func Cyan(text string) string {
	return ColorizedString(text, CyanColor)
}

// StripANSI removes ANSI color codes from a string
func StripANSI(text string) string {
	return ansiRegex.ReplaceAllString(text, "")
}

// VisibleLength returns the visible length of a string, excluding ANSI color codes
func VisibleLength(text string) int {
	return len(StripANSI(text))
}

// ColorizeStatus returns a colored string based on status value
func ColorizeStatus(status string) string {
	status = strings.TrimSpace(status)
	switch strings.ToLower(status) {
	case "running":
		return Blue(status)
	case "executing":
		return Blue(status)
	case "completed":
		return Green(status)
	case "pending":
		return Yellow(status)
	case "failed":
		return Red(status)
	case "canceled":
		return Cyan(status)
	default:
		return status
	}
}

// ColorizeNumber returns a blue-colored number for migration progress
func ColorizeNumber(number interface{}) string {
	return Blue(fmt.Sprintf("%v", number))
}

// ColorizeBoolean returns a colored string based on boolean value
func ColorizeBoolean(b bool) string {
	if b {
		return Green(fmt.Sprintf("%t", b))
	}
	return fmt.Sprintf("%t", b)
}

// ColorizedSeparator returns a separator line with the specified color
func ColorizedSeparator(length int, color string) string {
	return ColorizedString(strings.Repeat("=", length), color)
}
