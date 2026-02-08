package main

import (
	"context"
	"fmt"
	"strings"
)

type MockPSExecutor struct {
	Commands         map[string]MockResponse
	ExecutedCommands []string
}

type MockResponse struct {
	Output string
	Error  error
}

func NewMockPSExecutor() *MockPSExecutor {
	return &MockPSExecutor{
		Commands:         make(map[string]MockResponse),
		ExecutedCommands: []string{},
	}
}

func (m *MockPSExecutor) AddResponse(pattern string, output string, err error) {
	m.Commands[pattern] = MockResponse{Output: output, Error: err}
}

func (m *MockPSExecutor) Execute(ctx context.Context, command string) (string, error) {
	select {
	case <-ctx.Done():
		return "", &PSError{
			Message:   "command cancelled",
			Cancelled: true,
		}
	default:
	}

	m.ExecutedCommands = append(m.ExecutedCommands, command)

	if resp, ok := m.Commands[command]; ok {
		return resp.Output, resp.Error
	}

	for pattern, resp := range m.Commands {
		if strings.Contains(command, pattern) {
			return resp.Output, resp.Error
		}
	}

	return "", fmt.Errorf("no mock response for command: %s", command)
}

func (m *MockPSExecutor) GetExecutedCommands() []string {
	return m.ExecutedCommands
}

func (m *MockPSExecutor) Reset() {
	m.ExecutedCommands = []string{}
}

func (m *MockPSExecutor) SetupValidationMocks() {
	// HyperV availability check
	m.AddResponse("Get-Module -ListAvailable -Name Hyper-V", "AVAILABLE", nil)
	// HyperV permissions check
	m.AddResponse("Get-VM -ErrorAction Stop", "OK", nil)
}
