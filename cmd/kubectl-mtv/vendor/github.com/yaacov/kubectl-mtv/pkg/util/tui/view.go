package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Styles for the TUI
var (
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("63")).
			Padding(0, 1)

	statusBarErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("196")).
				Padding(0, 1)

	inputBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("27")).
			Padding(0, 1)

	searchResultsBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("214")).
				Padding(0, 1)

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

	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(m.renderHelpOverlay())
		b.WriteString("\n")
	}

	switch m.mode {
	case modeSearch:
		b.WriteString(m.renderSearchBar())
	case modeSearchResults:
		b.WriteString(m.renderSearchResultsBar())
	case modeQuery:
		b.WriteString(m.renderQueryBar())
	default:
		b.WriteString(m.renderStatusBar())
	}

	return b.String()
}

// renderSearchBar renders the search input bar.
func (m Model) renderSearchBar() string {
	matchCount := len(m.searchMatches)
	suffix := ""
	if m.searchInput.Value() != "" {
		suffix = fmt.Sprintf("  [%d matches]", matchCount)
	}

	text := "/" + m.searchInput.View() + suffix
	return inputBarStyle.Width(m.width).Render(text)
}

// renderSearchResultsBar renders the bar shown while navigating search results.
func (m Model) renderSearchResultsBar() string {
	total := len(m.searchMatches)
	current := m.searchIndex + 1

	text := fmt.Sprintf(" /%s  [%d/%d]  n/N: next/prev  Esc: clear  /: new search",
		m.searchTerm, current, total)

	return searchResultsBarStyle.Width(m.width).Render(text)
}

// renderQueryBar renders the query input bar.
func (m Model) renderQueryBar() string {
	text := ":" + m.queryInput.View()
	return inputBarStyle.Width(m.width).Render(text)
}

// renderStatusBar renders the status bar at the bottom
func (m Model) renderStatusBar() string {
	var parts []string

	if m.loading {
		parts = append(parts, m.spinner.View()+" Refreshing...")
	}

	elapsed := time.Since(m.lastUpdate)
	if elapsed < time.Minute {
		parts = append(parts, fmt.Sprintf("Updated %ds ago", int(elapsed.Seconds())))
	} else {
		parts = append(parts, fmt.Sprintf("Updated %s ago", elapsed.Round(time.Second)))
	}

	parts = append(parts, fmt.Sprintf("Refresh: %ds", int(m.refreshInterval.Seconds())))

	if m.queryUpdater != nil && m.currentQuery != "" {
		q := m.currentQuery
		if len(q) > 30 {
			q = q[:27] + "..."
		}
		parts = append(parts, fmt.Sprintf("Query: %s", q))
	}

	if m.searchTerm != "" {
		parts = append(parts, fmt.Sprintf("/%s [%d]", m.searchTerm, len(m.searchMatches)))
	}

	scrollPercent := m.viewport.ScrollPercent()
	if scrollPercent > 0 || scrollPercent < 1 {
		parts = append(parts, fmt.Sprintf("Scroll: %d%%", int(scrollPercent*100)))
	}

	parts = append(parts, "Press ? for help")

	statusText := strings.Join(parts, " • ")

	style := statusBarStyle
	if m.lastError != nil {
		style = statusBarErrorStyle
		errorMsg := fmt.Sprintf("Error: %v", m.lastError)
		if len(errorMsg) > m.width-10 {
			errorMsg = errorMsg[:m.width-13] + "..."
		}
		statusText = errorMsg + " • " + statusText
	}

	if len(statusText) > m.width-4 {
		statusText = statusText[:m.width-7] + "..."
	}

	return style.Width(m.width).Render(statusText)
}

// renderHelpOverlay renders the help panel overlay
func (m Model) renderHelpOverlay() string {
	helpContent := helpTitleStyle.Render("Keyboard Shortcuts") + "\n\n"
	helpContent += m.help.View(m.keys)

	helpContent += "\n\n"
	tips := "TIP: Use +/- to adjust refresh interval\n" +
		"     / to search, n/N to navigate matches"
	if m.queryUpdater != nil {
		tips += "\n     : to edit the filter query"
	}
	tips += "\n     Press ? again to close this help"

	helpContent += lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(tips)

	helpBox := helpStyle.Render(helpContent)

	helpWidth := lipgloss.Width(helpBox)
	helpHeight := lipgloss.Height(helpBox)

	horizontalMargin := (m.width - helpWidth) / 2
	verticalMargin := (m.height - helpHeight - 3) / 2

	if horizontalMargin < 0 {
		horizontalMargin = 0
	}
	if verticalMargin < 0 {
		verticalMargin = 0
	}

	centeredHelp := lipgloss.NewStyle().
		MarginLeft(horizontalMargin).
		MarginTop(verticalMargin).
		Render(helpBox)

	return centeredHelp
}
