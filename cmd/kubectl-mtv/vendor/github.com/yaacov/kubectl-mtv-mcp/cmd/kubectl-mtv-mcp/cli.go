package cmd

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is set via linker flags during build
var Version = "dev"

func Execute() error {
	version := flag.Bool("version", false, "Print version information and exit")
	help := flag.Bool("help", false, "Print help information and exit")
	sse := flag.Bool("sse", false, "Run in SSE (Server-Sent Events) mode over HTTP")
	port := flag.String("port", "8080", "Port to listen on for SSE mode")
	host := flag.String("host", "127.0.0.1", "Host address to bind to for SSE mode")
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
		fmt.Fprintf(os.Stderr, "  SSE mode: The server runs an HTTP server for SSE-based MCP connections.\n")
		return nil
	}

	if *version {
		fmt.Println("kubectl-mtv MCP Server")
		fmt.Printf("Version: %s\n", Version)
		return nil
	}

	if *sse {
		// SSE mode - run HTTP server
		addr := *host + ":" + *port

		handler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
			return CreateReadServer()
		}, nil)

		log.Printf("Starting kubectl-mtv MCP server in SSE mode on %s", addr)
		log.Printf("Connect clients to: http://%s/sse", addr)

		return http.ListenAndServe(addr, handler)
	}

	// Stdio mode - default behavior
	server := CreateReadServer()
	return server.Run(context.Background(), &mcp.StdioTransport{})
}
