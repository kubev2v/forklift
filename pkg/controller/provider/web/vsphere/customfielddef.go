package vsphere

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
)

// Routes.
const (
	CustomFieldDefParam      = "customfielddef"
	CustomFieldDefCollection = "customfielddefs"
	CustomFieldDefsRoot      = ProviderRoot + "/" + CustomFieldDefCollection
	CustomFieldDefRoot       = CustomFieldDefsRoot + "/:" + CustomFieldDefParam
)

// Custom field definition handler.
type CustomFieldDefHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *CustomFieldDefHandler) AddRoutes(e *gin.Engine) {
	e.GET(CustomFieldDefsRoot, h.List)
	e.GET(CustomFieldDefsRoot+"/", h.List)
	e.GET(CustomFieldDefRoot, h.Get)
}

// List resources in a REST collection.
func (h *CustomFieldDefHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
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
	list := []model.CustomFieldDef{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &CustomFieldDef{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h *CustomFieldDefHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	key, err := strconv.ParseInt(ctx.Param(CustomFieldDefParam), 10, 32)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusBadRequest)
		return
	}
	m := &model.CustomFieldDef{
		Key: int32(key),
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
	r := &CustomFieldDef{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// REST Resource.
type CustomFieldDef struct {
	SelfLink          string `json:"selfLink"`
	ID                string `json:"id"`
	Name              string `json:"name"`
	Key               int32  `json:"key"`
	ManagedObjectType string `json:"managedObjectType,omitempty"`
}

// Build the resource using the model.
func (r *CustomFieldDef) With(m *model.CustomFieldDef) {
	r.ID = strconv.Itoa(int(m.Key))
	r.Name = m.Name
	r.Key = m.Key
	r.ManagedObjectType = m.ManagedObjectType
}

// Build self link (URI).
func (r *CustomFieldDef) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		CustomFieldDefRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			CustomFieldDefParam: r.ID,
		})
}

// As content.
func (r *CustomFieldDef) Content(detail int) interface{} {
	return r
}
