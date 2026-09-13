package watch

import (
	"fmt"
	"time"

	"github.com/yaacov/kubectl-mtv/pkg/util/tui"
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

// WrapWithWatchAndQuery wraps a list function with optional watch mode and
// interactive query editing. Commands that support --query should use this
// instead of WrapWithWatch so the user can press : to edit the query at runtime.
func WrapWithWatchAndQuery(watchMode bool, outputFormat string, listFunc RenderFunc, interval time.Duration, queryUpdater tui.QueryUpdater, currentQuery string) error {
	if watchMode {
		if outputFormat != "table" {
			return fmt.Errorf("watch mode only supports table output format")
		}
		return WatchWithQuery(listFunc, interval, queryUpdater, currentQuery)
	}
	return listFunc()
}
