package web

import (
	"github.com/gin-gonic/gin"
	"github.com/konveyor/controller/pkg/inventory/container"
	"github.com/konveyor/controller/pkg/inventory/model"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//
// Web request handler.
type RequestHandler interface {
	// Add routes to the `gin` router.
	AddRoutes(*gin.Engine)
	// List resources in a REST collection.
	List(*gin.Context)
	// Get a specific REST resource.
	Get(*gin.Context)
}

//
// Paged handler.
type Paged struct {
	// The `page` parameter passed in the request.
	Page model.Page
}

//
// Prepare the handler to fulfil the request.
// Set the `page` field using passed parameters.
func (h *Paged) Prepare(ctx *gin.Context) int {
	status := h.setPage(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// Set the `page` field.
func (h *Paged) setPage(ctx *gin.Context) int {
	q := ctx.Request.URL.Query()
	page := model.Page{
		Limit:  int(^uint(0) >> 1),
		Offset: 0,
	}
	pLimit := q.Get("limit")
	if len(pLimit) != 0 {
		nLimit, err := strconv.Atoi(pLimit)
		if err != nil || nLimit < 0 {
			return http.StatusBadRequest
		}
		page.Limit = nLimit
	}
	pOffset := q.Get("offset")
	if len(pOffset) != 0 {
		nOffset, err := strconv.Atoi(pOffset)
		if err != nil || nOffset < 0 {
			return http.StatusBadRequest
		}
		page.Offset = nOffset
	}

	h.Page = page
	return http.StatusOK
}

//
// Consistent (not-partial) request handler.
type Consistent struct {
}

//
// Ensure that the
func (c *Consistent) EnsureConsistency(r container.Reconciler, w time.Duration) int {
	wait := time.Second * 30
	poll := time.Microsecond * 100
	for {
		mark := time.Now()
		if r.HasConsistency() {
			return http.StatusOK
		}
		if wait > 0 {
			time.Sleep(poll)
			wait -= time.Since(mark)
		} else {
			break
		}
	}

	return http.StatusPartialContent
}

//
// Authorized by k8s bearer token.
type Authorized struct {
	// Bearer token.
	Token string
}

//
// Prepare the handler to fulfil the request.
// Set the `token` field using passed parameters.
func (h *Authorized) Prepare(ctx *gin.Context) int {
	h.setToken(ctx)
	return http.StatusOK
}

//
// Set the `Token` field.
func (h *Authorized) setToken(ctx *gin.Context) {
	header := ctx.GetHeader("Authorization")
	fields := strings.Fields(header)
	if len(fields) == 2 && fields[0] == "Bearer" {
		h.Token = fields[1]
	}
}

//
// Schema (route) handler.
type SchemaHandler struct {
	// The `gin` router.
	router *gin.Engine
	// Schema version
	Version string
	// Schema release.
	Release int
}

//
// Add routes.
func (h *SchemaHandler) AddRoutes(r *gin.Engine) {
	r.GET("/schema", h.List)
	h.router = r
}

//
// List schema.
func (h *SchemaHandler) List(ctx *gin.Context) {
	type Schema struct {
		Version string   `json:"version,omitempty"`
		Release int      `json:"release,omitempty"`
		Paths   []string `json:"paths"`
	}
	schema := Schema{
		Version: h.Version,
		Release: h.Release,
		Paths:   []string{},
	}
	for _, rte := range h.router.Routes() {
		schema.Paths = append(schema.Paths, rte.Path)
	}

	ctx.JSON(http.StatusOK, schema)
}

//
// Not supported.
func (h SchemaHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}
