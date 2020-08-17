package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes
const (
	HostsRoot = Root + "/hosts"
	HostRoot  = HostsRoot + "/:host"
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
		obj := r.Object(h.Detail)
		content = append(content, obj)
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
			ID: ctx.Param("host"),
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

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Host struct {
	base.Resource
	InMaintenanceMode bool          `json:"inMaintenance"`
	ProductName       string        `json:"productName"`
	ProductVersion    string        `json:"productVersion"`
	Networks          model.RefList `json:"networks"`
	Datastores        model.RefList `json:"datastores"`
}

//
// Build the resource using the model.
func (r *Host) With(m *model.Host) {
	r.Resource.With(&m.Base)
	r.InMaintenanceMode = m.InMaintenanceMode
	r.ProductVersion = m.ProductVersion
	r.ProductName = m.ProductName
	r.Networks = *model.RefListPtr().With(m.Networks)
	r.Datastores = *model.RefListPtr().With(m.Datastores)
}

//
// Render.
func (r *Host) Object(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
