package web

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/konveyor/controller/pkg/inventory/container"
	"regexp"
	"time"
)

// Root - all routes.
const (
	NsParam      = "ns1"
	NsCollection = "namespaces"
	Root         = "/" + NsCollection + "/:" + NsParam
)

//
// Web server
type WebServer struct {
	// The optional port.  Default: 8080
	Port int
	// Allowed CORS origins.
	AllowedOrigins []string
	// Reference to the container.
	Container *container.Container
	// Handlers
	Handlers []RequestHandler
	// Compiled CORS origins.
	allowedOrigins []*regexp.Regexp
	// TLS.
	TLS struct {
		// Enabled.
		Enabled bool
		// Certificate path.
		Certificate string
		// Key path
		Key string
	}
}

//
// Start the web-server.
// Initializes `gin` with routes and CORS origins.
// Creates an http server to handle TLS
func (w *WebServer) Start() {
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET"},
		AllowHeaders:     []string{"Authorization", "Origin"},
		AllowOriginFunc:  w.allow,
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	w.buildOrigins()
	w.addRoutes(router)
	if w.TLS.Enabled {
		go router.RunTLS(w.address(), w.TLS.Certificate, w.TLS.Key)
	} else {
		go router.Run(w.address())
	}
}

//
// Determine the address.
func (w *WebServer) address() string {
	if w.Port == 0 {
		if w.TLS.Enabled {
			w.Port = 8443
		} else {
			w.Port = 8080
		}
	}

	return fmt.Sprintf(":%d", w.Port)
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
