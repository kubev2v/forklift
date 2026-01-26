package main

import (
	"context"
	"fmt"
	"os/exec"
)

type PSExecutor interface {
	Execute(ctx context.Context, command string) (string, error)
}

type RealPSExecutor struct{}

func (r *RealPSExecutor) Execute(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", &PSError{
				Message:   "command cancelled",
				Cancelled: true,
			}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &PSError{
				Message: err.Error(),
				Stderr:  string(exitErr.Stderr),
			}
		}
		return "", err
	}
	return string(out), nil
}

type PSError struct {
	Message   string
	Stderr    string
	Cancelled bool
}

func (e *PSError) Error() string {
	if e.Cancelled {
		return "command cancelled by user"
	}
	if e.Stderr != "" {
		return fmt.Sprintf("powershell error: %s\nstderr: %s", e.Message, e.Stderr)
	}
	return "powershell error: " + e.Message
}

func (e *PSError) IsCancelled() bool {
	return e.Cancelled
}
