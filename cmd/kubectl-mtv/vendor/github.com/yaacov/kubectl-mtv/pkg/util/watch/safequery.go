package watch

import "sync"

// SafeQuery provides goroutine-safe read/write access to a query string.
// It is used to eliminate data races between the TUI's queryUpdater callback
// (which writes from the bubbletea Update loop) and the list-function closure
// (which reads from a bubbletea Cmd goroutine).
type SafeQuery struct {
	mu    sync.RWMutex
	value string
}

// NewSafeQuery returns a SafeQuery initialised with the given value.
func NewSafeQuery(initial string) *SafeQuery {
	return &SafeQuery{value: initial}
}

// Get returns the current query string (read-lock).
func (sq *SafeQuery) Get() string {
	sq.mu.RLock()
	defer sq.mu.RUnlock()
	return sq.value
}

// Set replaces the current query string (write-lock).
// Its signature matches tui.QueryUpdater so it can be passed directly.
func (sq *SafeQuery) Set(q string) {
	sq.mu.Lock()
	defer sq.mu.Unlock()
	sq.value = q
}
