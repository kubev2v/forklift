package vsphere

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	FolderParam      = "folder"
	FolderCollection = "folders"
	FoldersRoot      = ProviderRoot + "/" + FolderCollection
	FolderRoot       = FoldersRoot + "/:" + FolderParam
)

// Folder handler.
type FolderHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *FolderHandler) AddRoutes(e *gin.Engine) {
	e.GET(FoldersRoot, h.List)
	e.GET(FoldersRoot+"/", h.List)
	e.GET(FolderRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h FolderHandler) List(ctx *gin.Context) {
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
	list := []model.Folder{}
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
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &Folder{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h FolderHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Folder{
		Base: model.Base{
			ID: ctx.Param(FolderParam),
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
	r := &Folder{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *FolderHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Folder{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Folder)
			folder := &Folder{}
			folder.With(m)
			folder.Link(h.Provider)
			folder.Path = pb.Path(m)
			r = folder
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
type Folder struct {
	Resource
	Folder     string      `json:"folder"`
	Datacenter string      `json:"datacenter"`
	Children   []model.Ref `json:"children"`
}

// Build the resource using the model.
func (r *Folder) With(m *model.Folder) {
	r.Resource.With(&m.Base)
	r.Folder = m.Folder
	r.Datacenter = m.Datacenter
	r.Children = m.Children
}

// Build self link (URI).
func (r *Folder) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		FolderRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			FolderParam:        r.ID,
		})
}

// Content.
func (r *Folder) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
