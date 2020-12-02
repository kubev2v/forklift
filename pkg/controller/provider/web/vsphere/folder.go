package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	FolderParam      = "folder"
	FolderCollection = "folders"
	FoldersRoot      = ProviderRoot + "/" + FolderCollection
	FolderRoot       = FoldersRoot + "/:" + FolderParam
)

//
// Folder handler.
type FolderHandler struct {
	base.Handler
	// Selected folder.
	folder *model.Folder
}

//
// Add routes to the `gin` router.
func (h *FolderHandler) AddRoutes(e *gin.Engine) {
	e.GET(FoldersRoot, h.List)
	e.GET(FoldersRoot+"/", h.List)
	e.GET(FolderRoot, h.Get)
}

//
// Prepare to handle the request.
func (h *FolderHandler) Prepare(ctx *gin.Context) int {
	status := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return status
	}
	id := ctx.Param(FolderParam)
	if id != "" {
		m := &model.Folder{
			Base: model.Base{
				ID: id,
			},
		}
		db := h.Reconciler.DB()
		err := db.Get(m)
		if errors.Is(err, model.NotFound) {
			return http.StatusNotFound
		}
		if err != nil {
			Log.Trace(err)
			return http.StatusInternalServerError
		}

		h.folder = m
	}

	return http.StatusOK
}

//
// List resources in a REST collection.
func (h FolderHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Folder{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Page: &h.Page,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Folder{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h FolderHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}

	r := &Folder{}
	r.With(h.folder)
	r.SelfLink = h.Link(h.Provider, h.folder)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h FolderHandler) Link(p *api.Provider, m *model.Folder) string {
	return h.Handler.Link(
		FolderRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			FolderParam:        m.ID,
		})
}

//
// REST Resource.
type Folder struct {
	Resource
	Children model.RefList `json:"children"`
}

//
// Build the resource using the model.
func (r *Folder) With(m *model.Folder) {
	r.Resource.With(&m.Base)
	r.Children = *model.RefListPtr().With(m.Children)
}

//
// Content.
func (r *Folder) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
