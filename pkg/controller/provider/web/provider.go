package web

import (
	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"

	"net/http"
)

//
// Routes.
const (
	ProvidersRoot = "/providers"
)

//
// Provider handler.
type ProviderHandler struct {
	base.Handler
}

//
// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(base.ProvidersRoot, h.List)
	e.GET(base.ProvidersRoot+"/", h.List)
}

//
// List resources in a REST collection.
func (h ProviderHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	// OCP
	ocpHandler := &ocp.ProviderHandler{
		Handler: base.Handler{
			Container: h.Container,
		},
	}
	status = ocpHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	ocpList, err := ocpHandler.ListContent(ctx)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	// vSphere
	vSphereHandler := &vsphere.ProviderHandler{
		Handler: base.Handler{
			Container: h.Container,
		},
	}
	status = vSphereHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	vSphereList, err := vSphereHandler.ListContent(ctx)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := Provider{
		api.OpenShift: ocpList,
		api.VSphere:   vSphereList,
	}

	content := r

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h ProviderHandler) Get(ctx *gin.Context) {
}

//
// REST resource.
type Provider map[string]interface{}
