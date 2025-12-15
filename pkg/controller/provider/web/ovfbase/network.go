package ovfbase

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	NetworkParam      = "network"
	NetworkCollection = "networks"
)

// Network handler.
type NetworkHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *NetworkHandler) AddRoutes(e *gin.Engine) {
	root := h.Config.ProviderRoot() + "/" + NetworkCollection
	e.GET(root, h.List)
	e.GET(root+"/", h.List)
	e.GET(root+"/:"+NetworkParam, h.Get)
}

// List resources in a REST collection.
// A GET on the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NetworkHandler) List(ctx *gin.Context) {
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
	list := []model.Network{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		return
	}
	err = h.filter(ctx, &list)
	if err != nil {
		return
	}
	pb := PathBuilder{DB: db}
	content := []interface{}{}
	for _, m := range list {
		r := &Network{}
		r.With(&m)
		r.Link(h.Provider, h.Config)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h NetworkHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Network{
		Base: model.Base{
			ID: ctx.Param(NetworkParam),
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
	r := &Network{}
	r.With(m)
	r.Link(h.Provider, h.Config)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *NetworkHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Network{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Network)
			network := &Network{}
			network.With(m)
			network.Link(h.Provider, h.Config)
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
func (h *NetworkHandler) filter(ctx *gin.Context, list *[]model.Network) (err error) {
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
	kept := []model.Network{}
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
type Network struct {
	Resource
	Description string
}

// Build the resource using the model.
func (r *Network) With(m *model.Network) {
	r.Resource.With(&m.Base)
	r.Variant = m.Variant
	r.Name = m.Name
	r.Description = m.Description
}

// Build self link (URI).
func (r *Network) Link(p *api.Provider, cfg Config) {
	r.SelfLink = base.Link(
		cfg.ProviderRoot()+"/"+NetworkCollection+"/:"+NetworkParam,
		base.Params{
			base.ProviderParam: string(p.UID),
			NetworkParam:       r.ID,
		})
}

// As content.
func (r *Network) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
