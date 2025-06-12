package openstack

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	SnapshotParam      = "snapshot"
	SnapshotCollection = "snapshots"
	SnapshotsRoot      = ProviderRoot + "/" + SnapshotCollection
	SnapshotRoot       = SnapshotsRoot + "/:" + SnapshotParam
)

// Snapshot handler.
type SnapshotHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *SnapshotHandler) AddRoutes(e *gin.Engine) {
	e.GET(SnapshotsRoot, h.List)
	e.GET(SnapshotsRoot+"/", h.List)
	e.GET(SnapshotRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h SnapshotHandler) List(ctx *gin.Context) {
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
	list := []model.Snapshot{}
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
		r := &Snapshot{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h SnapshotHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Snapshot{
		Base: model.Base{
			ID: ctx.Param(SnapshotParam),
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
	}
	pb := PathBuilder{DB: db}
	r := &Snapshot{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *SnapshotHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Snapshot{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Snapshot)
			network := &Snapshot{}
			network.With(m)
			network.Link(h.Provider)
			network.Path = pb.Path(m)
			r = network
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
func (h *SnapshotHandler) filter(ctx *gin.Context, list *[]model.Snapshot) (err error) {
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
	kept := []model.Snapshot{}
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
type Snapshot struct {
	Resource
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Description string            `json:"description"`
	VolumeID    string            `json:"volumeID"`
	Status      string            `json:"status"`
	Size        int               `json:"size"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Build the resource using the model.
func (r *Snapshot) With(m *model.Snapshot) {
	r.Resource.With(&m.Base)
	r.CreatedAt = m.CreatedAt
	r.UpdatedAt = m.UpdatedAt
	r.Description = m.Description
	r.VolumeID = m.VolumeID
	r.Status = m.Status
	r.Size = m.Size
	r.Metadata = m.Metadata
}

// Build self link (URI).
func (r *Snapshot) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		SnapshotRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			SnapshotParam:      r.ID,
		})
}

// As content.
func (r *Snapshot) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
