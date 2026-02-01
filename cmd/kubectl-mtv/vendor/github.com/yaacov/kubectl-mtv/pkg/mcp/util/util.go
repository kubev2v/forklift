package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	// kubeconfigServerKey is the context key for Kubernetes API server URL
	kubeconfigServerKey contextKey = "kubeconfig_server"
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

// WithKubeServer adds a Kubernetes API server URL to the context
func WithKubeServer(ctx context.Context, server string) context.Context {
	return context.WithValue(ctx, kubeconfigServerKey, server)
}

// GetKubeServer retrieves the Kubernetes API server URL from the context
func GetKubeServer(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	server, ok := ctx.Value(kubeconfigServerKey).(string)
	return server, ok
}

// WithKubeCredsFromHeaders extracts Kubernetes credentials from HTTP headers
// and adds them to the context. Supported headers:
//   - Authorization: Bearer <token> - extracted and added via WithKubeToken
//   - X-Kubernetes-Server: <url> - extracted and added via WithKubeServer
//
// If headers are nil or the specific headers are not present, the context
// is returned unchanged (fallback to default kubeconfig behavior).
func WithKubeCredsFromHeaders(ctx context.Context, headers http.Header) context.Context {
	if headers == nil {
		return ctx
	}

	// Extract Authorization Bearer token
	if auth := headers.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != "" {
			ctx = WithKubeToken(ctx, token)
		}
	}

	// Extract Kubernetes API server URL
	if server := headers.Get("X-Kubernetes-Server"); server != "" {
		ctx = WithKubeServer(ctx, server)
	}

	return ctx
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

// outputFormat stores the configured output format for MCP responses
var outputFormat = "json"

// SetOutputFormat sets the output format for MCP responses.
// Valid values are "json" (default) or "text".
func SetOutputFormat(format string) {
	if format == "" {
		outputFormat = "json"
	} else {
		outputFormat = format
	}
}

// GetOutputFormat returns the configured output format for MCP responses.
func GetOutputFormat() string {
	return outputFormat
}

