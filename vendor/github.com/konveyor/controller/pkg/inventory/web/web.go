package web

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/controller/pkg/inventory/container"
	"regexp"
	"time"
)

// Root - all routes.
const (
	Root = "/namespaces/:ns1"
)

//
// Web server
type WebServer struct {
	// Allowed CORS origins.
	AllowedOrigins []string
	// Reference to the container.
	Container *container.Container
	// Handlers
	Handlers []RequestHandler
	// Compiled CORS origins.
	allowedOrigins []*regexp.Regexp
}

//
// Start the web-server.
// Initializes `gin` with routes and CORS origins.
func (w *WebServer) Start() {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Authorization", "Origin"},
		AllowOriginFunc:  w.allow,
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	w.buildOrigins()
	w.addRoutes(r)
	go r.Run()
}

//
// Build a REGEX for each CORS origin.
func (w *WebServer) buildOrigins() {
	w.allowedOrigins = []*regexp.Regexp{}
	for _, r := range w.AllowedOrigins {
		expr, err := regexp.Compile(r)
		if err != nil {
			continue
		}
		w.allowedOrigins = append(w.allowedOrigins, expr)
	}
}

//
// Add the routes.
func (w *WebServer) addRoutes(r *gin.Engine) {
	for _, h := range w.Handlers {
		h.AddRoutes(r)
	}
}

//
// Called by `gin` to perform CORS authorization.
func (w *WebServer) allow(origin string) bool {
	for _, expr := range w.allowedOrigins {
		if expr.MatchString(origin) {
			return true
		}
	}

	return false
}
