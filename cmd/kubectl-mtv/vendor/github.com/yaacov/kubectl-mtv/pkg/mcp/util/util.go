package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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
var outputFormat = "text"

// maxResponseChars limits the size of text output returned to the LLM.
// Long responses fill the context window and increase the chance of small LLMs
// losing track of the tool protocol. When > 0, the "output" field is truncated
// to this many characters with a hint to use structured queries.
// 0 means no truncation (default).
var maxResponseChars int

// SetMaxResponseChars sets the maximum number of characters for text output.
// 0 disables truncation.
func SetMaxResponseChars(n int) {
	maxResponseChars = n
}

// GetMaxResponseChars returns the configured max response chars limit.
func GetMaxResponseChars() int {
	return maxResponseChars
}

// SetOutputFormat sets the output format for MCP responses.
// Valid values are "text" (default, table output) or "json".
func SetOutputFormat(format string) {
	if format == "" {
		outputFormat = "text"
	} else {
		outputFormat = format
	}
}

// GetOutputFormat returns the configured output format for MCP responses.
func GetOutputFormat() string {
	return outputFormat
}

// defaultKubeServer stores the default Kubernetes API server URL set via CLI flags.
// This is used as a fallback when no server URL is provided in the request context (HTTP headers).
var defaultKubeServer string

// SetDefaultKubeServer sets the default Kubernetes API server URL from CLI flags.
func SetDefaultKubeServer(server string) {
	defaultKubeServer = server
}

// GetDefaultKubeServer returns the default Kubernetes API server URL set via CLI flags.
func GetDefaultKubeServer() string {
	return defaultKubeServer
}

// defaultKubeToken stores the default Kubernetes authentication token set via CLI flags.
// This is used as a fallback when no token is provided in the request context (HTTP headers).
var defaultKubeToken string

// SetDefaultKubeToken sets the default Kubernetes authentication token from CLI flags.
func SetDefaultKubeToken(token string) {
	defaultKubeToken = token
}

// GetDefaultKubeToken returns the default Kubernetes authentication token set via CLI flags.
func GetDefaultKubeToken() string {
	return defaultKubeToken
}

// defaultInsecureSkipTLS stores whether to skip TLS verification for Kubernetes API connections.
// When true, --insecure-skip-tls-verify is prepended to every subprocess invocation.
var defaultInsecureSkipTLS bool

// SetDefaultInsecureSkipTLS sets the default TLS skip verification flag.
func SetDefaultInsecureSkipTLS(skip bool) {
	defaultInsecureSkipTLS = skip
}

// GetDefaultInsecureSkipTLS returns whether TLS verification should be skipped.
func GetDefaultInsecureSkipTLS() bool {
	return defaultInsecureSkipTLS
}

