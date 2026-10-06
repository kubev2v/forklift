package vsphere

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	ResourcePoolParam      = "resourcepool"
	ResourcePoolCollection = "resourcepools"
	ResourcePoolsRoot      = ProviderRoot + "/" + ResourcePoolCollection
	ResourcePoolRoot       = ResourcePoolsRoot + "/:" + ResourcePoolParam
)

// ResourcePool handler.
type ResourcePoolHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ResourcePoolHandler) AddRoutes(e *gin.Engine) {
	e.GET(ResourcePoolsRoot, h.List)
	e.GET(ResourcePoolsRoot+"/", h.List)
	e.GET(ResourcePoolRoot, h.Get)
}

// List resources in a REST collection.
func (h ResourcePoolHandler) List(ctx *gin.Context) {
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
	list := []model.ResourcePool{}
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
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &ResourcePool{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ResourcePoolHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.ResourcePool{
		Base: model.Base{
			ID: ctx.Param(ResourcePoolParam),
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
	pb := PathBuilder{DB: db}
	r := &ResourcePool{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *ResourcePoolHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.ResourcePool{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.ResourcePool)
			pool := &ResourcePool{}
			pool.With(m)
			pool.Link(h.Provider)
			pool.Path = pb.Path(m)
			r = pool
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
type ResourcePool struct {
	Resource
}

// Build the resource using the model.
func (r *ResourcePool) With(m *model.ResourcePool) {
	r.Resource.With(&m.Base)
}

// Build self link (URI).
func (r *ResourcePool) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ResourcePoolRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ResourcePoolParam:  r.ID,
		})
}

// As content.
func (r *ResourcePool) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
