package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"net/http"
)

const (
	FoldersRoot = Root + "/folders"
	FolderRoot  = FoldersRoot + "/:folder"
)

//
// Folder handler.
type FolderHandler struct {
	Base
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
	return h.Base.Prepare(ctx)
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
	content := []*Folder{}
	for _, m := range list {
		r := &Folder{}
		r.With(&m, false)
		content = append(content, r)
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
	m := &model.Folder{
		Base: model.Base{
			ID: ctx.Param("folder"),
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
	r := &Folder{}
	r.With(m, true)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Folder struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Object model.Object `json:"object,omitempty"`
}

//
// Build the resource using the model.
func (r *Folder) With(m *model.Folder, detail bool) {
	r.ID = m.ID
	r.Name = m.Name
	if detail {
		r.Object = m.DecodeObject()
	}
}
