package web

import (
	"github.com/gin-gonic/gin"
	libcontainer "github.com/konveyor/controller/pkg/inventory/container"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"time"
)

//
// Root - all routes.
const (
	Root = libweb.Root + "/providers/:provider"
)

//
// Base handler.
type Base struct {
	libweb.Consistent
	libweb.Paged
	// Container
	Container *libcontainer.Container
	// Provider referenced in the request.
	Provider *api.Provider
	// Reconciler responsible for the provider.
	Reconciler libcontainer.Reconciler
}

func (h *Base) Prepare(ctx *gin.Context) int {
	status := h.Paged.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	status = h.setProvider(ctx)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}

func (h *Base) setProvider(ctx *gin.Context) int {
	var found bool
	h.Provider = &api.Provider{
		ObjectMeta: meta.ObjectMeta{
			Namespace: ctx.Param("ns1"),
			Name:      ctx.Param("provider"),
		},
	}
	if h.Reconciler, found = h.Container.Get(h.Provider); !found {
		return http.StatusNotFound
	}
	status := h.EnsureConsistency(h.Reconciler, time.Second*30)
	if status != http.StatusOK {
		return status
	}

	return http.StatusOK
}
