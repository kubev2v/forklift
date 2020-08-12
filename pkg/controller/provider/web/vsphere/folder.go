package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	FoldersRoot = Root + "/folders"
	FolderRoot  = FoldersRoot + "/:folder"
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
	id := ctx.Param("folder")
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
	selector := &model.Folder{}
	options := libmodel.ListOptions{
		Page: &h.Page,
	}
	list := []model.Folder{}
	err := db.List(selector, options, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Folder{}
		r.With(&m)
		obj := r.Object(h.Detail)
		content = append(content, obj)
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

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Folder struct {
	base.Resource
	Children model.RefList `json:"children"`
}

//
// Build the resource using the model.
func (r *Folder) With(m *model.Folder) {
	r.Resource.With(&m.Base)
	r.Children = *model.RefListPtr().With(m.Children)
}

//
// Render.
func (r *Folder) Object(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
