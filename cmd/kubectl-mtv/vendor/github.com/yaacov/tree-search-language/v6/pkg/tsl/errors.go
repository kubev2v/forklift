package tsl

import "fmt"

// SyntaxError represents a TSL parsing error with context
type SyntaxError struct {
	Message  string
	Position int
	Context  string
	Input    string
}

// Error implements the error interface
func (e *SyntaxError) Error() string {
	pointer := make([]rune, e.Position)
	for i := range pointer {
		pointer[i] = ' '
	}
	pointerLine := string(pointer) + "^"

	return fmt.Sprintf("%s at position %d:\n%s\n%s",
		e.Message, e.Position, e.Input, pointerLine)
}

// UnexpectedLiteralError is returned when encountering an unexpected literal or operator
type UnexpectedLiteralError struct {
	Literal interface{}
}

func (e UnexpectedLiteralError) Error() string {
	return fmt.Sprintf("unexpected literal: %v", e.Literal)
}

// DivisionByZeroError is returned when attempting to divide by zero
type DivisionByZeroError struct {
	Operation string
}

func (e DivisionByZeroError) Error() string {
	return fmt.Sprintf("%s by zero", e.Operation)
}

// TypeMismatchError is returned when operand types don't match the operator
type TypeMismatchError struct {
	Expected string
	Got      interface{}
}

func (e TypeMismatchError) Error() string {
	return fmt.Sprintf("type mismatch: expected %s, got %v", e.Expected, e.Got)
}

// UnexpectedOperatorError is returned when encountering an unexpected operator
type UnexpectedOperatorError struct {
	Operator interface{}
}

func (e UnexpectedOperatorError) Error() string {
	return fmt.Sprintf("unexpected operator: %v", e.Operator)
}

// UnexpectedTypeError is returned when encountering an unexpected type in the AST
type UnexpectedTypeError struct {
	Type interface{}
}

func (e UnexpectedTypeError) Error() string {
	return fmt.Sprintf("unexpected type: %v", e.Type)
}

// BetweenOperatorError is returned when BETWEEN operator is used incorrectly
type BetweenOperatorError struct {
	Message string
}

func (e BetweenOperatorError) Error() string {
	return fmt.Sprintf("between operator error: %s", e.Message)
}

// KeyNotFoundError is returned when a requested key is not found in the data
type KeyNotFoundError struct {
	Key string
}

func (e KeyNotFoundError) Error() string {
	return fmt.Sprintf("key not found: %s", e.Key)
}
