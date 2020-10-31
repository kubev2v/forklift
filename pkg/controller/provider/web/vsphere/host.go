package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes
const (
	HostParam      = "host"
	HostCollection = "hosts"
	HostsRoot      = ProviderRoot + "/" + HostCollection
	HostRoot       = HostsRoot + "/:" + HostParam
)

//
// Host handler.
type HostHandler struct {
	base.Handler
}

//
// Add routes to the `gin` router.
func (h *HostHandler) AddRoutes(e *gin.Engine) {
	e.GET(HostsRoot, h.List)
	e.GET(HostsRoot+"/", h.List)
	e.GET(HostRoot, h.Get)
}

//
// List resources in a REST collection.
func (h HostHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Host{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Page: &h.Page,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Host{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h HostHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Host{
		Base: model.Base{
			ID: ctx.Param(HostParam),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &Host{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h HostHandler) Link(p *api.Provider, m *model.Host) string {
	return h.Handler.Link(
		HostRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			HostParam:          m.ID,
		})
}

//
// REST Resource.
type Host struct {
	Resource
	InMaintenanceMode bool          `json:"inMaintenance"`
	ProductName       string        `json:"productName"`
	ProductVersion    string        `json:"productVersion"`
	Thumbprint        string        `json:"thumbprint"`
	Networks          model.RefList `json:"networks"`
	Datastores        model.RefList `json:"datastores"`
	VMs               model.RefList `json:"vms"`
}

//
// Build the resource using the model.
func (r *Host) With(m *model.Host) {
	r.Resource.With(&m.Base)
	r.InMaintenanceMode = m.InMaintenanceMode
	r.ProductVersion = m.ProductVersion
	r.ProductName = m.ProductName
	r.Thumbprint = m.Thumbprint
	r.Networks = *model.RefListPtr().With(m.Networks)
	r.Datastores = *model.RefListPtr().With(m.Datastores)
	r.VMs = *model.RefListPtr().With(m.Vms)
}

//
// As content.
func (r *Host) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
