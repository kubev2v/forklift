package nutanix

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
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

// Add routes.
func (h *ImageHandler) AddRoutes(e *gin.Engine) {
	e.GET(ImagesRoot, h.List)
	e.GET(ImagesRoot+"/", h.List)
	e.GET(ImageRoot, h.Get)
}

// List resources.
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
	list := []model.Image{}
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
		r := &Image{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
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
	pb := PathBuilder{DB: db}
	r := &Image{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

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
			pb := PathBuilder{DB: db}
			m := in.(*model.Image)
			image := &Image{}
			image.With(m)
			image.Link(h.Provider)
			image.Path = pb.Path(m)
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

// Filter result set.
func (h *ImageHandler) filter(ctx *gin.Context, list *[]model.Image) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	pb := PathBuilder{DB: h.Collector.DB()}
	kept := []model.Image{}
	for _, m := range *list {
		path := pb.Path(&m)
		if path == name || strings.HasSuffix(path, "/"+name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// REST resource.
type Image struct {
	Resource
	ImageUUID    string `json:"imageUuid"`
	ImageType    string `json:"imageType"`
	SizeBytes    int64  `json:"sizeBytes"`
	Architecture string `json:"architecture"`
	SourceURI    string `json:"sourceUri"`
}

// Build the resource using the model.
func (r *Image) With(m *model.Image) {
	r.Resource.With(&m.Base)
	r.ImageUUID = m.ImageUUID
	r.ImageType = m.ImageType
	r.SizeBytes = m.SizeBytes
	r.Architecture = m.Architecture
	r.SourceURI = m.SourceURI
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
	return r
}
