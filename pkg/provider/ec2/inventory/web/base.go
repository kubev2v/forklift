package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Package logger.
var log = logging.WithName("ec2|web")

// Query parameters
const (
	NameParam   = base.NameParam
	LabelPrefix = "label." // Query parameter prefix for label filtering (e.g., label.env=production)
)

// Handler base.
type Handler struct {
	base.Handler
}

// ParseLabels extracts label filters from query parameters.
// Parameters prefixed with "label." are treated as label filters.
// Example: ?label.env=production&label.team=platform
// Returns a map of label name -> value pairs.
func (h Handler) ParseLabels(ctx *gin.Context) libmodel.Labels {
	labels := make(libmodel.Labels)
	q := ctx.Request.URL.Query()

	for key, values := range q {
		if strings.HasPrefix(key, LabelPrefix) && len(values) > 0 {
			labelName := strings.TrimPrefix(key, LabelPrefix)
			if labelName != "" {
				labels[labelName] = values[0]
			}
		}
	}

	return labels
}

// Build predicate from query parameters
func (h Handler) Predicate(ctx *gin.Context) (p libmodel.Predicate) {
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) > 0 {
		// Handle path-based names (e.g., "vpc/name")
		path := strings.Split(name, "/")
		name = path[len(path)-1]
		p = libmodel.Eq(NameParam, name)
	}

	return
}

// PredicateWithLabels builds a predicate that includes both name and label filtering.
// Uses libmodel.Match() which generates efficient SQL with INTERSECT for AND logic.
func (h Handler) PredicateWithLabels(ctx *gin.Context) libmodel.Predicate {
	predicates := []libmodel.Predicate{}

	// Add name predicate if specified
	basePredicate := h.Predicate(ctx)
	if basePredicate != nil {
		predicates = append(predicates, basePredicate)
	}

	// Add label-based predicate if labels are specified
	// libmodel.Match() generates SQL that queries the Label table using INTERSECT
	// for AND logic across multiple labels. The Label table is automatically
	// populated by the Labeler when models with Labels() are inserted.
	labels := h.ParseLabels(ctx)
	if len(labels) > 0 {
		predicates = append(predicates, libmodel.Match(labels))
	}

	// Combine all predicates with AND
	if len(predicates) == 0 {
		return nil
	}
	if len(predicates) == 1 {
		return predicates[0]
	}
	return libmodel.And(predicates...)
}

// Build list options from query parameters and handler state
func (h Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := h.Detail
	if detail > 0 {
		detail = model.MaxDetail
	}
	return libmodel.ListOptions{
		Predicate: h.Predicate(ctx),
		Detail:    detail,
		Page:      &h.Page,
	}
}

// ListOptionsWithLabels builds list options that include label filtering.
func (h Handler) ListOptionsWithLabels(ctx *gin.Context) libmodel.ListOptions {
	detail := h.Detail
	if detail > 0 {
		detail = model.MaxDetail
	}

	return libmodel.ListOptions{
		Predicate: h.PredicateWithLabels(ctx),
		Detail:    detail,
		Page:      &h.Page,
	}
}

// Provider handler.
type ProviderHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot, h.Get)
}

// Get provider info.
func (h *ProviderHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusOK)
}
