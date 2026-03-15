package output

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	boldStyle   = lipgloss.NewStyle().Bold(true)
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	blueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// colorEnabled controls whether ANSI color codes are emitted.
// Defaults to true; set to false via SetColorEnabled for terminals that
// don't support colors or when the --no-color flag / NO_COLOR env var is set.
var colorEnabled = true

// SetColorEnabled globally enables or disables ANSI color output.
func SetColorEnabled(enabled bool) { colorEnabled = enabled }

// IsColorEnabled reports whether ANSI color output is currently enabled.
func IsColorEnabled() bool { return colorEnabled }

func Bold(text string) string {
	if !colorEnabled {
		return text
	}
	return boldStyle.Render(text)
}

func Red(text string) string {
	if !colorEnabled {
		return text
	}
	return redStyle.Render(text)
}

func Green(text string) string {
	if !colorEnabled {
		return text
	}
	return greenStyle.Render(text)
}

func Yellow(text string) string {
	if !colorEnabled {
		return text
	}
	return yellowStyle.Render(text)
}

func Blue(text string) string {
	if !colorEnabled {
		return text
	}
	return blueStyle.Render(text)
}

func Cyan(text string) string {
	if !colorEnabled {
		return text
	}
	return cyanStyle.Render(text)
}

// StripANSI removes ANSI escape sequences from a string.
func StripANSI(text string) string {
	return ansi.Strip(text)
}

// VisibleLength returns the display width of a string, excluding ANSI escape sequences.
func VisibleLength(text string) int {
	return ansi.StringWidth(text)
}

// TruncateANSI truncates text to maxWidth visible characters while preserving
// ANSI color codes. Appends "..." when truncation occurs.
func TruncateANSI(text string, maxWidth int) string {
	return ansi.Truncate(text, maxWidth, "...")
}

// Separator returns a repeated "=" line, optionally colored.
func Separator(length int, colorFn func(string) string) string {
	s := strings.Repeat("=", length)
	if colorFn != nil {
		return colorFn(s)
	}
	return s
}

// ---------------------------------------------------------------------------
// Semantic colorizers
// ---------------------------------------------------------------------------

// ColorizeStatus returns a colored string based on status value.
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

// ColorizeNumber returns a blue-colored number for migration progress.
func ColorizeNumber(number interface{}) string {
	return Blue(fmt.Sprintf("%v", number))
}

// ColorizeBoolean returns a colored string based on boolean value.
func ColorizeBoolean(b bool) string {
	if b {
		return Green(fmt.Sprintf("%t", b))
	}
	return fmt.Sprintf("%t", b)
}

// ColorizeConditionStatus returns a colored string for Kubernetes condition status values.
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

// ColorizeBooleanString returns a colored string for string representations of booleans.
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

// ColorizeConcerns colors a "C/W/I" concerns summary string:
// red when criticals > 0, yellow when warnings > 0, green when all zeros.
func ColorizeConcerns(val string) string {
	parts := strings.SplitN(strings.TrimSpace(val), "/", 3)
	if len(parts) < 2 {
		return val
	}
	critical, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	warning, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	if critical > 0 {
		return Red(val)
	}
	if warning > 0 {
		return Yellow(val)
	}
	return Green(val)
}

// ColorizeProgress returns a colored string based on percentage thresholds.
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
