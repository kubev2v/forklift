package parser

import "fmt"

// ParseError represents a general parsing error
type ParseError struct {
	Message  string
	Position int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}
