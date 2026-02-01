package watch

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/yaacov/kubectl-mtv/pkg/util/tui"
)

// DefaultInterval is the default watch interval for all watch operations
const DefaultInterval = 5 * time.Second

// RenderFunc is a function that renders output and returns an error if any
type RenderFunc func() error

// Watch uses TUI mode for watching with smooth updates and interactive features
// It exits when user presses q or Ctrl+C
func Watch(renderFunc RenderFunc, interval time.Duration) error {
	// Create a data fetcher that captures output from the renderFunc
	dataFetcher := func() (string, error) {
		// Create a pipe to capture stdout
		oldStdout := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			return "", fmt.Errorf("failed to create pipe: %w", err)
		}
		os.Stdout = w

		// Create a channel to collect output
		outputChan := make(chan string)
		go func() {
			var buf strings.Builder
			_, _ = io.Copy(&buf, r) // Explicitly ignore copy errors as we're just capturing output
			outputChan <- buf.String()
		}()

		// Call renderFunc which will print to our captured stdout
		renderErr := renderFunc()

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout
		output := <-outputChan

		return output, renderErr
	}

	return tui.Run(dataFetcher, interval)
}
