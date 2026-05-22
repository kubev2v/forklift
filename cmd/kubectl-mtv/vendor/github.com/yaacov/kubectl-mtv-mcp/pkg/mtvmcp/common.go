package mtvmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	shellquote "github.com/kballard/go-shellquote"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// kubeconfigTokenKey is the context key for Kubernetes token
	kubeconfigTokenKey contextKey = "kubeconfig_token"
	// dryRunKey is the context key for dry run mode
	dryRunKey contextKey = "dry_run"
)

// WithKubeToken adds a Kubernetes token to the context
func WithKubeToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, kubeconfigTokenKey, token)
}

// GetKubeToken retrieves the Kubernetes token from the context
func GetKubeToken(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	token, ok := ctx.Value(kubeconfigTokenKey).(string)
	return token, ok
}

// WithDryRun adds a dry run flag to the context
func WithDryRun(ctx context.Context, dryRun bool) context.Context {
	return context.WithValue(ctx, dryRunKey, dryRun)
}

// GetDryRun retrieves the dry run flag from the context
func GetDryRun(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	dryRun, ok := ctx.Value(dryRunKey).(bool)
	return ok && dryRun
}

// CommandResponse represents the structured response from command execution
type CommandResponse struct {
	Command     string `json:"command"`
	ReturnValue int    `json:"return_value"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
}

// RunKubectlMTVCommand executes a kubectl-mtv command and returns structured JSON
// It accepts a context which may contain a Kubernetes token for authentication.
// If a token is present in the context, it will be passed via the --token flag.
// If no token is present, it falls back to the default kubeconfig behavior.
// If dry run mode is enabled in the context, it returns a teaching response instead of executing.
func RunKubectlMTVCommand(ctx context.Context, args []string) (string, error) {
	// Check if we have a token in the context and prepend --token flag
	if token, ok := GetKubeToken(ctx); ok && token != "" {
		// Insert --token flag at the beginning of args (after subcommand if present)
		// This ensures it's processed before any other flags
		args = append([]string{"--token", token}, args...)
	}

	// Check if we're in dry run mode
	if GetDryRun(ctx) {
		// In dry run mode, just return the command that would be executed
		// The AI will explain it in context
		cmdStr := formatShellCommand("kubectl-mtv", args)
		response := CommandResponse{
			Command:     cmdStr,
			ReturnValue: 0,
			Stdout:      cmdStr,
			Stderr:      "",
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}

		return string(jsonData), nil
	}

	cmd := exec.Command("kubectl-mtv", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout of 120 seconds
	timer := time.AfterFunc(120*time.Second, func() {
		_ = cmd.Process.Kill()
	})
	defer timer.Stop()

	err := cmd.Run()

	response := CommandResponse{
		Command: formatShellCommand("kubectl-mtv", args),
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			response.ReturnValue = exitErr.ExitCode()
		} else {
			response.ReturnValue = -1
			if response.Stderr == "" {
				response.Stderr = err.Error()
			}
		}
	} else {
		response.ReturnValue = 0
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonData), nil
}

// RunKubectlCommand executes a kubectl command and returns structured JSON
// It accepts a context which may contain a Kubernetes token for authentication.
// If a token is present in the context, it will be passed via the --token flag.
// If no token is present, it falls back to the default kubeconfig behavior.
// If dry run mode is enabled in the context, it returns a teaching response instead of executing.
func RunKubectlCommand(ctx context.Context, args []string) (string, error) {
	// Check if we have a token in the context and prepend --token flag
	if token, ok := GetKubeToken(ctx); ok && token != "" {
		// Insert --token flag at the beginning of args (after subcommand if present)
		// This ensures it's processed before any other flags
		args = append([]string{"--token", token}, args...)
	}

	// Check if we're in dry run mode
	if GetDryRun(ctx) {
		// In dry run mode, just return the command that would be executed
		// The AI will explain it in context
		cmdStr := formatShellCommand("kubectl", args)
		response := CommandResponse{
			Command:     cmdStr,
			ReturnValue: 0,
			Stdout:      cmdStr,
			Stderr:      "",
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal response: %w", err)
		}

		return string(jsonData), nil
	}

	cmd := exec.Command("kubectl", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout of 120 seconds
	timer := time.AfterFunc(120*time.Second, func() {
		_ = cmd.Process.Kill()
	})
	defer timer.Stop()

	err := cmd.Run()

	response := CommandResponse{
		Command: formatShellCommand("kubectl", args),
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			response.ReturnValue = exitErr.ExitCode()
		} else {
			response.ReturnValue = -1
			if response.Stderr == "" {
				response.Stderr = err.Error()
			}
		}
	} else {
		response.ReturnValue = 0
	}

	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(jsonData), nil
}

// formatShellCommand formats a command and args into a display string
// It sanitizes sensitive parameters like passwords and tokens
// Note: This is for display/logging only. Actual command execution uses exec.Command()
// which handles arguments directly without shell interpretation.
func formatShellCommand(cmd string, args []string) string {
	// Sensitive flags that should have their values redacted
	sensitiveFlags := map[string]bool{
		"--password": true,
		"--token":    true,
	}

	// Build the display command with sanitization
	sanitizedArgs := []string{}
	sanitizeNext := false

	for _, arg := range args {
		if sanitizeNext {
			// Replace sensitive value with ****
			sanitizedArgs = append(sanitizedArgs, "****")
			sanitizeNext = false
		} else if sensitiveFlags[arg] {
			// This is a sensitive flag, add it and mark next arg for sanitization
			sanitizedArgs = append(sanitizedArgs, arg)
			sanitizeNext = true
		} else {
			// Normal argument
			sanitizedArgs = append(sanitizedArgs, arg)
		}
	}

	// Use shellquote.Join to properly quote all arguments
	quotedArgs := shellquote.Join(sanitizedArgs...)
	return cmd + " " + quotedArgs
}

// UnmarshalJSONResponse unmarshals a JSON string response into a native object
// This is needed because the MCP SDK expects a native object, not a JSON string
func UnmarshalJSONResponse(responseJSON string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(responseJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}
	return result, nil
}
