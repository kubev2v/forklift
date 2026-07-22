package web

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

var log = logging.WithName("azure|web")

const (
	NameParam   = base.NameParam
	LabelPrefix = "label."
)

type Handler struct {
	base.Handler
}

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

func (h Handler) Predicate(ctx *gin.Context) (p libmodel.Predicate) {
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) > 0 {
		path := strings.Split(name, "/")
		name = path[len(path)-1]
		p = libmodel.Eq(NameParam, name)
	}

	return
}

func (h Handler) PredicateWithLabels(ctx *gin.Context) libmodel.Predicate {
	predicates := []libmodel.Predicate{}

	basePredicate := h.Predicate(ctx)
	if basePredicate != nil {
		predicates = append(predicates, basePredicate)
	}

	labels := h.ParseLabels(ctx)
	if len(labels) > 0 {
		predicates = append(predicates, libmodel.Match(labels))
	}

	if len(predicates) == 0 {
		return nil
	}
	if len(predicates) == 1 {
		return predicates[0]
	}
	return libmodel.And(predicates...)
}

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

// decodeParam URL-decodes a gin path parameter. Azure resource IDs
// contain slashes that must be percent-encoded in the URL path.
func decodeParam(ctx *gin.Context, name string) string {
	raw := ctx.Param(name)
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return decoded
}
