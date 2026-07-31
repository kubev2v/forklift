package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Routes
const (
	SnapshotsRoot = ProviderRoot + "/snapshots"
	SnapshotRoot  = SnapshotsRoot + "/:id"
)

// Snapshot handler
type SnapshotHandler struct {
	Handler
}

// Add routes
func (h *SnapshotHandler) AddRoutes(e *gin.Engine) {
	e.GET(SnapshotsRoot, h.List)
	e.GET(SnapshotRoot, h.Get)
}

// List snapshots
func (h *SnapshotHandler) List(ctx *gin.Context) {
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

	listOptions := h.ListOptionsWithLabels(ctx)

	db := h.Collector.DB()
	var list []model.Snapshot
	err = db.List(&list, listOptions)
	if err != nil {
		log.Error(err, "Failed to list snapshots")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var result []interface{}
	for _, snapshot := range list {
		r := &Snapshot{}
		r.ID = snapshot.ID
		r.Name = snapshot.Name
		r.Revision = snapshot.Revision
		r.Link(h.Provider)
		if details, err := snapshot.GetDetails(); err == nil {
			r.Object = details
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

// Get snapshot
func (h *SnapshotHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	snapshot := &model.Snapshot{}
	snapshot.ID = ctx.Param("id")

	db := h.Collector.DB()
	err = db.Get(snapshot)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Snapshot{}
	r.ID = snapshot.ID
	r.Name = snapshot.Name
	r.Revision = snapshot.Revision
	r.Link(h.Provider)
	details, err := snapshot.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

// Watch snapshots via WebSocket.
func (h *SnapshotHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Snapshot{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Snapshot)
			snapshot := &Snapshot{}
			snapshot.ID = m.ID
			snapshot.Name = m.Name
			snapshot.Revision = m.Revision
			snapshot.Link(h.Provider)
			if details, err := m.GetDetails(); err == nil {
				snapshot.Object = details
			}
			r = snapshot
			return
		})
	if err != nil {
		log.Error(err, "watch failed")
		ctx.Status(http.StatusInternalServerError)
	}
}
