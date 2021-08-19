package base

import (
	"github.com/gin-gonic/gin"
	libcontainer "github.com/konveyor/controller/pkg/inventory/container"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//
// Root - all routes.
const (
	ProvidersRoot = "providers"
	ProviderParam = "provider"
	DetailParam   = "detail"
	NsParam       = "namespace"
	NameParam     = "name"
)

//
// Header.
const (
	ProviderHeader = "X-Provider"
)

//
// Params
type Params = map[string]string

//
// Build link.
func Link(path string, params Params) string {
	for k, v := range params {
		if len(v) > 0 {
			path = strings.Replace(path, ":"+k, v, 1)
		}
	}

	return path
}

//
// Base handler.
type Handler struct {
	libweb.Parity
	libweb.Watched
	libweb.Paged
	// Container
	Container *libcontainer.Container
	// Provider referenced in the request.
	Provider *api.Provider
	// Collector responsible for the provider.
	Collector libcontainer.Collector
	// Resources include details.
	Detail bool
}

//
// Prepare to handle the request.
func (h *Handler) Prepare(ctx *gin.Context) int {
	status := h.Paged.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.Watched.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.setDetail(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.setProvider(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.permit(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

//
// Build link.
func (h *Handler) Link(path string, params Params) string {
	return Link(path, params)
}

//
// Set the provider.
// Set the Provider field and the X-Provider header.
func (h *Handler) setProvider(ctx *gin.Context) (status int) {
	var found bool
	uid := ctx.Param(ProviderParam)
	h.Provider = &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			UID: types.UID(uid),
		},
	}
	if h.Provider.UID != "" {
		if h.Collector, found = h.Container.Get(h.Provider); !found {
			status = http.StatusNotFound
			return
		}
		ctx.Header(ProviderHeader, uid)
		h.Provider = h.Collector.Owner().(*api.Provider)
		status = h.EnsureParity(h.Collector, time.Second*10)
	} else {
		status = http.StatusOK
	}

	return
}

//
// Set detail
func (h *Handler) setDetail(ctx *gin.Context) int {
	q := ctx.Request.URL.Query()
	pDetail := q.Get(DetailParam)
	if len(pDetail) > 0 {
		b, err := strconv.ParseBool(pDetail)
		if err == nil {
			h.Detail = b
		} else {
			return http.StatusBadRequest
		}
	}

	return http.StatusOK
}

//
// Permit request - Authorization.
func (h *Handler) permit(ctx *gin.Context) (status int) {
	status = http.StatusOK
	if Settings.AuthRequired {
		return DefaultAuth.Permit(ctx, h.Provider)
	}

	return
}

//
// Match (compare) paths.
// Determine if the relative path is contained
// in the absolute path.
func (h *Handler) PathMatch(absolute, relative string) (matched bool) {
	absolute = strings.TrimLeft(absolute, "/")
	relative = strings.TrimLeft(relative, "/")
	pathA := strings.Split(absolute, "/")
	pathR := strings.Split(relative, "/")
	a := len(pathA) - 1
	r := len(pathR) - 1
	for {
		if r < 0 {
			matched = true
			break
		}
		if a < 0 {
			break
		}
		if pathA[a] != pathR[r] {
			break
		}
		a--
		r--
	}
	return
}

//
// Match (compare) paths.
// Determine if the paths have the same root.
func (h *Handler) PathMatchRoot(absolute, path string) (matched bool) {
	absolute = strings.TrimLeft(absolute, "/")
	path = strings.TrimLeft(path, "/")
	dcA := strings.Split(absolute, "/")[0]
	dcB := strings.Split(path, "/")[0]
	matched = dcA == dcB
	return
}
