package watch

import (
	"fmt"
	"time"
)

// WrapWithWatch wraps a list function with optional watch mode
// If watchMode is true, it validates output format and enables watching
// Otherwise, it just calls the list function once
func WrapWithWatch(watchMode bool, outputFormat string, listFunc RenderFunc, interval time.Duration) error {
	if watchMode {
		if outputFormat != "table" {
			return fmt.Errorf("watch mode only supports table output format")
		}
		return Watch(listFunc, interval)
	}
	return listFunc()
}
