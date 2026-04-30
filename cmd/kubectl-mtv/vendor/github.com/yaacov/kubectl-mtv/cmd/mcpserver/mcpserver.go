package mcpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/discovery"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/tools"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
	"github.com/yaacov/kubectl-mtv/pkg/version"
	"k8s.io/klog/v2"
)

var (
	httpMode         bool
	port             string
	host             string
	certFile         string
	keyFile          string
	outputFormat     string
	kubeServer       string
	kubeToken        string
	insecureSkipTLS  bool
	maxResponseChars int
	readOnly         bool
)

// NewMCPServerCmd creates the mcp-server command
func NewMCPServerCmd() *cobra.Command {
	mcpCmd := &cobra.Command{
		Use:   "mcp-server",
		Short: "Start the MCP (Model Context Protocol) server",
		Long: `Start the MCP (Model Context Protocol) server for kubectl-mtv.

This server provides AI assistants with access to kubectl-mtv resources.
USE WITH CAUTION: Includes write operations that can modify resources.

Modes:
  Default: Stdio mode for AI assistant integration
  --http:  HTTP server mode using Streamable HTTP transport

Read-Only Mode:
  --read-only: Disables all write operations (mtv_write tool not registered)
               Only read operations will be available to AI assistants

Security:
  --cert-file:   Path to TLS certificate file (enables TLS when both cert and key provided)
  --key-file:    Path to TLS private key file (enables TLS when both cert and key provided)

Kubernetes Authentication:
  --server:  Kubernetes API server URL (passed to kubectl via --server flag)
  --token:   Kubernetes authentication token (passed to kubectl via --token flag)

  These flags set default credentials for all requests. They work in both stdio and HTTP modes.

HTTP Mode Authentication (HTTP Headers):
  In HTTP mode, the following HTTP headers are supported for per-request authentication:

  Authorization: Bearer <token>
    Kubernetes authentication token. Passed to kubectl via --token flag.

  X-Kubernetes-Server: <url>
    Kubernetes API server URL. Passed to kubectl via --server flag.

  Precedence: HTTP headers (per-request) > CLI flags (--server/--token) > kubeconfig (implicit).

  Each HTTP POST carries its own headers, so token rotation works seamlessly.

Quick Setup for AI Assistants:

Claude Desktop: claude mcp add kubectl-mtv kubectl mtv mcp-server
Cursor IDE: Settings → MCP → Add Server (Name: kubectl-mtv, Command: kubectl, Args: mtv mcp-server)

Manual Claude config: Add to claude_desktop_config.json:
  "kubectl-mtv": {"command": "kubectl", "args": ["mtv", "mcp-server"]}`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Validate output format
			validFormats := map[string]bool{"json": true, "text": true, "markdown": true}
			if !validFormats[outputFormat] {
				return fmt.Errorf("invalid --output-format value %q: must be one of: json, text, markdown", outputFormat)
			}

			// Set the output format for MCP responses
			util.SetOutputFormat(outputFormat)

			// Set max response size (helps small LLMs stay within context window)
			util.SetMaxResponseChars(maxResponseChars)

			// Set default Kubernetes credentials from CLI flags
			// These serve as fallback when HTTP headers don't provide credentials
			util.SetDefaultKubeServer(kubeServer)
			util.SetDefaultKubeToken(kubeToken)
			util.SetDefaultInsecureSkipTLS(insecureSkipTLS)

			// Propagate verbosity from the inherited global --verbose flag
			// so tool subprocesses produce matching debug output
			if v, err := cobraCmd.Flags().GetInt("verbose"); err == nil {
				util.SetDefaultVerbosity(v)
			}

			// Create a context that listens for interrupt signals
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			if httpMode {
				// HTTP mode - run Streamable HTTP server
				addr := net.JoinHostPort(host, port)

				// Discover commands once at startup; the schema is static.
				registry, err := discovery.NewRegistry(ctx)
				if err != nil {
					return fmt.Errorf("failed to discover commands: %w", err)
				}

				innerHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
					server, err := createMCPServerWithRegistry(registry, readOnly)
					if err != nil {
						klog.Errorf("Failed to create server: %v", err)
						return nil
					}
					return server
				}, &mcp.StreamableHTTPOptions{})

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if origin := r.Header.Get("Origin"); origin != "" {
						parsed, err := url.Parse(origin)
						if err != nil || parsed.Host != r.Host {
							http.Error(w, "Forbidden", http.StatusForbidden)
							return
						}
					}

					if r.Method == http.MethodPost {
						if auth := r.Header.Get("Authorization"); auth != "" {
							scheme := "unknown"
							if parts := strings.SplitN(auth, " ", 2); len(parts) > 0 {
								scheme = parts[0]
							}
							klog.V(2).Infof("[auth] SERVER: POST request with Authorization: %s [REDACTED]", scheme)
						} else {
							klog.V(2).Info("[auth] SERVER: POST request with NO Authorization header")
						}
					}
					innerHandler.ServeHTTP(w, r)
				})

				server := &http.Server{
					Addr:              addr,
					Handler:           handler,
					ReadHeaderTimeout: 5 * time.Second,
				}

				// Start server in a goroutine
				errChan := make(chan error, 1)
				go func() {
					useTLS := certFile != "" && keyFile != ""

					if useTLS {
						klog.V(1).Infof("Starting kubectl-mtv MCP server with TLS in HTTP mode on %s", addr)
						klog.V(1).Infof("Using cert: %s, key: %s", certFile, keyFile)
						klog.V(1).Infof("Connect clients to: https://%s/mcp", addr)
						errChan <- server.ListenAndServeTLS(certFile, keyFile)
					} else {
						klog.V(1).Infof("Starting kubectl-mtv MCP server in HTTP mode on %s", addr)
						klog.V(1).Infof("Connect clients to: http://%s/mcp", addr)
						errChan <- server.ListenAndServe()
					}
				}()

				// Wait for either an error or interrupt signal
				select {
				case err := <-errChan:
					if err != nil && err != http.ErrServerClosed {
						return err
					}
				case <-sigChan:
					klog.V(1).Info("Shutting down server...")
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutdownCancel()
					if err := server.Shutdown(shutdownCtx); err != nil {
						klog.Errorf("Server shutdown error: %v", err)
					}
				}
				return nil
			}

			// Stdio mode - default behavior
			server, err := createMCPServer(readOnly)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			klog.V(1).Info("Starting kubectl-mtv MCP server in stdio mode")
			klog.V(1).Info("Server is ready and listening for MCP protocol messages on stdin/stdout")

			// Run server in a goroutine
			errChan := make(chan error, 1)
			go func() {
				errChan <- server.Run(ctx, &mcp.StdioTransport{})
			}()

			// Wait for either an error or interrupt signal
			select {
			case err := <-errChan:
				return err
			case <-sigChan:
				klog.V(1).Info("Shutting down server...")
				cancel()
				// Give the server a moment to clean up
				time.Sleep(100 * time.Millisecond)
				return nil
			}
		},
	}

	mcpCmd.Flags().BoolVar(&httpMode, "http", false, "Run in HTTP mode using Streamable HTTP transport")
	mcpCmd.Flags().StringVar(&port, "port", "8080", "Port to listen on for HTTP mode")
	mcpCmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host address to bind to for HTTP mode")
	mcpCmd.Flags().StringVar(&certFile, "cert-file", "", "Path to TLS certificate file (enables TLS when used with --key-file)")
	mcpCmd.Flags().StringVar(&keyFile, "key-file", "", "Path to TLS private key file (enables TLS when used with --cert-file)")
	mcpCmd.Flags().StringVar(&outputFormat, "output-format", "markdown", "Default output format for commands: markdown, text (table), or json")
	mcpCmd.Flags().StringVar(&kubeServer, "server", "", "Kubernetes API server URL (passed to kubectl via --server flag)")
	mcpCmd.Flags().StringVar(&kubeToken, "token", "", "Kubernetes authentication token (passed to kubectl via --token flag)")
	mcpCmd.Flags().BoolVar(&insecureSkipTLS, "insecure-skip-tls-verify", false, "Skip TLS certificate verification for Kubernetes API connections")
	mcpCmd.Flags().IntVar(&maxResponseChars, "max-response-chars", 0, "Max characters for text output (0=unlimited). Helps small LLMs by truncating long responses")
	mcpCmd.Flags().BoolVar(&readOnly, "read-only", false, "Run in read-only mode (disables write operations)")

	return mcpCmd
}

