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
	VolumeParam      = "volume"
	VolumeCollection = "volumes"
	VolumesRoot      = ProviderRoot + "/" + VolumeCollection
	VolumeRoot       = VolumesRoot + "/:" + VolumeParam
)

// Volume handler.
type VolumeHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *VolumeHandler) AddRoutes(e *gin.Engine) {
	e.GET(VolumesRoot, h.List)
	e.GET(VolumesRoot+"/", h.List)
	e.GET(VolumeRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h VolumeHandler) List(ctx *gin.Context) {
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
	list := []model.Volume{}
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
		r := &Volume{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h VolumeHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Volume{
		Base: model.Base{
			ID: ctx.Param(VolumeParam),
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
	r := &Volume{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *VolumeHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Volume{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Volume)
			network := &Volume{}
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
func (h *VolumeHandler) filter(ctx *gin.Context, list *[]model.Volume) (err error) {
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
	kept := []model.Volume{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

type Attachment = model.Attachment

// REST Resource.
type Volume struct {
	Resource                              // Unique identifier for the volume.
	Status              string            `json:"status"`
	Size                int               `json:"size"`
	AvailabilityZone    string            `json:"availabilityZone"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	Attachments         []Attachment      `json:"attachments"`
	Description         string            `json:"description,omitempty"`
	VolumeType          string            `json:"volumeType"`
	SnapshotID          string            `json:"snapshotID,omitempty"`
	SourceVolID         string            `json:"sourceVolID,omitempty"`
	BackupID            *string           `json:"backupID,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	UserID              string            `json:"userID"`
	Bootable            string            `json:"bootable"`
	Encrypted           bool              `json:"encrypted"`
	ReplicationStatus   string            `json:"replicationStatus"`
	ConsistencyGroupID  string            `json:"consistencygroupID,omitempty"`
	Multiattach         bool              `json:"multiattach"`
	VolumeImageMetadata map[string]string `json:"volumeImageMetadata,omitempty"`
}

// Build the resource using the model.
func (r *Volume) With(m *model.Volume) {
	r.Resource.With(&m.Base)
	r.Status = m.Status
	r.Size = m.Size
	r.AvailabilityZone = m.AvailabilityZone
	r.CreatedAt = m.CreatedAt
	r.UpdatedAt = m.UpdatedAt
	r.Attachments = m.Attachments
	r.Description = m.Description
	r.VolumeType = m.VolumeType
	r.SnapshotID = m.SnapshotID
	r.SourceVolID = m.SourceVolID
	r.BackupID = m.BackupID
	r.Metadata = m.Metadata
	r.UserID = m.UserID
	r.Bootable = m.Bootable
	r.Encrypted = m.Encrypted
	r.ReplicationStatus = m.ReplicationStatus
	r.ConsistencyGroupID = m.ConsistencyGroupID
	r.Multiattach = m.Multiattach
	r.VolumeImageMetadata = m.VolumeImageMetadata
}

// Build self link (URI).
func (r *Volume) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VolumeRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VolumeParam:        r.ID,
		})
}

// As content.
func (r *Volume) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
