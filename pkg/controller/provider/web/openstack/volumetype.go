package openstack

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	VolumeTypeParam      = "volumetype"
	VolumeTypeCollection = "volumetypes"
	VolumeTypesRoot      = ProviderRoot + "/" + VolumeTypeCollection
	VolumeTypeRoot       = VolumeTypesRoot + "/:" + VolumeTypeParam
)

// VolumeType handler.
type VolumeTypeHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *VolumeTypeHandler) AddRoutes(e *gin.Engine) {
	e.GET(VolumeTypesRoot, h.List)
	e.GET(VolumeTypesRoot+"/", h.List)
	e.GET(VolumeTypeRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h VolumeTypeHandler) List(ctx *gin.Context) {
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
	list := []model.VolumeType{}
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
		r := &VolumeType{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h VolumeTypeHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.VolumeType{
		Base: model.Base{
			ID: ctx.Param(VolumeTypeParam),
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
	r := &VolumeType{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *VolumeTypeHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VolumeType{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.VolumeType)
			network := &VolumeType{}
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
func (h *VolumeTypeHandler) filter(ctx *gin.Context, list *[]model.VolumeType) (err error) {
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
	kept := []model.VolumeType{}
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
type VolumeType struct {
	Resource
	Description  string            `json:"description"`
	ExtraSpecs   map[string]string `json:"extraSpecs,omitempty"`
	IsPublic     bool              `json:"isPublic"`
	QosSpecID    string            `json:"qosSpecsID"`
	PublicAccess bool              `json:"publicAccess"`
}

// Build the resource using the model.
func (r *VolumeType) With(m *model.VolumeType) {
	r.Resource.With(&m.Base)
	r.Description = m.Description
	r.ExtraSpecs = m.ExtraSpecs
	r.IsPublic = m.IsPublic
	r.QosSpecID = m.QosSpecID
	r.PublicAccess = m.PublicAccess
}

// Build self link (URI).
func (r *VolumeType) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VolumeTypeRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VolumeTypeParam:    r.ID,
		})
}

// As content.
func (r *VolumeType) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
