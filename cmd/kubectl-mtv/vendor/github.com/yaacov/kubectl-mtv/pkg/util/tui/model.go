package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// DataFetcher is a function that fetches and returns data as a formatted string
type DataFetcher func() (string, error)

// Model represents the TUI state
type Model struct {
	viewport        viewport.Model
	spinner         spinner.Model
	help            help.Model
	keys            keyMap
	dataFetcher     DataFetcher
	content         string
	lastUpdate      time.Time
	lastError       error
	refreshInterval time.Duration
	showHelp        bool
	loading         bool
	ready           bool
	quitting        bool
	width           int
	height          int
}

// keyMap defines the keybindings for the TUI
type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Home        key.Binding
	End         key.Binding
	Quit        key.Binding
	Refresh     key.Binding
	Help        key.Binding
	IncreaseInt key.Binding
	DecreaseInt key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Refresh, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Home, k.End},
		{k.Refresh, k.IncreaseInt, k.DecreaseInt},
		{k.Help, k.Quit},
	}
}

// defaultKeys returns default key bindings
func defaultKeys() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup/b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f", " "),
			key.WithHelp("pgdn/f/space", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to start"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to end"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh now"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		IncreaseInt: key.NewBinding(
			key.WithKeys("+", "="),
			key.WithHelp("+", "increase refresh interval"),
		),
		DecreaseInt: key.NewBinding(
			key.WithKeys("-"),
			key.WithHelp("-", "decrease refresh interval"),
		),
	}
}

// NewModel creates a new TUI model
func NewModel(dataFetcher DataFetcher, refreshInterval time.Duration) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return Model{
		spinner:         s,
		help:            help.New(),
		keys:            defaultKeys(),
		dataFetcher:     dataFetcher,
		refreshInterval: refreshInterval,
		lastUpdate:      time.Now(),
		loading:         false,
		showHelp:        false,
		ready:           false,
		quitting:        false,
	}
}

// Init initializes the TUI model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchData(m.dataFetcher),
		tickCmd(m.refreshInterval),
	)
}

// TickMsg is sent on each refresh interval
type tickMsg time.Time

// fetchDataMsg is sent when data fetching completes
type fetchDataMsg struct {
	content string
	err     error
}

// tickCmd returns a command that sends a tick message after the given duration
func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// fetchData returns a command that fetches data
func fetchData(fetcher DataFetcher) tea.Cmd {
	return func() tea.Msg {
		content, err := fetcher()
		return fetchDataMsg{content: content, err: err}
	}
}
