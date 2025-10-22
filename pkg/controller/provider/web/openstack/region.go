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
	RegionParam      = "region"
	RegionCollection = "regions"
	RegionsRoot      = ProviderRoot + "/" + RegionCollection
	RegionRoot       = RegionsRoot + "/:" + RegionParam
)

// Region handler.
type RegionHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *RegionHandler) AddRoutes(e *gin.Engine) {
	e.GET(RegionsRoot, h.List)
	e.GET(RegionsRoot+"/", h.List)
	e.GET(RegionRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h RegionHandler) List(ctx *gin.Context) {
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
	list := []model.Region{}
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
		r := &Region{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h RegionHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Region{
		Base: model.Base{
			ID: ctx.Param(RegionParam),
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
	r := &Region{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *RegionHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Region{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Region)
			network := &Region{}
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
func (h *RegionHandler) filter(ctx *gin.Context, list *[]model.Region) (err error) {
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
	kept := []model.Region{}
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
type Region struct {
	Resource
	Description    string `json:"description"`
	ParentRegionID string `json:"parentRegionID,omitempty"`
}

// Build the resource using the model.
func (r *Region) With(m *model.Region) {
	r.Resource.With(&m.Base)
	r.Description = m.Description
	r.ParentRegionID = m.ParentRegionID
}

// Build self link (URI).
func (r *Region) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		RegionRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			RegionParam:        r.ID,
		})
}

// As content.
func (r *Region) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
