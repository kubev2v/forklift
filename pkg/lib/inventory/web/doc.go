package web

import "github.com/konveyor/forklift-controller/pkg/lib/inventory/container"

// Build new web server.
func New(c *container.Container, routes ...RequestHandler) *WebServer {
	return &WebServer{
		Handlers:  routes,
		Container: c,
	}
}