// CommandResponse represents the structured response from command execution
type CommandResponse struct {
	Command     string `json:"command"`
	ReturnValue int    `json:"return_value"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
}

// RunKubectlMTVCommand executes a kubectl-mtv command and returns structured JSON
// It accepts a context which may contain a Kubernetes token and/or server URL for authentication.
// If a token is present in the context, it will be passed via the --token flag.
// If a server URL is present in the context, it will be passed via the --server flag.
// If neither is present, it falls back to the default kubeconfig behavior.
// If dry run mode is enabled in the context, it returns a teaching response instead of executing.
func RunKubectlMTVCommand(ctx context.Context, args []string) (string, error) {
	// Check if we have a server URL in the context and prepend --server flag
	// Server flag is prepended first so it appears before --token in the final command
	if server, ok := GetKubeServer(ctx); ok && server != "" {
		args = append([]string{"--server", server}, args...)
	}

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

	// Resolve environment variable references for sensitive flags (e.g., $VCENTER_PASSWORD)
	// This is done after dry run check so dry run shows $VAR syntax, not resolved values
	resolvedArgs, err := ResolveSensitiveFlagEnvVars(args)
	if err != nil {
		return "", fmt.Errorf("failed to resolve environment variables: %w", err)
	}

	cmd := exec.Command("kubectl-mtv", resolvedArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout of 120 seconds
	timer := time.AfterFunc(120*time.Second, func() {
		_ = cmd.Process.Kill()
	})
	defer timer.Stop()

	err = cmd.Run()

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
// It accepts a context which may contain a Kubernetes token and/or server URL for authentication.
// If a token is present in the context, it will be passed via the --token flag.
// If a server URL is present in the context, it will be passed via the --server flag.
// If neither is present, it falls back to the default kubeconfig behavior.
// If dry run mode is enabled in the context, it returns a teaching response instead of executing.
func RunKubectlCommand(ctx context.Context, args []string) (string, error) {
	// Check if we have a server URL in the context and prepend --server flag
	// Server flag is prepended first so it appears before --token in the final command
	if server, ok := GetKubeServer(ctx); ok && server != "" {
		args = append([]string{"--server", server}, args...)
	}

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

// sensitiveFlags defines flags whose values should be redacted in logs/output
// and which support environment variable resolution (${ENV_VAR} syntax).
var sensitiveFlags = map[string]bool{
	"--password":                 true,
	"-p":                         true, // shorthand for password
	"--token":                    true,
	"-T":                         true, // shorthand for token
	"--offload-vsphere-password": true,
	"--offload-storage-password": true,
	"--target-secret-access-key": true, // AWS secret key for EC2
}

// resolveEnvVar resolves an environment variable reference.
// Only values in ${VAR_NAME} format are treated as env var references.
// This allows literal passwords starting with $ (e.g., "$ecureP@ss") to work correctly.
// Returns the resolved value or an error if the env var is not set.
func resolveEnvVar(value string) (string, error) {
	// Only recognize ${VAR_NAME} syntax for env var references
	// This allows literal passwords starting with $ to work correctly
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envName := strings.TrimPrefix(strings.TrimSuffix(value, "}"), "${")
		if envName == "" {
			return "", fmt.Errorf("empty environment variable name in ${}")
		}
		resolved := os.Getenv(envName)
		if resolved == "" {
			return "", fmt.Errorf("environment variable %s is not set", envName)
		}
		return resolved, nil
	}
	return value, nil
}

// ResolveSensitiveFlagEnvVars resolves environment variable references for sensitive flag values.
// This allows users to pass ${ENV_VAR_NAME} instead of actual secrets.
// Only sensitive flags (passwords, tokens, etc.) are resolved to prevent unintended expansion.
func ResolveSensitiveFlagEnvVars(args []string) ([]string, error) {
	result := make([]string, len(args))
	copy(result, args)

	for i := 0; i < len(result)-1; i++ {
		if sensitiveFlags[result[i]] {
			resolved, err := resolveEnvVar(result[i+1])
			if err != nil {
				return nil, err
			}
			result[i+1] = resolved
		}
	}
	return result, nil
}

// formatShellCommand formats a command and args into a display string
// It sanitizes sensitive parameters like passwords and tokens
// Note: This is for display/logging only. Actual command execution uses exec.Command()
// which handles arguments directly without shell interpretation.
func formatShellCommand(cmd string, args []string) string {

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

// UnmarshalJSONResponse unmarshals a JSON string response into a native object.
// This is needed because the MCP SDK expects a native object, not a JSON string.
//
// The function also parses the stdout field if it contains JSON:
//   - If stdout is a JSON object or array, it's parsed and moved to a "data" field
//   - If stdout is plain text, it's renamed to "output" for clarity
//
// This makes responses clearer for LLMs by avoiding double-encoded strings.
func UnmarshalJSONResponse(responseJSON string) (map[string]interface{}, error) {
	var cmdResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseJSON), &cmdResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	// Try to parse stdout as JSON
	if stdout, ok := cmdResponse["stdout"].(string); ok && stdout != "" {
		stdout = strings.TrimSpace(stdout)

		// Try parsing as JSON object
		var jsonObj map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &jsonObj); err == nil {
			delete(cmdResponse, "stdout")
			cmdResponse["data"] = jsonObj
			return cmdResponse, nil
		}

		// Try parsing as JSON array
		var jsonArr []interface{}
		if err := json.Unmarshal([]byte(stdout), &jsonArr); err == nil {
			delete(cmdResponse, "stdout")
			cmdResponse["data"] = jsonArr
			return cmdResponse, nil
		}

		// Not JSON - rename to "output" for clarity (plain text)
		delete(cmdResponse, "stdout")
		cmdResponse["output"] = stdout
	}

	return cmdResponse, nil
}
