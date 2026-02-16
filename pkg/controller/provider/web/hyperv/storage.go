package hyperv

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	StorageParam      = "storage"
	StorageCollection = "storages"
	StoragesRoot      = ProviderRoot + "/" + StorageCollection
	StorageRoot       = StoragesRoot + "/:" + StorageParam
)

// Storage handler.
type StorageHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *StorageHandler) AddRoutes(e *gin.Engine) {
	e.GET(StoragesRoot, h.List)
	e.GET(StoragesRoot+"/", h.List)
	e.GET(StorageRoot, h.Get)
}

// List resources in a REST collection.
func (h StorageHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Collector.DB()
	list := []model.Storage{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	err = h.filter(ctx, &list)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Storage{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h StorageHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Storage{
		Base: model.Base{
			ID: ctx.Param(StorageParam),
		},
	}
	db := h.Collector.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &Storage{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *StorageHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Storage{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Storage)
			storage := &Storage{}
			storage.With(m)
			storage.Link(h.Provider)
			r = storage
			return
		})
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

// Filter result set.
func (h *StorageHandler) filter(ctx *gin.Context, list *[]model.Storage) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	if len(strings.Split(name, "/")) < 2 {
		return
	}
	kept := []model.Storage{}
	for _, m := range *list {
		if h.PathMatchRoot(m.Name, name) {
			kept = append(kept, m)
		}
	}

	*list = kept
	return
}

// REST Resource.
type Storage struct {
	Resource
	Type     string `json:"type"`
	Path     string `json:"path"`
	Capacity int64  `json:"capacity"`
	Free     int64  `json:"free"`
}

// Build the resource using the model.
func (r *Storage) With(m *model.Storage) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Path = m.Path
	r.Capacity = m.Capacity
	r.Free = m.Free
}

// Build self link (URI).
func (r *Storage) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		StorageRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			StorageParam:       r.ID,
		})
}

// As content.
func (r *Storage) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}
	return r
}
