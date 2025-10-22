package base

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/base"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Root - all routes.
const (
	ProvidersRoot = "providers"
	ProviderParam = "provider"
	DetailParam   = "detail"
	NsParam       = "namespace"
	NameParam     = "name"
)

// Reply Header.
const (
	// Explains reason behind status code.
	ReasonHeader = "X-Reason"
	// Explains 404 caused by provider not found in
	// inventory as opposed to the requested resource
	// not found within the provider in the inventory.
	UnknownProvider = "ProviderNotFound"
)

// Params
type Params = map[string]string

// Build link.
func Link(path string, params Params) string {
	for k, v := range params {
		if len(v) > 0 {
			path = strings.Replace(path, ":"+k, v, 1)
		}
	}

	return path
}

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
	// Resources detail level.
	Detail int
}

// Prepare to handle the request.
func (h *Handler) Prepare(ctx *gin.Context) (int, error) {
	status := h.Paged.Prepare(ctx)
	if status != http.StatusOK {
		return status, nil
	}
	status = h.Watched.Prepare(ctx)
	if status != http.StatusOK {
		return status, nil
	}
	status = h.setDetail(ctx)
	if status != http.StatusOK {
		return status, nil
	}
	status = h.setProvider(ctx)
	if status != http.StatusOK {
		return status, nil
	}
	status, err := h.permit(ctx)
	if status != http.StatusOK {
		return status, err
	}

	return http.StatusOK, nil
}

// Build link.
func (h *Handler) Link(path string, params Params) string {
	return Link(path, params)
}

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
			ctx.Header(ReasonHeader, UnknownProvider)
			status = http.StatusNotFound
			return
		}
		h.Provider = h.Collector.Owner().(*api.Provider)
		status = h.EnsureParity(h.Collector, time.Second*10)
	} else {
		status = http.StatusOK
	}

	return
}

// Set detail
// "all" = MaxDetail.
func (h *Handler) setDetail(ctx *gin.Context) (status int) {
	status = http.StatusOK
	q := ctx.Request.URL.Query()
	pDetail := q.Get(DetailParam)
	if len(pDetail) == 0 {
		return
	}
	if strings.ToLower(pDetail) == "all" {
		h.Detail = base.MaxDetail
		return
	}
	n, err := strconv.Atoi(pDetail)
	if err == nil {
		h.Detail = n
	} else {
		status = http.StatusBadRequest
	}

	return
}

func (h *Handler) Token(ctx *gin.Context) string {
	return DefaultAuth.Token(ctx)
}

// Permit request - Authorization.
func (h *Handler) permit(ctx *gin.Context) (status int, err error) {
	status = http.StatusOK
	if Settings.AuthRequired {
		return DefaultAuth.Permit(ctx, h.Provider)
	}

	return
}

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
