package mtvmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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

// ValidationError represents a structured validation error
type ValidationError struct {
	Error   string   `json:"error"`
	Type    string   `json:"type"`
	Message string   `json:"message"`
	Missing []string `json:"missing_params,omitempty"`
}

// ValidateRequiredParams validates that required parameters are not empty
func ValidateRequiredParams(params map[string]string) error {
	var missing []string
	for name, value := range params {
		if value == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		validationErr := ValidationError{
			Error:   "validation_error",
			Type:    "missing_required_parameters",
			Message: fmt.Sprintf("Missing required parameter(s): %s", strings.Join(missing, ", ")),
			Missing: missing,
		}
		jsonData, _ := json.MarshalIndent(validationErr, "", "  ")
		return fmt.Errorf("%s", string(jsonData))
	}
	return nil
}

// NetworkPairValidationError represents a network pair validation error
type NetworkPairValidationError struct {
	Error       string   `json:"error"`
	Type        string   `json:"type"`
	Message     string   `json:"message"`
	Target      string   `json:"target"`
	Sources     []string `json:"sources"`
	Explanation string   `json:"explanation"`
}

// ValidateNetworkPairs validates that network pairs follow the constraint rules:
// - Pod networking ('default') can only be mapped ONCE across all sources
// - Each specific NAD can only be mapped ONCE across all sources
// - 'ignored' can be used multiple times
func ValidateNetworkPairs(pairsStr string) error {
	if pairsStr == "" {
		return nil
	}

	// Parse the pairs
	pairs := strings.Split(pairsStr, ",")
	targetMap := make(map[string][]string) // target -> list of sources using it

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue // Skip malformed pairs, let kubectl-mtv handle the error
		}

		source := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])

		// Skip 'ignored' targets - they can be used multiple times
		if target == "ignored" {
			continue
		}

		// Track which sources map to this target
		targetMap[target] = append(targetMap[target], source)
	}

	// Check for duplicate target usage
	for target, sources := range targetMap {
		if len(sources) > 1 {
			var explanation string
			if target == "default" {
				explanation = "Pod networking can only be used once. Consider using 'ignored' for sources that don't need network access, or map to different network attachment definitions."
			} else {
				explanation = "Each network attachment definition can only be mapped once. Consider using different target networks or 'ignored' for sources that don't need this network."
			}

			validationErr := NetworkPairValidationError{
				Error:       "validation_error",
				Type:        "duplicate_network_target",
				Message:     fmt.Sprintf("Target network '%s' is mapped multiple times", target),
				Target:      target,
				Sources:     sources,
				Explanation: explanation,
			}
			jsonData, _ := json.MarshalIndent(validationErr, "", "  ")
			return fmt.Errorf("%s", string(jsonData))
		}
	}

	return nil
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
		response := CommandResponse{
			Command:     formatShellCommand("kubectl-mtv", args),
			ReturnValue: 0,
			Stdout:      formatShellCommand("kubectl-mtv", args),
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
		response := CommandResponse{
			Command:     formatShellCommand("kubectl", args),
			ReturnValue: 0,
			Stdout:      formatShellCommand("kubectl", args),
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

// ExtractStdoutFromResponse extracts stdout from a structured JSON response
func ExtractStdoutFromResponse(responseJSON string) string {
	var response CommandResponse
	if err := json.Unmarshal([]byte(responseJSON), &response); err != nil {
		return responseJSON // Fallback to original response
	}
	return response.Stdout
}

// AddBooleanFlag adds a boolean flag to args if value is not nil
func AddBooleanFlag(args *[]string, flagName string, value *bool) {
	if value != nil {
		if *value {
			*args = append(*args, "--"+flagName)
		} else {
			*args = append(*args, "--"+flagName+"=false")
		}
	}
}

// BoolPtr returns a pointer to a bool value
func BoolPtr(b bool) *bool {
	return &b
}

// BuildBaseArgs builds base arguments for kubectl-mtv commands
func BuildBaseArgs(namespace string, allNamespaces bool) []string {
	args := []string{}

	if allNamespaces {
		args = append(args, "-A")
	} else if namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Always use JSON output format for MCP
	args = append(args, "-o", "json")

	return args
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
