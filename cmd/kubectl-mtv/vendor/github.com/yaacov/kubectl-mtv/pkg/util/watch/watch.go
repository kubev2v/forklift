package watch

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yaacov/kubectl-mtv/pkg/util/tui"
)

// DefaultInterval is the default watch interval for all watch operations
const DefaultInterval = 5 * time.Second

// RenderFunc is a function that renders output and returns an error if any
type RenderFunc func() error

// stdoutMu serializes access to os.Stdout inside captureOutput so concurrent
// data fetches don't race on the global file descriptor.
var stdoutMu sync.Mutex

// captureOutput wraps a RenderFunc into a DataFetcher by capturing its stdout.
func captureOutput(renderFunc RenderFunc) tui.DataFetcher {
	return func() (output string, retErr error) {
		stdoutMu.Lock()
		defer stdoutMu.Unlock()

		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			return "", fmt.Errorf("failed to create pipe: %w", err)
		}

		os.Stdout = w

		// Buffered so the reader goroutine never blocks if we return early.
		outputChan := make(chan string, 1)
		go func() {
			var buf strings.Builder
			_, _ = io.Copy(&buf, r)
			outputChan <- buf.String()
		}()

		defer func() {
			w.Close()
			os.Stdout = oldStdout
			output = <-outputChan

			if p := recover(); p != nil {
				retErr = fmt.Errorf("renderFunc panicked: %v", p)
			}
		}()

		retErr = renderFunc()
		return
	}
}

// Watch uses TUI mode for watching with smooth updates and interactive features.
func Watch(renderFunc RenderFunc, interval time.Duration) error {
	return tui.Run(captureOutput(renderFunc), interval)
}

// WatchWithQuery uses TUI mode with interactive query editing support.
func WatchWithQuery(renderFunc RenderFunc, interval time.Duration, queryUpdater tui.QueryUpdater, currentQuery string) error {
	return tui.RunWithOptions(
		captureOutput(renderFunc),
		interval,
		tui.WithQueryUpdater(queryUpdater),
		tui.WithInitialQuery(currentQuery),
	)
}
