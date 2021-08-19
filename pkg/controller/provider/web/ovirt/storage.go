package ovirt

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
	"strings"
)

//
// Routes.
const (
	StorageDomainParam      = "storagedomain"
	StorageDomainCollection = "storagedomains"
	StorageDomainsRoot      = ProviderRoot + "/" + StorageDomainCollection
	StorageDomainRoot       = StorageDomainsRoot + "/:" + StorageDomainParam
)

//
// StorageDomain handler.
type StorageDomainHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *StorageDomainHandler) AddRoutes(e *gin.Engine) {
	e.GET(StorageDomainsRoot, h.List)
	e.GET(StorageDomainsRoot+"/", h.List)
	e.GET(StorageDomainRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h StorageDomainHandler) List(ctx *gin.Context) {
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
	list := []model.StorageDomain{}
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
	err = h.filter(ctx, &list)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range list {
		r := &StorageDomain{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h StorageDomainHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.StorageDomain{
		Base: model.Base{
			ID: ctx.Param(StorageDomainParam),
		},
	}
	db := h.Collector.DB()
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
	r := &StorageDomain{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Watch.
func (h StorageDomainHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.StorageDomain{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.StorageDomain)
			ds := &StorageDomain{}
			ds.With(m)
			ds.Link(h.Provider)
			r = ds
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
// Filter result set.
// Filter by path for `name` query.
func (h *StorageDomainHandler) filter(ctx *gin.Context, list *[]model.StorageDomain) (err error) {
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
	db := h.Collector.DB()
	kept := []model.StorageDomain{}
	for _, m := range *list {
		path, pErr := m.Path(db)
		if pErr != nil {
			err = pErr
			return
		}
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

//
// REST Resource.
type StorageDomain struct {
	Resource
	DataCenter string `json:"dataCenter"`
	Type       string `json:"type"`
	Capacity   int64  `json:"capacity"`
	Free       int64  `json:"free"`
	Storage    struct {
		Type string `json:"type"`
	} `json:"storage"`
}

//
// Build the resource using the model.
func (r *StorageDomain) With(m *model.StorageDomain) {
	r.Resource.With(&m.Base)
	r.DataCenter = m.DataCenter
	r.Type = m.Type
	r.Capacity = m.Available
	r.Free = m.Available - m.Used
	r.Storage.Type = m.Storage.Type
}

//
// Build self link (URI).
func (r *StorageDomain) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		StorageDomainRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			StorageDomainParam: r.ID,
		})
}

//
// As content.
func (r *StorageDomain) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
