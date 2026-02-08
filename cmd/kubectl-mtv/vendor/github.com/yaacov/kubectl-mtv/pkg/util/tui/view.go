package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles for the TUI
var (
	// Status bar styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("63")).
			Padding(0, 1)

	statusBarErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("196")).
				Padding(0, 1)

	// Help styles
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			MarginBottom(1)

	// Spinner style
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
)

// View renders the TUI
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return fmt.Sprintf("\n  %s Loading...\n\n", m.spinner.View())
	}

	var b strings.Builder

	// Render the main content viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Render help overlay if visible
	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(m.renderHelpOverlay())
		b.WriteString("\n")
	}

	// Render the status bar
	b.WriteString(m.renderStatusBar())

	return b.String()
}

// renderStatusBar renders the status bar at the bottom
func (m Model) renderStatusBar() string {
	var parts []string

	// Loading indicator
	if m.loading {
		parts = append(parts, m.spinner.View()+" Refreshing...")
	}

	// Last update time
	elapsed := time.Since(m.lastUpdate)
	if elapsed < time.Minute {
		parts = append(parts, fmt.Sprintf("Updated %ds ago", int(elapsed.Seconds())))
	} else {
		parts = append(parts, fmt.Sprintf("Updated %s ago", elapsed.Round(time.Second)))
	}

	// Refresh interval
	parts = append(parts, fmt.Sprintf("Refresh: %ds", int(m.refreshInterval.Seconds())))

	// Scroll position hint
	scrollPercent := m.viewport.ScrollPercent()
	if scrollPercent > 0 || scrollPercent < 1 {
		parts = append(parts, fmt.Sprintf("Scroll: %d%%", int(scrollPercent*100)))
	}

	// Quick help hint
	parts = append(parts, "Press ? for help")

	statusText := strings.Join(parts, " • ")

	// Use error style if there's an error, otherwise normal style
	style := statusBarStyle
	if m.lastError != nil {
		style = statusBarErrorStyle
		errorMsg := fmt.Sprintf("Error: %v", m.lastError)
		// Truncate error message if too long
		if len(errorMsg) > m.width-10 {
			errorMsg = errorMsg[:m.width-13] + "..."
		}
		statusText = errorMsg + " • " + statusText
	}

	// Ensure the status bar fits the width
	if len(statusText) > m.width-4 {
		statusText = statusText[:m.width-7] + "..."
	}

	return style.Width(m.width).Render(statusText)
}

// renderHelpOverlay renders the help panel overlay
func (m Model) renderHelpOverlay() string {
	helpContent := helpTitleStyle.Render("Keyboard Shortcuts") + "\n\n"
	helpContent += m.help.View(m.keys)

	// Add additional information
	helpContent += "\n\n"
	helpContent += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		"TIP: Use +/- to adjust refresh interval\n" +
			"Press ? again to close this help",
	)

	// Center the help panel
	helpBox := helpStyle.Render(helpContent)

	// Calculate centering
	helpWidth := lipgloss.Width(helpBox)
	helpHeight := lipgloss.Height(helpBox)

	horizontalMargin := (m.width - helpWidth) / 2
	verticalMargin := (m.height - helpHeight - 3) / 2 // -3 for status bar and spacing

	if horizontalMargin < 0 {
		horizontalMargin = 0
	}
	if verticalMargin < 0 {
		verticalMargin = 0
	}

	// Add margin to center the help box
	centeredHelp := lipgloss.NewStyle().
		MarginLeft(horizontalMargin).
		MarginTop(verticalMargin).
		Render(helpBox)

	return centeredHelp
}
