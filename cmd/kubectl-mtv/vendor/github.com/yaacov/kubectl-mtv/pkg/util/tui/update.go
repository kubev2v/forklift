package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeSearch:
			return m.handleSearchKey(msg)
		case modeSearchResults:
			return m.handleSearchResultsKey(msg)
		case modeQuery:
			return m.handleQueryKey(msg)
		default:
			return m.handleKeyPress(msg)
		}

	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)

	case tickMsg:
		// Pause refresh when in an interactive mode
		if m.mode != modeNormal {
			return m, tickCmd(m.refreshInterval)
		}
		m.loading = true
		return m, tea.Batch(
			fetchData(m.dataFetcher),
			tickCmd(m.refreshInterval),
		)

	case fetchDataMsg:
		m.loading = false
		m.lastUpdate = time.Now()

		if msg.err != nil {
			m.lastError = msg.err
		} else {
			m.lastError = nil
			m.content = msg.content

			if m.searchTerm != "" {
				m.reapplySearch()
			} else {
				m.viewport.SetContent(m.content)
			}
		}

		if !m.ready {
			m.ready = true
		}

		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input in normal mode
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Help) {
		m.showHelp = !m.showHelp
		if m.showHelp {
			m.help.ShowAll = true
		}
		return m, nil
	}

	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	if key.Matches(msg, m.keys.Quit) {
		m.quitting = true
		return m, tea.Quit
	}

	if key.Matches(msg, m.keys.Search) {
		m.mode = modeSearch
		m.searchInput.SetValue("")
		m.searchInput.Focus()
		return m, m.searchInput.Cursor.BlinkCmd()
	}

	if key.Matches(msg, m.keys.Query) && m.queryUpdater != nil {
		m.mode = modeQuery
		m.queryInput.SetValue(m.currentQuery)
		m.queryInput.Focus()
		return m, m.queryInput.Cursor.BlinkCmd()
	}

	if key.Matches(msg, m.keys.Refresh) {
		m.loading = true
		return m, fetchData(m.dataFetcher)
	}

	if key.Matches(msg, m.keys.IncreaseInt) {
		m.refreshInterval += 5 * time.Second
		if m.refreshInterval > 300*time.Second {
			m.refreshInterval = 300 * time.Second
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.DecreaseInt) {
		m.refreshInterval -= 5 * time.Second
		if m.refreshInterval < 5*time.Second {
			m.refreshInterval = 5 * time.Second
		}
		return m, nil
	}

	var cmd tea.Cmd
	if key.Matches(msg, m.keys.Up) {
		m.viewport.ScrollUp(1)
	} else if key.Matches(msg, m.keys.Down) {
		m.viewport.ScrollDown(1)
	} else if key.Matches(msg, m.keys.PageUp) {
		m.viewport.HalfPageUp()
	} else if key.Matches(msg, m.keys.PageDown) {
		m.viewport.HalfPageDown()
	} else if key.Matches(msg, m.keys.Home) {
		m.viewport.GotoTop()
	} else if key.Matches(msg, m.keys.End) {
		m.viewport.GotoBottom()
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return m, cmd
}

// handleSearchKey handles input while typing a search term.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		term := m.searchInput.Value()
		if term == "" {
			m.clearSearch()
			m.mode = modeNormal
			return m, nil
		}
		m.searchTerm = term
		m.performSearch()
		if len(m.searchMatches) > 0 {
			m.searchIndex = 0
			m.mode = modeSearchResults
			m.updateFocusHighlight()
			m.scrollToMatch()
		} else {
			m.mode = modeNormal
		}
		return m, nil

	case tea.KeyEsc:
		m.clearSearch()
		m.mode = modeNormal
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)

	term := m.searchInput.Value()
	if term != "" {
		m.searchTerm = term
		m.performSearch()
	} else {
		m.clearSearch()
	}

	return m, cmd
}

