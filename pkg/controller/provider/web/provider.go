package web

import (
	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"

	"net/http"
)

// Package logger.
var log = logging.WithName("web|provider")

// Routes.
const (
	ProvidersRoot = "/providers"
)

// Provider handler.
type ProviderHandler struct {
	base.Handler
}

// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(base.ProvidersRoot, h.List)
	e.GET(base.ProvidersRoot+"/", h.List)
}

// List resources in a REST collection.
func (h ProviderHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	// OCP
	ocpHandler := &ocp.ProviderHandler{
		Handler: ocp.Handler{Handler: base.Handler{
			Container: h.Container,
		}},
	}
	status, err = ocpHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	ocpList, err := ocpHandler.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	// vSphere
	vSphereHandler := &vsphere.ProviderHandler{
		Handler: base.Handler{
			Container: h.Container,
		},
	}
	status, err = vSphereHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	vSphereList, err := vSphereHandler.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	// oVirt
	oVirtHandler := &ovirt.ProviderHandler{
		Handler: base.Handler{
			Container: h.Container,
		},
	}
	status, err = oVirtHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	oVirtList, err := oVirtHandler.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	// OpenStack
	openStackHandler := &openstack.ProviderHandler{
		Handler: base.Handler{
			Container: h.Container,
		},
	}
	status, err = openStackHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	openStackList, err := openStackHandler.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	// OVA
	ovaHandler := &ova.ProviderHandler{
		Handler: base.Handler{
			Container: h.Container,
		},
	}
	status, err = ovaHandler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	ovaList, err := ovaHandler.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := Provider{
		string(api.OpenShift): ocpList,
		string(api.VSphere):   vSphereList,
		string(api.OVirt):     oVirtList,
		string(api.OpenStack): openStackList,
		string(api.Ova):       ovaList,
	}

	content := r

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ProviderHandler) Get(ctx *gin.Context) {
}

// REST resource.
type Provider map[string]interface{}
