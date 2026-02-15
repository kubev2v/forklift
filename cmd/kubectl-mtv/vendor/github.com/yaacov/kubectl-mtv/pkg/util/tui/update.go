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
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)

	case tickMsg:
		// Time to refresh data
		m.loading = true
		return m, tea.Batch(
			fetchData(m.dataFetcher),
			tickCmd(m.refreshInterval),
		)

	case fetchDataMsg:
		// Data fetch completed
		m.loading = false
		m.lastUpdate = time.Now()

		if msg.err != nil {
			m.lastError = msg.err
			// Keep old content on error
		} else {
			m.lastError = nil
			m.content = msg.content
			m.viewport.SetContent(m.content)
		}

		if !m.ready {
			m.ready = true
		}

		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Handle viewport updates
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle help toggle
	if key.Matches(msg, m.keys.Help) {
		m.showHelp = !m.showHelp
		if m.showHelp {
			m.help.ShowAll = true
		}
		return m, nil
	}

	// If help is showing, hide it on any other key
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}

	// Handle quit
	if key.Matches(msg, m.keys.Quit) {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle refresh
	if key.Matches(msg, m.keys.Refresh) {
		m.loading = true
		return m, fetchData(m.dataFetcher)
	}

	// Handle interval adjustments
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

	// Handle viewport navigation
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
		// Let viewport handle other keys
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return m, cmd
}

// handleWindowResize handles terminal window resize events
func (m Model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	headerHeight := 0
	footerHeight := 2 // Status bar + spacing
	verticalMarginHeight := headerHeight + footerHeight

	if !m.ready {
		// We're still initializing, create viewport
		m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
		m.viewport.YPosition = headerHeight
		m.viewport.SetContent(m.content)
	} else {
		// Update existing viewport
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMarginHeight
	}

	// Update help width
	m.help.Width = msg.Width

	return m, nil
}

// Run starts the TUI program
func Run(dataFetcher DataFetcher, refreshInterval time.Duration) error {
	model := NewModel(dataFetcher, refreshInterval)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