// handleSearchResultsKey handles keys when navigating search results.
func (m Model) handleSearchResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.NextMatch):
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex + 1) % len(m.searchMatches)
			m.updateFocusHighlight()
			m.scrollToMatch()
		}
		return m, nil

	case key.Matches(msg, m.keys.PrevMatch):
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex - 1 + len(m.searchMatches)) % len(m.searchMatches)
			m.updateFocusHighlight()
			m.scrollToMatch()
		}
		return m, nil

	case key.Matches(msg, m.keys.Search):
		m.mode = modeSearch
		m.searchInput.SetValue("")
		m.searchInput.Focus()
		return m, m.searchInput.Cursor.BlinkCmd()

	case msg.Type == tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit

	case msg.Type == tea.KeyEsc, msg.String() == "q":
		m.clearSearch()
		m.mode = modeNormal
		return m, nil
	}

	// Allow viewport scrolling in search results mode
	var cmd tea.Cmd
	if key.Matches(msg, m.keys.Up) {
		m.viewport.ScrollUp(1)
	} else if key.Matches(msg, m.keys.Down) {
		m.viewport.ScrollDown(1)
	} else if key.Matches(msg, m.keys.PageUp) {
		m.viewport.HalfPageUp()
	} else if key.Matches(msg, m.keys.PageDown) {
		m.viewport.HalfPageDown()
	} else if key.Matches(msg, m.keys.Home) {
		m.viewport.GotoTop()
	} else if key.Matches(msg, m.keys.End) {
		m.viewport.GotoBottom()
	}

	return m, cmd
}

// handleQueryKey handles input while typing a query string.
func (m Model) handleQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		q := m.queryInput.Value()
		m.currentQuery = q
		if m.queryUpdater != nil {
			m.queryUpdater(q)
		}
		m.mode = modeNormal
		m.loading = true
		return m, fetchData(m.dataFetcher)

	case tea.KeyEsc:
		m.mode = modeNormal
		return m, nil
	}

	var cmd tea.Cmd
	m.queryInput, cmd = m.queryInput.Update(msg)
	return m, cmd
}

// performSearch finds all matches and applies uniform highlighting.
func (m *Model) performSearch() {
	m.searchMatches = findMatches(m.content, m.searchTerm)
	m.highlightedContent = highlightContent(m.content, m.searchTerm)
	m.viewport.SetContent(m.highlightedContent)
}

// reapplySearch re-runs the search on updated content (after data refresh).
func (m *Model) reapplySearch() {
	m.searchMatches = findMatches(m.content, m.searchTerm)
	m.highlightedContent = highlightContent(m.content, m.searchTerm)
	m.viewport.SetContent(m.highlightedContent)

	if m.searchIndex >= len(m.searchMatches) {
		m.searchIndex = 0
	}
}

// updateFocusHighlight re-renders content with the focused match distinctly highlighted.
func (m *Model) updateFocusHighlight() {
	m.highlightedContent = highlightContentWithFocus(m.content, m.searchTerm, m.searchMatches, m.searchIndex)
	m.viewport.SetContent(m.highlightedContent)
}

// clearSearch resets all search state and restores original content.
func (m *Model) clearSearch() {
	m.searchTerm = ""
	m.searchMatches = nil
	m.searchIndex = 0
	m.highlightedContent = ""
	m.viewport.SetContent(m.content)
}

// scrollToMatch scrolls the viewport so the current match is visible.
func (m *Model) scrollToMatch() {
	line := lineForMatch(m.searchMatches, m.searchIndex)
	m.viewport.SetYOffset(line)
}

// handleWindowResize handles terminal window resize events
func (m Model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	headerHeight := 0
	footerHeight := 2
	verticalMarginHeight := headerHeight + footerHeight

	if !m.ready {
		m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
		m.viewport.YPosition = headerHeight
		m.viewport.SetContent(m.content)
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMarginHeight
	}

	m.help.Width = msg.Width

	return m, nil
}

// Run starts the TUI program (backward-compatible).
func Run(dataFetcher DataFetcher, refreshInterval time.Duration) error {
	return RunWithOptions(dataFetcher, refreshInterval)
}

// RunWithOptions starts the TUI program with optional configuration.
func RunWithOptions(dataFetcher DataFetcher, refreshInterval time.Duration, opts ...Option) error {
	model := NewModel(dataFetcher, refreshInterval, opts...)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
