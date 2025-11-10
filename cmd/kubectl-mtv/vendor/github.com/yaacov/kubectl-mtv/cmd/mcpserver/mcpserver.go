package mcpserver

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	cmd "github.com/yaacov/kubectl-mtv-mcp/cmd/kubectl-mtv-mcp"
)

var (
	sse  bool
	port string
	host string
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
  --sse:   HTTP server mode for web-based integrations

Quick Setup for AI Assistants:

Claude Desktop: claude mcp add kubectl-mtv kubectl mtv mcp-server
Cursor IDE: Settings → MCP → Add Server (Name: kubectl-mtv, Command: kubectl, Args: mtv mcp-server)

Manual Claude config: Add to claude_desktop_config.json:
  "kubectl-mtv": {"command": "kubectl", "args": ["mtv", "mcp-server"]}`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Create a context that listens for interrupt signals
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Setup signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			if sse {
				// SSE mode - run HTTP server
				addr := net.JoinHostPort(host, port)

				handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
					return cmd.CreateReadServer()
				}, nil)

				server := &http.Server{
					Addr:    addr,
					Handler: handler,
				}

				// Start server in a goroutine
				errChan := make(chan error, 1)
				go func() {
					log.Printf("Starting kubectl-mtv MCP server in SSE mode on %s", addr)
					log.Printf("Connect clients to: http://%s/sse", addr)
					errChan <- server.ListenAndServe()
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
			server := cmd.CreateReadServer()

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

	return mcpCmd
}
