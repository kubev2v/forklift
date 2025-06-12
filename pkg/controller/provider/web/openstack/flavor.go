package openstack

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	FlavorParam      = "flavor"
	FlavorCollection = "flavors"
	FlavorsRoot      = ProviderRoot + "/" + FlavorCollection
	FlavorRoot       = FlavorsRoot + "/:" + FlavorParam
)

// Flavor handler.
type FlavorHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *FlavorHandler) AddRoutes(e *gin.Engine) {
	e.GET(FlavorsRoot, h.List)
	e.GET(FlavorsRoot+"/", h.List)
	e.GET(FlavorRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h FlavorHandler) List(ctx *gin.Context) {
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
	list := []model.Flavor{}
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
		r := &Flavor{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h FlavorHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Flavor{
		Base: model.Base{
			ID: ctx.Param(FlavorParam),
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
	r := &Flavor{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *FlavorHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Flavor{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Flavor)
			network := &Flavor{}
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
func (h *FlavorHandler) filter(ctx *gin.Context, list *[]model.Flavor) (err error) {
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
	kept := []model.Flavor{}
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
type Flavor struct {
	Resource
	Description string            `json:"description"`
	Disk        int               `json:"disk"`
	RAM         int               `json:"ram"`
	RxTxFactor  float64           `json:"rxtxFactor"`
	Swap        int               `json:"swap"`
	VCPUs       int               `json:"vcpus"`
	IsPublic    bool              `json:"isPublic"`
	Ephemeral   int               `json:"ephemeral"`
	ExtraSpecs  map[string]string `json:"extraSpecs,omitempty"`
}

// Build the resource using the model.
func (r *Flavor) With(m *model.Flavor) {
	r.Resource.With(&m.Base)
	r.Description = m.Description
	r.Disk = m.Disk
	r.RAM = m.RAM
	r.RxTxFactor, _ = strconv.ParseFloat(m.RxTxFactor, 64)
	r.Swap = m.Swap
	r.VCPUs = m.VCPUs
	r.IsPublic = m.IsPublic
	r.Ephemeral = m.Ephemeral
	r.ExtraSpecs = m.ExtraSpecs
}

// Build self link (URI).
func (r *Flavor) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		FlavorRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			FlavorParam:        r.ID,
		})
}

// As content.
func (r *Flavor) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
