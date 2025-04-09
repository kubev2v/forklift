package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	storage "k8s.io/api/storage/v1"
)

// Routes.
const (
	StorageClassParam  = "sc"
	StorageClassesRoot = ProviderRoot + "/storageclasses"
	StorageClassRoot   = StorageClassesRoot + "/:" + StorageClassParam
)

// StorageClass handler.
type StorageClassHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *StorageClassHandler) AddRoutes(e *gin.Engine) {
	e.GET(StorageClassesRoot, h.List)
	e.GET(StorageClassesRoot+"/", h.List)
	e.GET(StorageClassRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h StorageClassHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusNotImplemented)
		return
	}
	storageclasses, err := h.StorageClasses(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range storageclasses {
		r := &StorageClass{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h StorageClassHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	storageclasses, err := h.StorageClasses(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, sc := range storageclasses {
		if sc.UID == ctx.Param(NsParam) {
			r := &StorageClass{}
			r.With(&sc)
			r.Link(h.Provider)
			content := r.Content(model.MaxDetail)
			ctx.JSON(http.StatusOK, content)
			return
		}
	}
	ctx.Status(http.StatusNotFound)
}

// Watch.
func (h StorageClassHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.StorageClass{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.StorageClass)
			sc := &StorageClass{}
			sc.With(m)
			sc.Link(h.Provider)
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

// REST Resource.
type StorageClass struct {
	Resource
	Object storage.StorageClass `json:"object"`
}

// Set fields with the specified object.
func (r *StorageClass) With(m *model.StorageClass) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *StorageClass) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		StorageClassRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			StorageClassParam:  r.UID,
		})
}

// As content.
func (r *StorageClass) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
