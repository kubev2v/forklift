package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yaacov/kubectl-mtv-mcp/pkg/mtvmcp"
)

// Version is set via linker flags during build
var Version = "dev"

func Execute() error {
	version := flag.Bool("version", false, "Print version information and exit")
	help := flag.Bool("help", false, "Print help information and exit")
	sse := flag.Bool("sse", false, "Run in SSE (Server-Sent Events) mode over HTTP")
	port := flag.String("port", "8080", "Port to listen on for SSE mode")
	host := flag.String("host", "127.0.0.1", "Host address to bind to for SSE mode")
	tlsCert := flag.String("tls-cert", "", "Path to TLS certificate file (enables HTTPS)")
	tlsKey := flag.String("tls-key", "", "Path to TLS private key file (enables HTTPS)")
	flag.Parse()

	if *help {
		fmt.Fprintf(os.Stderr, "kubectl-mtv MCP Server\n\n")
		fmt.Fprintf(os.Stderr, "This is an MCP (Model Context Protocol) server that provides comprehensive\n")
		fmt.Fprintf(os.Stderr, "access to kubectl-mtv resources including read operations for monitoring\n")
		fmt.Fprintf(os.Stderr, "and troubleshooting, as well as write operations for managing MTV resources.\n")
		fmt.Fprintf(os.Stderr, "USE WITH CAUTION: Write operations can create, modify, and delete resources.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nModes:\n")
		fmt.Fprintf(os.Stderr, "  Default: The server communicates via stdio using the MCP protocol.\n")
		fmt.Fprintf(os.Stderr, "  SSE mode: The server runs an HTTP/HTTPS server for SSE-based MCP connections.\n")
		fmt.Fprintf(os.Stderr, "\nTLS/HTTPS:\n")
		fmt.Fprintf(os.Stderr, "  To enable HTTPS, provide both --tls-cert and --tls-key flags.\n")
		fmt.Fprintf(os.Stderr, "  Without these flags, the server runs over HTTP (not secure for production).\n")
		return nil
	}

	if *version {
		fmt.Println("kubectl-mtv MCP Server")
		fmt.Printf("Version: %s\n", Version)
		return nil
	}

	if *sse {
		// SSE mode - run HTTP/HTTPS server
		addr := *host + ":" + *port

		// Validate TLS configuration
		useTLS := false
		if *tlsCert != "" || *tlsKey != "" {
			if *tlsCert == "" || *tlsKey == "" {
				return fmt.Errorf("both --tls-cert and --tls-key must be provided for HTTPS")
			}
			useTLS = true
		}

		// Create SSE handler
		sseHandler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return CreateReadServer()
		}, nil)

		// Wrap handler with middleware to extract token from Authorization header
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				// Check if it's a Bearer token
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					token := parts[1]
					// Add token to request context
					ctx := mtvmcp.WithKubeToken(r.Context(), token)
					r = r.WithContext(ctx)
					log.Printf("Token received via Authorization header (length: %d)", len(token))
				}
			}
			sseHandler.ServeHTTP(w, r)
		})

		if useTLS {
			protocol := "https"
			log.Printf("Starting kubectl-mtv MCP server in SSE mode on %s", addr)
			log.Printf("Protocol: HTTPS (TLS enabled)")
			log.Printf("TLS Certificate: %s", *tlsCert)
			log.Printf("TLS Key: %s", *tlsKey)
			log.Printf("Connect clients to: %s://%s/sse", protocol, addr)
			log.Printf("Token authentication: Enabled via Authorization header (Bearer token)")

			return http.ListenAndServeTLS(addr, *tlsCert, *tlsKey, handler)
		} else {
			protocol := "http"
			log.Printf("Starting kubectl-mtv MCP server in SSE mode on %s", addr)
			log.Printf("Protocol: HTTP (TLS disabled - use --tls-cert and --tls-key for HTTPS)")
			log.Printf("Connect clients to: %s://%s/sse", protocol, addr)
			log.Printf("Token authentication: Enabled via Authorization header (Bearer token)")

			return http.ListenAndServe(addr, handler)
		}
	}

	// Stdio mode - default behavior
	server := CreateReadServer()
	return server.Run(context.Background(), &mcp.StdioTransport{})
}
