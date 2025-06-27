package vsphere

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	DatastoreParam      = "datastore"
	DatastoreCollection = "datastores"
	DatastoresRoot      = ProviderRoot + "/" + DatastoreCollection
	DatastoreRoot       = DatastoresRoot + "/:" + DatastoreParam
)

// Datastore handler.
type DatastoreHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *DatastoreHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatastoresRoot, h.List)
	e.GET(DatastoresRoot+"/", h.List)
	e.GET(DatastoreRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DatastoreHandler) List(ctx *gin.Context) {
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
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	db := h.Collector.DB()
	list := []model.Datastore{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		return
	}
	content := []interface{}{}
	err = h.filter(ctx, &list)
	if err != nil {
		return
	}
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &Datastore{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DatastoreHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Datastore{
		Base: model.Base{
			ID: ctx.Param(DatastoreParam),
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
	r := &Datastore{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *DatastoreHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Datastore{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Datastore)
			ds := &Datastore{}
			ds.With(m)
			ds.Link(h.Provider)
			ds.Path = pb.Path(m)
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

// Filter result set.
// Filter by path for `name` query.
func (h *DatastoreHandler) filter(ctx *gin.Context, list *[]model.Datastore) (err error) {
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
	pb := PathBuilder{DB: db}
	kept := []model.Datastore{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// REST Resource.
type Datastore struct {
	Resource
	Type                string   `json:"type"`
	Capacity            int64    `json:"capacity"`
	Free                int64    `json:"free"`
	MaintenanceMode     string   `json:"maintenance"`
	BackingDevicesNames []string `json:"backingDevicesNames"`
}

// Build the resource using the model.
func (r *Datastore) With(m *model.Datastore) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Capacity = m.Capacity
	r.Free = m.Free
	r.MaintenanceMode = m.MaintenanceMode
	r.BackingDevicesNames = m.BackingDevicesNames
}

// Build self link (URI).
func (r *Datastore) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DatastoreRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DatastoreParam:     r.ID,
		})
}

// As content.
func (r *Datastore) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
