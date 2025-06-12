package openstack

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	VMParam      = "vm"
	VMCollection = "vms"
	VMsRoot      = ProviderRoot + "/" + VMCollection
	VMRoot       = VMsRoot + "/:" + VMParam
)

// Virtual Machine handler.
type VMHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h VMHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		ctx.Status(status)
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
	list := []model.VM{}
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
		r := &VM{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h VMHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if err != nil {
		return
	}
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param(VMParam),
		},
	}
	h.Detail = model.MaxDetail
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
	r := &VM{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(h.Detail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *VMHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VM{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.VM)
			vm := &VM{}
			vm.With(m)
			vm.Link(h.Provider)
			vm.Path = pb.Path(m)
			r = vm
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
func (h *VMHandler) filter(ctx *gin.Context, list *[]model.VM) (err error) {
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
	kept := []model.VM{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// VM detail=0
type VM0 = Resource

// VM detail=1
type VM1 struct {
	VM0
	TenantID          string                 `json:"tenantID"`
	Status            string                 `json:"status"`
	HostID            string                 `json:"hostID,omitempty"`
	RevisionValidated int64                  `json:"revisionValidated"`
	ImageID           string                 `json:"imageID,omitempty"`
	FlavorID          string                 `json:"flavorID"`
	Addresses         map[string]interface{} `json:"addresses"`
	AttachedVolumes   []AttachedVolume       `json:"attachedVolumes,omitempty"`
	Concerns          []Concern              `json:"concerns"`
}

// Build the resource using the model.
func (r *VM1) With(m *model.VM) {
	r.VM0.With(&m.Base)
	r.TenantID = m.TenantID
	r.Status = m.Status
	r.HostID = m.HostID
	r.RevisionValidated = m.RevisionValidated
	r.ImageID = m.ImageID
	r.FlavorID = m.FlavorID
	r.Addresses = m.Addresses
	r.AttachedVolumes = m.AttachedVolumes
	r.Concerns = m.Concerns
}

// As content.
func (r *VM1) Content(detail int) interface{} {
	if detail < 1 {
		return &r.VM0
	}

	return r
}

// VM resource.
type VM struct {
	VM1
	UserID         string                   `json:"userID"`
	Updated        time.Time                `json:"updated"`
	Created        time.Time                `json:"created"`
	Progress       int                      `json:"progress"`
	AccessIPv4     string                   `json:"accessIPv4,omitempty"`
	AccessIPv6     string                   `json:"accessIPv6,omitempty"`
	Metadata       map[string]string        `json:"metadata,omitempty"`
	KeyName        string                   `json:"keyName,omitempty"`
	AdminPass      string                   `json:"adminPass,omitempty"`
	SecurityGroups []map[string]interface{} `json:"securityGroups,omitempty"`
	Fault          Fault                    `json:"fault"`
	Tags           *[]string                `json:"tags,omitempty"`
	ServerGroups   *[]string                `json:"serverGroups,omitempty"`
}

type AttachedVolume = model.AttachedVolume
type Concern = model.Concern
type Fault = model.Fault

// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.VM1.With(m)
	r.UserID = m.UserID
	r.Updated = m.Updated
	r.Created = m.Created
	r.Progress = m.Progress
	r.AccessIPv4 = m.AccessIPv4
	r.AccessIPv6 = m.AccessIPv6
	r.Metadata = m.Metadata
	r.KeyName = m.KeyName
	r.AdminPass = m.AdminPass
	r.SecurityGroups = m.SecurityGroups
	r.Fault = m.Fault
	r.Tags = m.Tags
	r.ServerGroups = m.ServerGroups
}

// Build self link (URI).
func (r *VM) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}

// As content.
func (r *VM) Content(detail int) interface{} {
	if detail < 2 {
		return r.VM1.Content(detail)
	}

	return r
}
