package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
	"strings"
)

//
// Routes.
const (
	NetworkParam      = "network"
	NetworkCollection = "networks"
	NetworksRoot      = ProviderRoot + "/" + NetworkCollection
	NetworkRoot       = NetworksRoot + "/:" + NetworkParam
)

//
// Network handler.
type NetworkHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *NetworkHandler) AddRoutes(e *gin.Engine) {
	e.GET(NetworksRoot, h.List)
	e.GET(NetworksRoot+"/", h.List)
	e.GET(NetworkRoot, h.Get)
}

//
// List resources in a REST collection.
func (h NetworkHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Network{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	err = h.filter(ctx, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Network{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NetworkHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Network{
		Base: model.Base{
			ID: ctx.Param(NetworkParam),
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
	r := &Network{}
	r.With(m)
	r.Path, err = m.Path(db)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h NetworkHandler) Link(p *api.Provider, m *model.Network) string {
	return h.Handler.Link(
		NetworkRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			NetworkParam:       m.ID,
		})
}

//
// Filter result set.
// Filter by path for `name` query.
func (h NetworkHandler) filter(ctx *gin.Context, list *[]model.Network) (err error) {
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
	db := h.Reconciler.DB()
	kept := []model.Network{}
	for _, m := range *list {
		path, pErr := m.Path(db)
		if pErr != nil {
			err = pErr
			return
		}
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

//
// REST Resource.
type Network struct {
	Resource
	Variant  string          `json:"variant"`
	DVSwitch *model.Ref      `json:"dvSwitch,omitempty"`
	Host     []model.DVSHost `json:"host"`
	Tag      string          `json:"tag,omitempty"`
}

//
// Build the resource using the model.
func (r *Network) With(m *model.Network) {
	r.Resource.With(&m.Base)
	r.Variant = m.Variant
	switch m.Variant {
	case model.NetStandard:
		r.Tag = m.Tag
	case model.NetDvPortGroup:
		r.DVSwitch = &m.DVSwitch
	case model.NetDvSwitch:
		r.Host = m.Host
	}
}

//
// As content.
func (r *Network) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
