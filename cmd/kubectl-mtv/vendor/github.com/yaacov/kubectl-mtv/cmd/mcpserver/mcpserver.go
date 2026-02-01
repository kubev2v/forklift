package mcpserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/discovery"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/tools"
	"github.com/yaacov/kubectl-mtv/pkg/mcp/util"
	"github.com/yaacov/kubectl-mtv/pkg/version"
)

var (
	sse          bool
	port         string
	host         string
	certFile     string
	keyFile      string
	outputFormat string
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
  --sse:   HTTP server mode with optional TLS

Security:
  --cert-file:   Path to TLS certificate file (enables TLS when both cert and key provided)
  --key-file:    Path to TLS private key file (enables TLS when both cert and key provided)

SSE Mode Authentication (HTTP Headers):
  In SSE mode, the following HTTP headers are supported for Kubernetes authentication:

  Authorization: Bearer <token>
    Kubernetes authentication token. Passed to kubectl via --token flag.

  X-Kubernetes-Server: <url>
    Kubernetes API server URL. Passed to kubectl via --server flag.

  If headers are not provided, the server falls back to the default kubeconfig behavior.

Quick Setup for AI Assistants:

Claude Desktop: claude mcp add kubectl-mtv kubectl mtv mcp-server
Cursor IDE: Settings → MCP → Add Server (Name: kubectl-mtv, Command: kubectl, Args: mtv mcp-server)

Manual Claude config: Add to claude_desktop_config.json:
  "kubectl-mtv": {"command": "kubectl", "args": ["mtv", "mcp-server"]}`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Validate output format - only "json" and "text" are supported in MCP mode
			validFormats := map[string]bool{"json": true, "text": true}
			if !validFormats[outputFormat] {
				return fmt.Errorf("invalid --output-format value %q: must be one of: json, text", outputFormat)
			}

			// Set the output format for MCP responses
			util.SetOutputFormat(outputFormat)

			// Create a context that listens for interrupt signals
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			if sse {
				// SSE mode - run HTTP server
				addr := net.JoinHostPort(host, port)

				// Create MCP handler
				handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
					server, err := createMCPServer()
					if err != nil {
						log.Printf("Failed to create server: %v", err)
						return nil
					}
					return server
				}, nil)

				server := &http.Server{
					Addr:    addr,
					Handler: handler,
				}

				// Start server in a goroutine
				errChan := make(chan error, 1)
				go func() {
					// Check if TLS should be enabled (both cert and key files provided)
					useTLS := certFile != "" && keyFile != ""

					if useTLS {
						log.Printf("Starting kubectl-mtv MCP server with TLS in SSE mode on %s", addr)
						log.Printf("Using cert: %s, key: %s", certFile, keyFile)
						log.Printf("Connect clients to: https://%s/sse", addr)
						errChan <- server.ListenAndServeTLS(certFile, keyFile)
					} else {
						log.Printf("Starting kubectl-mtv MCP server in SSE mode on %s", addr)
						log.Printf("Connect clients to: http://%s/sse", addr)
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
					log.Println("\nShutting down server...")
					// Give the server 5 seconds to gracefully shutdown
					shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer shutdownCancel()
					if err := server.Shutdown(shutdownCtx); err != nil {
						log.Printf("Server shutdown error: %v", err)
					}
				}
				return nil
			}

			// Stdio mode - default behavior
			server, err := createMCPServer()
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			log.Println("Starting kubectl-mtv MCP server in stdio mode")
			log.Println("Server is ready and listening for MCP protocol messages on stdin/stdout")

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
				log.Println("\nShutting down server...")
				cancel()
				// Give the server a moment to clean up
				time.Sleep(100 * time.Millisecond)
				return nil
			}
		},
	}

	// Add flags matching the MCP CLI flags
	mcpCmd.Flags().BoolVar(&sse, "sse", false, "Run in SSE (Server-Sent Events) mode over HTTP")
	mcpCmd.Flags().StringVar(&port, "port", "8080", "Port to listen on for SSE mode")
	mcpCmd.Flags().StringVar(&host, "host", "127.0.0.1", "Host address to bind to for SSE mode")
	mcpCmd.Flags().StringVar(&certFile, "cert-file", "", "Path to TLS certificate file (enables TLS when used with --key-file)")
	mcpCmd.Flags().StringVar(&keyFile, "key-file", "", "Path to TLS private key file (enables TLS when used with --cert-file)")
	mcpCmd.Flags().StringVar(&outputFormat, "output-format", "json", "Default output format for commands: json or text")

	return mcpCmd
}

// createMCPServer creates the MCP server with dynamically discovered tools.
// Discovery happens at startup using kubectl-mtv help --machine.
func createMCPServer() (*mcp.Server, error) {
	ctx := context.Background()
	registry, err := discovery.NewRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover commands: %w", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "kubectl-mtv",
		Version: version.ClientVersion,
	}, nil)

	// Register the three dynamic tools
	mcp.AddTool(server, tools.GetMTVReadTool(registry), tools.HandleMTVRead(registry))
	mcp.AddTool(server, tools.GetMTVWriteTool(registry), tools.HandleMTVWrite(registry))
	mcp.AddTool(server, tools.GetKubectlDebugTool(), tools.HandleKubectlDebug)

	return server, nil
}
