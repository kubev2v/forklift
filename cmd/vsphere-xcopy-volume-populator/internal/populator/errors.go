package populator

import "fmt"

// MapUnmapError represents a non-fatal error that occurs during map/unmap operations
// These errors should not cause the populate container to restart
type MapUnmapError struct {
	Operation string // "map" or "unmap"
	Message   string
	Err       error
}

func (e *MapUnmapError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s operation failed: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("%s operation failed: %s", e.Operation, e.Message)
}

func (e *MapUnmapError) Unwrap() error {
	return e.Err
}

// IsMapUnmapError checks if an error is a MapUnmapError
func IsMapUnmapError(err error) bool {
	_, ok := err.(*MapUnmapError)
	return ok
}

// NewMapError creates a new MapUnmapError for map operations
func NewMapError(message string, err error) *MapUnmapError {
	return &MapUnmapError{
		Operation: "map",
		Message:   message,
		Err:       err,
	}
}

// NewUnmapError creates a new MapUnmapError for unmap operations
func NewUnmapError(message string, err error) *MapUnmapError {
	return &MapUnmapError{
		Operation: "unmap",
		Message:   message,
		Err:       err,
	}
}
