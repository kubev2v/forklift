package ocp

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	storage "k8s.io/api/storage/v1"
	"net/http"
)

//
// Routes.
const (
	StorageClassParam  = "sc"
	StorageClassesRoot = ProviderRoot + "/storageclasses"
	StorageClassRoot   = StorageClassesRoot + "/:" + StorageClassParam
)

//
// StorageClass handler.
type StorageClassHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *StorageClassHandler) AddRoutes(e *gin.Engine) {
	e.GET(StorageClassesRoot, h.List)
	e.GET(StorageClassesRoot+"/", h.List)
	e.GET(StorageClassRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h StorageClassHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Reconciler.DB()
	list := []model.StorageClass{}
	err := db.List(&list, h.ListOptions(ctx))
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
		r := &StorageClass{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h StorageClassHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.StorageClass{
		Base: model.Base{
			PK: ctx.Param(StorageClassParam),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
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
	r := &StorageClass{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h StorageClassHandler) Link(p *api.Provider, m *model.StorageClass) string {
	return h.Handler.Link(
		StorageClassRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			StorageClassParam:  m.PK,
		})
}

//
// Watch.
func (h StorageClassHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.StorageClass{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.StorageClass)
			sc := &StorageClass{}
			sc.With(m)
			sc.SelfLink = h.Link(h.Provider, m)
			r = sc
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

//
// REST Resource.
type StorageClass struct {
	Resource
	Object storage.StorageClass `json:"object"`
}

//
// Set fields with the specified object.
func (r *StorageClass) With(m *model.StorageClass) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

//
// As content.
func (r *StorageClass) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
