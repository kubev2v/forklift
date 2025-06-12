package ovirt

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	ServerCpuParam      = "servercpu"
	ServerCpuCollection = "servercpus"
	ServerCpusRoot      = ProviderRoot + "/" + ServerCpuCollection
	ServerCpuRoot       = ServerCpusRoot + "/:" + ServerCpuParam
)

// ServerCpu handler.
type ServerCpuHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ServerCpuHandler) AddRoutes(e *gin.Engine) {
	e.GET(ServerCpusRoot, h.List)
	e.GET(ServerCpusRoot+"/", h.List)
	e.GET(ServerCpuRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h ServerCpuHandler) List(ctx *gin.Context) {
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
	list := []model.ServerCpu{}
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
		r := &ServerCpu{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ServerCpuHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.ServerCpu{
		Base: model.Base{
			ID: ctx.Param(ServerCpuParam),
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
	r := &ServerCpu{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *ServerCpuHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.ServerCpu{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.ServerCpu)
			serverCpu := &ServerCpu{}
			serverCpu.With(m)
			serverCpu.Link(h.Provider)
			serverCpu.Path = pb.Path(m)
			r = serverCpu
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
func (h *ServerCpuHandler) filter(ctx *gin.Context, list *[]model.ServerCpu) (err error) {
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
	kept := []model.ServerCpu{}
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
type ServerCpu struct {
	Resource
	SystemOptionValue []SystemOptionValue `json:"systemOptionValue"`
}

type SystemOptionValue = model.SystemOptionValue

// Build the resource using the model.
func (r *ServerCpu) With(m *model.ServerCpu) {
	r.Resource.With(&m.Base)
	r.SystemOptionValue = []SystemOptionValue{m.SystemOptionValue}
}

// Build self link (URI).
func (r *ServerCpu) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ServerCpuRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ServerCpuParam:     r.ID,
		})
}

// As content.
func (r *ServerCpu) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
