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
	ProjectParam      = "project"
	ProjectCollection = "projects"
	ProjectsRoot      = ProviderRoot + "/" + ProjectCollection
	ProjectRoot       = ProjectsRoot + "/:" + ProjectParam
)

// Project handler.
type ProjectHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ProjectHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProjectsRoot, h.List)
	e.GET(ProjectsRoot+"/", h.List)
	e.GET(ProjectRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h ProjectHandler) List(ctx *gin.Context) {
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
	list := []model.Project{}
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
		r := &Project{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ProjectHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Project{
		Base: model.Base{
			ID: ctx.Param(ProjectParam),
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
	r := &Project{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *ProjectHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Project{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Project)
			project := &Project{}
			project.With(m)
			project.Link(h.Provider)
			project.Path = pb.Path(m)
			r = project
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
func (h *ProjectHandler) filter(ctx *gin.Context, list *[]model.Project) (err error) {
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
	kept := []model.Project{}
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
type Project struct {
	Resource
	IsDomain    bool   `json:"isDomain"`
	Description string `json:"description"`
	DomainID    string `json:"domainID"`
	Enabled     bool   `json:"enabled"`
	ParentID    string `json:"parentID,omitempty"`
}

// Build the resource using the model.
func (r *Project) With(m *model.Project) {
	r.Resource.With(&m.Base)
	r.IsDomain = m.IsDomain
	r.Description = m.Description
	r.DomainID = m.DomainID
	r.Enabled = m.Enabled
	r.ParentID = m.ParentID
}

// Build self link (URI).
func (r *Project) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProjectRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ProjectParam:       r.ID,
		})
}

// As content.
func (r *Project) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
