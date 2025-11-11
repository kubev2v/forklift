package watch

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const exitMessage = "\n\033[1;34mPress Ctrl+C to exit watch mode...\033[0m"

// RenderFunc is a function that renders output and returns an error if any
type RenderFunc func() error

// Watch periodically calls a render function and refreshes the screen
// It exits when Ctrl+C is received
func Watch(renderFunc RenderFunc, interval time.Duration) error {
	// Setup signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a ticker for periodic rendering
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Render immediately on start
	clearScreen()
	if err := renderFunc(); err != nil {
		return err
	}
	fmt.Println(exitMessage)

	// Main watch loop
	for {
		select {
		case <-ticker.C:
			clearScreen()
			if err := renderFunc(); err != nil {
				return err
			}
			fmt.Println(exitMessage)
		case <-sigChan:
			return nil
		}
	}
}

// clearScreen clears the terminal screen
func clearScreen() {
	fmt.Print("\033[2J\033[H") // ANSI escape code to clear screen and move cursor to top-left
}
