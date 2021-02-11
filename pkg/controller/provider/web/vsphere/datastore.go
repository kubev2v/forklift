package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
	"strings"
)

//
// Routes.
const (
	DatastoreParam      = "datastore"
	DatastoreCollection = "datastores"
	DatastoresRoot      = ProviderRoot + "/" + DatastoreCollection
	DatastoreRoot       = DatastoresRoot + "/:" + DatastoreParam
)

//
// Datastore handler.
type DatastoreHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *DatastoreHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatastoresRoot, h.List)
	e.GET(DatastoresRoot+"/", h.List)
	e.GET(DatastoreRoot, h.Get)
}

//
// List resources in a REST collection.
func (h DatastoreHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Datastore{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	err = h.filter(ctx, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range list {
		r := &Datastore{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h DatastoreHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Datastore{
		Base: model.Base{
			ID: ctx.Param(DatastoreParam),
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
	r := &Datastore{}
	r.With(m)
	r.Path, err = m.Path(db)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h DatastoreHandler) Link(p *api.Provider, m *model.Datastore) string {
	return h.Handler.Link(
		DatastoreRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			DatastoreParam:     m.ID,
		})
}

//
// Filter result set.
// Filter by path for `name` query.
func (h DatastoreHandler) filter(ctx *gin.Context, list *[]model.Datastore) (err error) {
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
	db := h.Reconciler.DB()
	kept := []model.Datastore{}
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
type Datastore struct {
	Resource
	Type            string `json:"type"`
	Capacity        int64  `json:"capacity"`
	Free            int64  `json:"free"`
	MaintenanceMode string `json:"maintenance"`
}

//
// Build the resource using the model.
func (r *Datastore) With(m *model.Datastore) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Capacity = m.Capacity
	r.Free = m.Free
	r.MaintenanceMode = m.MaintenanceMode
}

//
// As content.
func (r *Datastore) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
