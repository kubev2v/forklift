package openstack

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes
const (
	ImageParam      = "image"
	ImageCollection = "images"
	ImagesRoot      = ProviderRoot + "/" + ImageCollection
	ImageRoot       = ImagesRoot + "/:" + ImageParam
)

// Image handler.
type ImageHandler struct {
	Handler
}

type Image struct {
	Resource
	Status                      string                 `json:"status"`
	Tags                        []string               `json:"tags"`
	ContainerFormat             string                 `json:"containerFormat"`
	DiskFormat                  string                 `json:"diskFormat"`
	MinDiskGigabytes            int                    `json:"minDisk"`
	MinRAMMegabytes             int                    `json:"minRam"`
	Owner                       string                 `json:"owner"`
	Protected                   bool                   `json:"protected"`
	Visibility                  string                 `json:"visibility"`
	Hidden                      bool                   `json:"osHidden"`
	Checksum                    string                 `json:"checksum"`
	SizeBytes                   int64                  `json:"sizeBytes"`
	Metadata                    map[string]string      `json:"metadata"`
	CreatedAt                   time.Time              `json:"createdAt"`
	UpdatedAt                   time.Time              `json:"updatedAt"`
	File                        string                 `json:"file"`
	Schema                      string                 `json:"schema"`
	VirtualSize                 int64                  `json:"virtualSize"`
	OpenStackImageImportMethods []string               `json:"imageImportMethods"`
	OpenStackImageStoreIDs      []string               `json:"imageStoreIDs"`
	Properties                  map[string]interface{} `json:"properties"`
}

// Add routes to the `gin` router.
func (h *ImageHandler) AddRoutes(e *gin.Engine) {
	e.GET(ImagesRoot, h.List)
	e.GET(ImagesRoot+"/", h.List)
	e.GET(ImageRoot, h.Get)
}

// Build the resource using the model.
func (r *Image) With(m *model.Image) {
	r.Resource.ID = m.ID
	r.Resource.Revision = m.Revision
	r.Resource.Name = m.Name
	r.Status = string(m.Status)
	r.Tags = m.Tags
	r.ContainerFormat = m.ContainerFormat
	r.DiskFormat = m.DiskFormat
	r.MinDiskGigabytes = m.MinDiskGigabytes
	r.MinRAMMegabytes = m.MinRAMMegabytes
	r.Owner = m.Owner
	r.Protected = m.Protected
	r.Visibility = string(m.Visibility)
	r.Hidden = m.Hidden
	r.Checksum = m.Checksum
	r.SizeBytes = m.SizeBytes
	r.Metadata = m.Metadata
	r.Properties = m.Properties
	r.CreatedAt = m.CreatedAt
	r.UpdatedAt = m.UpdatedAt
	r.File = m.File
	r.Schema = m.Schema
	r.VirtualSize = m.VirtualSize
	r.OpenStackImageImportMethods = m.OpenStackImageImportMethods
	r.OpenStackImageStoreIDs = m.OpenStackImageStoreIDs
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h ImageHandler) List(ctx *gin.Context) {
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
	list := []model.Image{}
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
	for _, m := range list {
		r := &Image{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ImageHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	h.Detail = model.MaxDetail
	m := &model.Image{
		Base: model.Base{
			ID: ctx.Param(ImageParam),
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
	r := &Image{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(h.Detail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *ImageHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Image{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Image)
			image := &Image{}
			image.With(m)
			image.Link(h.Provider)
			r = image
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

// Build self link (URI).
func (r *Image) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ImageRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ImageParam:         r.ID,
		})
}

// As content.
func (r *Image) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