// createMCPServer discovers commands and creates the MCP server.
// Used by stdio mode where a single server instance is sufficient.
func createMCPServer(readOnlyMode bool) (*mcp.Server, error) {
	ctx := context.Background()
	registry, err := discovery.NewRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover commands: %w", err)
	}
	return createMCPServerWithRegistry(registry, readOnlyMode)
}

// createMCPServerWithRegistry builds an MCP server from a pre-built registry.
// HTTP mode calls this per-request so that each POST gets its own server
// instance while reusing the (static) command schema discovered at startup.
//
// In HTTP mode, the SDK populates req.Extra.Header on every POST with that
// request's HTTP headers, giving each tool call fresh auth credentials.
// In stdio mode, there are no HTTP headers and we fall back to CLI defaults.
func createMCPServerWithRegistry(registry *discovery.Registry, readOnlyMode bool) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubectl-mtv",
		Version: version.ClientVersion,
	}, &mcp.ServerOptions{
		Instructions: registry.GenerateServerInstructions(),
	})

	tools.AddToolWithCoercion(server, tools.GetMTVReadTool(registry), tools.HandleMTVRead(registry))
	mcp.AddTool(server, tools.GetMTVHelpTool(), tools.HandleMTVHelp)

	if !readOnlyMode {
		tools.AddToolWithCoercion(server, tools.GetMTVWriteTool(registry), tools.HandleMTVWrite(registry))
	} else {
		klog.V(1).Info("Running in read-only mode - write operations disabled")
	}

	return server, nil
}