// CommandResponse represents the structured response from command execution
type CommandResponse struct {
	Command     string `json:"command"`
	ReturnValue int    `json:"return_value"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
}

// selfExePath caches the path to the currently running executable.
// This ensures the MCP server always calls its own binary rather than
// whatever "kubectl-mtv" happens to be on PATH (which may be an older version).
var selfExePath = func() string {
	exe, err := os.Executable()
	if err != nil {
		return "kubectl-mtv" // fallback to PATH lookup
	}
	return exe
}()

// RunKubectlMTVCommand executes a kubectl-mtv command and returns structured JSON
// It accepts a context which may contain a Kubernetes token and/or server URL for authentication.
// If a token is present in the context, it will be passed via the --token flag.
// If a server URL is present in the context, it will be passed via the --server flag.
// If neither is present, it falls back to CLI default values, then to the default kubeconfig behavior.
// Precedence: context (HTTP headers) > CLI defaults > kubeconfig (implicit).
// If dry run mode is enabled in the context, it returns a teaching response instead of executing.
func RunKubectlMTVCommand(ctx context.Context, args []string) (string, error) {
	// Prepend --insecure-skip-tls-verify when configured (before server/token)
	if defaultInsecureSkipTLS {
		args = append([]string{"--insecure-skip-tls-verify"}, args...)
	}

	// Check context first (HTTP headers), then fall back to CLI defaults for --server flag
	// Server flag is prepended first so it appears before --token in the final command
	if server, ok := GetKubeServer(ctx); ok && server != "" {
		args = append([]string{"--server", server}, args...)
	} else if defaultKubeServer != "" {
		args = append([]string{"--server", defaultKubeServer}, args...)
	}

	// Check context first (HTTP headers), then fall back to CLI defaults for --token flag
	if token, ok := GetKubeToken(ctx); ok && token != "" {
		// Insert --token flag at the beginning of args (after subcommand if present)
		// This ensures it's processed before any other flags
		args = append([]string{"--token", token}, args...)
	} else if defaultKubeToken != "" {
		log.Println("[auth] RunKubectlMTVCommand: using token from CLI defaults")
		args = append([]string{"--token", defaultKubeToken}, args...)
	} else {
		log.Println("[auth] RunKubectlMTVCommand: NO token available (context or defaults)")
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
	resolvedArgs, err := ResolveEnvVars(args)
	if err != nil {
		return "", fmt.Errorf("failed to resolve environment variables: %w", err)
	}

	cmd := exec.Command(selfExePath, resolvedArgs...)

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
// If neither is present, it falls back to CLI default values, then to the default kubeconfig behavior.
// Precedence: context (HTTP headers) > CLI defaults > kubeconfig (implicit).
// If dry run mode is enabled in the context, it returns a teaching response instead of executing.
func RunKubectlCommand(ctx context.Context, args []string) (string, error) {
	// Prepend --insecure-skip-tls-verify when configured (before server/token)
	if defaultInsecureSkipTLS {
		args = append([]string{"--insecure-skip-tls-verify"}, args...)
	}

	// Check context first (HTTP headers), then fall back to CLI defaults for --server flag
	// Server flag is prepended first so it appears before --token in the final command
	if server, ok := GetKubeServer(ctx); ok && server != "" {
		args = append([]string{"--server", server}, args...)
	} else if defaultKubeServer != "" {
		args = append([]string{"--server", defaultKubeServer}, args...)
	}

	// Check context first (HTTP headers), then fall back to CLI defaults for --token flag
	if token, ok := GetKubeToken(ctx); ok && token != "" {
		// Insert --token flag at the beginning of args (after subcommand if present)
		// This ensures it's processed before any other flags
		args = append([]string{"--token", token}, args...)
	} else if defaultKubeToken != "" {
		log.Println("[auth] RunKubectlCommand: using token from CLI defaults")
		args = append([]string{"--token", defaultKubeToken}, args...)
	} else {
		log.Println("[auth] RunKubectlCommand: NO token available (context or defaults)")
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

// sensitiveFlags defines flags whose values should be redacted in logs/output.
// These are security-sensitive flags like passwords and tokens.
var sensitiveFlags = map[string]bool{
	"--password":                 true,
	"-p":                         true, // shorthand for password
	"--token":                    true,
	"-T":                         true, // shorthand for token
	"--offload-vsphere-password": true,
	"--offload-storage-password": true,
	"--target-secret-access-key": true, // AWS secret key for EC2
}

// envVarPattern matches ${VAR_NAME} references embedded anywhere in a string.
// Only the ${VAR} syntax (with curly braces) is recognized as an env var reference.
// This allows literal values starting with $ (e.g., "$ecureP@ss") to pass through unchanged.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// resolveEnvVar resolves environment variable references within a string value.
// It supports both whole-value references (e.g., "${GOVC_URL}") and embedded references
// (e.g., "${GOVC_URL}/sdk", "https://${HOST}:${PORT}/api").
// Only the ${VAR_NAME} syntax with curly braces is recognized, so bare $VAR or literal
// values starting with $ (e.g., "$ecureP@ss") pass through unchanged.
// Returns an error if any referenced env var is not set.
func resolveEnvVar(value string) (string, error) {
	if !strings.Contains(value, "${") {
		return value, nil
	}

	var resolveErr error
	result := envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		if resolveErr != nil {
			return match
		}
		// Extract the variable name from ${VAR_NAME}
		envName := match[2 : len(match)-1]
		if envName == "" {
			resolveErr = fmt.Errorf("empty environment variable name in ${}")
			return match
		}
		resolved := os.Getenv(envName)
		if resolved == "" {
			resolveErr = fmt.Errorf("environment variable %s is not set", envName)
			return match
		}
		return resolved
	})

	if resolveErr != nil {
		return "", resolveErr
	}
	return result, nil
}

// ResolveEnvVars resolves environment variable references for all argument values.
// This allows users to pass ${ENV_VAR_NAME} instead of literal values for any flag or argument.
// Environment variables are resolved for any value matching the ${VAR_NAME} pattern.
func ResolveEnvVars(args []string) ([]string, error) {
	result := make([]string, len(args))
	copy(result, args)

	for i := 0; i < len(result); i++ {
		// Skip flags themselves (only resolve values)
		if strings.HasPrefix(result[i], "-") {
			continue
		}
		resolved, err := resolveEnvVar(result[i])
		if err != nil {
			return nil, err
		}
		result[i] = resolved
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
// Post-processing to help small LLMs:
//   - The "command" field (full CLI command string) is stripped to prevent models
//     from mimicking CLI syntax instead of using structured MCP tool calls.
//   - Empty "stderr" is removed to reduce noise.
//   - The "output" field is truncated to maxResponseChars (if configured) to keep
//     responses within manageable context window sizes.
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
			cleanupResponse(cmdResponse)
			return cmdResponse, nil
		}

		// Try parsing as JSON array
		var jsonArr []interface{}
		if err := json.Unmarshal([]byte(stdout), &jsonArr); err == nil {
			delete(cmdResponse, "stdout")
			cmdResponse["data"] = jsonArr
			cleanupResponse(cmdResponse)
			return cmdResponse, nil
		}

		// Not JSON - rename to "output" for clarity (plain text)
		delete(cmdResponse, "stdout")
		cmdResponse["output"] = stdout
	}

	cleanupResponse(cmdResponse)
	return cmdResponse, nil
}

// cleanupResponse removes noise from tool responses to help small LLMs stay on track.
//   - Strips the "command" field (full CLI echo like "kubectl-mtv get plan --namespace demo")
//     which causes small models to mimic CLI syntax instead of using structured tool calls.
//   - Removes empty "stderr" to reduce noise.
//   - Truncates the "output" field if maxResponseChars is configured.
func cleanupResponse(data map[string]interface{}) {
	// Strip CLI command echo — this is the #1 cause of small LLMs generating
	// raw CLI strings instead of structured {command, flags} tool calls.
	delete(data, "command")

	// Remove empty stderr to reduce noise
	if stderr, ok := data["stderr"].(string); ok && strings.TrimSpace(stderr) == "" {
		delete(data, "stderr")
	}

	// Truncate long text output if configured
	if maxResponseChars > 0 {
		if output, ok := data["output"].(string); ok && len(output) > maxResponseChars {
			truncated := output[:maxResponseChars]
			truncated += fmt.Sprintf("\n\n[truncated at %d chars. Use flags: {output: \"json\"} with fields: [\"name\", \"id\"] to get specific data]", maxResponseChars)
			data["output"] = truncated
		}
	}
}
