package ocp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
)

// Routes.
const (
	ClusterInstanceParam = "clusterinstancetype"
	ClusterInstancesRoot = ProviderRoot + "/clusterinstancetypes"
	ClusterInstanceRoot  = ClusterInstancesRoot + "/:" + ClusterInstanceParam
)

// ClusterInstanceType handler.
type ClusterInstanceHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ClusterInstanceHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClusterInstancesRoot, h.List)
	e.GET(ClusterInstancesRoot+"/", h.List)
	e.GET(ClusterInstanceRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h ClusterInstanceHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Collector.DB()
	list := []model.ClusterInstanceType{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &ClusterInstanceType{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ClusterInstanceHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.ClusterInstanceType{
		Base: model.Base{
			UID: ctx.Param(ClusterInstanceParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &ClusterInstanceType{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h ClusterInstanceHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.ClusterInstanceType{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.ClusterInstanceType)
			it := &ClusterInstanceType{}
			it.With(m)
			it.Link(h.Provider)
			r = it
			return
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

// REST Resource.
type ClusterInstanceType struct {
	Resource
	Object instancetype.VirtualMachineClusterInstancetype `json:"object"`
}

// Set fields with the specified object.
func (r *ClusterInstanceType) With(m *model.ClusterInstanceType) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *ClusterInstanceType) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ClusterInstanceRoot,
		base.Params{
			base.ProviderParam:  string(p.UID),
			ClusterInstanceRoot: r.UID,
		})
}

// As content.
func (r *ClusterInstanceType) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
