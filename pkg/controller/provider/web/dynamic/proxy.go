package dynamic

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
)

// Proxy forwards requests to external provider service or handles watch requests
func (h *Handler) Proxy(ctx *gin.Context) {
	// Extract provider UID and path from URL
	providerUID := ctx.Param("provider")
	path := ctx.Param("path")

	// Handle refresh endpoint (POST /refresh)
	if path == "/refresh" && ctx.Request.Method == http.MethodPost {
		h.Refresh(ctx)
		return
	}

	// Check if this is a watch request (websocket)
	// Watch requests are handled locally via SQLite cache, not proxied
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		h.watch(ctx, providerUID, path)
		return
	}

	// Check if this resource should be served from cache
	// vms, networks, and storage are cached for performance
	if isCachedResource(path) {
		h.serveFromCache(ctx, providerUID, path)
		return
	}

	// All other requests are proxied directly
	h.proxyRequest(ctx, providerUID, path)
}

// proxyRequest forwards the request to the external provider service
func (h *Handler) proxyRequest(ctx *gin.Context, providerUID, path string) {
	providerType := ctx.Param("type")

	// Verify this is a registered dynamic provider type
	config, isDynamic := h.registry.Get(providerType)
	if !isDynamic {
		// Not a dynamic provider type
		ctx.Status(http.StatusNotFound)
		return
	}

	// Get provider from container
	var provider *api.Provider
	collectors := h.Container.List()
	for _, collector := range collectors {
		if string(collector.Owner().GetUID()) == providerUID {
			provider = collector.Owner().(*api.Provider)
			break
		}
	}

	if provider == nil {
		ctx.Status(http.StatusNotFound)
		ctx.Header(base.ReasonHeader, base.UnknownProvider)
		return
	}

	// Verify provider type matches URL
	if string(provider.Type()) != providerType {
		ctx.Status(http.StatusBadRequest)
		ctx.Header(base.ReasonHeader, "Provider type mismatch")
		return
	}

	// Build target URL
	targetURL := config.ServiceURL + path
	if ctx.Request.URL.RawQuery != "" {
		targetURL += "?" + ctx.Request.URL.RawQuery
	}

	log.V(3).Info("Proxying request",
		"provider", provider.Name,
		"method", ctx.Request.Method,
		"path", path,
		"target", targetURL)

	// Create proxy request
	proxyReq, err := http.NewRequest(ctx.Request.Method, targetURL, ctx.Request.Body)
	if err != nil {
		log.Error(err, "Failed to create proxy request")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// Copy headers (except host)
	for key, values := range ctx.Request.Header {
		if key == "Host" {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Add provider context headers
	proxyReq.Header.Set("X-Forklift-Provider-Name", provider.Name)
	proxyReq.Header.Set("X-Forklift-Provider-Namespace", provider.Namespace)
	proxyReq.Header.Set("X-Forklift-Provider-UID", string(provider.UID))

	// Execute request
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Error(err, "Failed to proxy request", "target", targetURL)
		ctx.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			ctx.Header(key, value)
		}
	}

	// Copy response status and body
	ctx.Status(resp.StatusCode)
	_, err = io.Copy(ctx.Writer, resp.Body)
	if err != nil {
		log.Error(err, "Failed to copy response body")
	}
}
