package output

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
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

// colorEnabled controls whether ANSI color codes are emitted.
// Defaults to true; set to false via SetColorEnabled for terminals that
// don't support colors or when the --no-color flag / NO_COLOR env var is set.
var colorEnabled = true

// SetColorEnabled globally enables or disables ANSI color output.
func SetColorEnabled(enabled bool) { colorEnabled = enabled }

// IsColorEnabled reports whether ANSI color output is currently enabled.
func IsColorEnabled() bool { return colorEnabled }

// Bold returns a bold-formatted string
func Bold(text string) string {
	return ColorizedString(text, BoldText)
}

// ColorizedString returns a string with the specified color applied.
// When color output is disabled, the text is returned unchanged.
func ColorizedString(text string, color string) string {
	if !colorEnabled {
		return text
	}
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

// VisibleLength returns the visible rune count of a string, excluding ANSI color codes
func VisibleLength(text string) int {
	return utf8.RuneCountInString(StripANSI(text))
}

// ColorizeStatus returns a colored string based on status value.
// Handles migration-phase statuses (Running, Completed, Failed, ...),
// general resource statuses (Ready, Not Ready, Unknown, ...),
// and cloud provider states (stopped, available, terminated, ...).
func ColorizeStatus(status string) string {
	status = strings.TrimSpace(status)
	switch strings.ToLower(status) {
	case "running", "executing", "in-use":
		return Blue(status)
	case "completed", "succeeded", "ready", "available", "bound":
		return Green(status)
	case "pending", "stopped", "stopping", "creating", "unknown":
		return Yellow(status)
	case "failed", "not ready", "terminated", "shutting-down", "deleting", "error", "lost":
		return Red(status)
	case "canceled":
		return Cyan(status)
	default:
		return status
	}
}

// ColorizeCategory returns a colored string based on condition category.
func ColorizeCategory(category string) string {
	category = strings.TrimSpace(category)
	switch strings.ToLower(category) {
	case "critical", "error":
		return Red(category)
	case "warn":
		return Yellow(category)
	case "advisory", "information", "required":
		return Green(category)
	default:
		return category
	}
}

// ColorizePowerState returns a colored string based on VM power state.
// Handles both descriptive states (Running/Stopped) and short forms (On/Off)
// as set by augmentVMInfo's powerStateHuman field.
func ColorizePowerState(state string) string {
	state = strings.TrimSpace(state)
	switch strings.ToLower(state) {
	case "running", "on":
		return Green(state)
	case "stopped", "off":
		return Yellow(state)
	case "not found":
		return Red(state)
	default:
		return state
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

// ColorizeConditionStatus returns a colored string for Kubernetes condition status values
func ColorizeConditionStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "True":
		return Green(status)
	case "False":
		return Red(status)
	default:
		return status
	}
}

// ColorizeBooleanString returns a colored string for string representations of booleans
func ColorizeBooleanString(val string) string {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "true", "yes":
		return Green(val)
	case "false", "no":
		return Red(val)
	default:
		return val
	}
}

// ColorizeProgress returns a colored string based on percentage thresholds.
// Expects strings like "85.0%" or "100.0%".
func ColorizeProgress(progress string) string {
	trimmed := strings.TrimSpace(progress)
	numStr := strings.TrimRight(trimmed, "%")
	pct, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return progress
	}
	if pct >= 100 {
		return Green(progress)
	} else if pct >= 75 {
		return Blue(progress)
	} else if pct >= 25 {
		return Yellow(progress)
	}
	return Cyan(progress)
}

// TruncateANSI truncates text to maxWidth visible characters while preserving
// ANSI color codes. Appends "..." and a Reset code when truncation occurs.
func TruncateANSI(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if VisibleLength(text) <= maxWidth {
		return text
	}

	truncWidth := maxWidth - 3
	if truncWidth < 0 {
		truncWidth = 0
	}

	var result strings.Builder
	visCount := 0
	runes := []rune(text)
	i := 0

	for i < len(runes) && visCount < truncWidth {
		if runes[i] == '\033' {
			for i < len(runes) {
				result.WriteRune(runes[i])
				if runes[i] == 'm' {
					i++
					break
				}
				i++
			}
			continue
		}
		result.WriteRune(runes[i])
		visCount++
		i++
	}

	result.WriteString(Reset)
	if truncWidth < maxWidth {
		result.WriteString("...")
	}
	return result.String()
}

// ColorizedSeparator returns a separator line with the specified color
func ColorizedSeparator(length int, color string) string {
	return ColorizedString(strings.Repeat("=", length), color)
}
