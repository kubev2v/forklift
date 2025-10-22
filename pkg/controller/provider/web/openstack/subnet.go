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
	SubnetParam      = "subnet"
	SubnetCollection = "subnets"
	SubnetsRoot      = ProviderRoot + "/" + SubnetCollection
	SubnetRoot       = SubnetsRoot + "/:" + SubnetParam
)

// Subnet handler.
type SubnetHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *SubnetHandler) AddRoutes(e *gin.Engine) {
	e.GET(SubnetsRoot, h.List)
	e.GET(SubnetsRoot+"/", h.List)
	e.GET(SubnetRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h SubnetHandler) List(ctx *gin.Context) {
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
	list := []model.Subnet{}
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
		r := &Subnet{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h SubnetHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Subnet{
		Base: model.Base{
			ID: ctx.Param(SubnetParam),
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
	r := &Subnet{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *SubnetHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Subnet{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Subnet)
			subnet := &Subnet{}
			subnet.With(m)
			subnet.Link(h.Provider)
			subnet.Path = pb.Path(m)
			r = subnet
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
func (h *SubnetHandler) filter(ctx *gin.Context, list *[]model.Subnet) (err error) {
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
	kept := []model.Subnet{}
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
type Subnet struct {
	Resource
	NetworkID       string           `json:"networkID"`
	Description     string           `json:"description,omitempty"`
	IPVersion       int              `json:"ipVersion"`
	CIDR            string           `json:"cidr"`
	GatewayIP       string           `json:"gatewayIP,omitempty"`
	DNSNameservers  []string         `json:"dnsNameservers,omitempty"`
	ServiceTypes    []string         `json:"serviceTypes,omitempty"`
	AllocationPools []AllocationPool `json:"allocationPools,omitempty"`
	HostRoutes      []HostRoute      `json:"hostRoutes,omitempty"`
	EnableDHCP      bool             `json:"enableDHCP"`
	TenantID        string           `json:"tenantID"`
	ProjectID       string           `json:"projectID"`
	IPv6AddressMode string           `json:"ipv6AddressMode,omitempty"`
	IPv6RAMode      string           `json:"ipv6RAMode,omitempty"`
	SubnetPoolID    string           `json:"subnetpoolID,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	RevisionNumber  int              `json:"revisionNumber"`
}

type AllocationPool = model.AllocationPool
type HostRoute = model.HostRoute

// Build the resource using the model.
func (r *Subnet) With(m *model.Subnet) {
	r.Resource.With(&m.Base)
	r.NetworkID = m.NetworkID
	r.Description = m.Description
	r.IPVersion = m.IPVersion
	r.CIDR = m.CIDR
	r.GatewayIP = m.GatewayIP
	r.DNSNameservers = m.DNSNameservers
	r.ServiceTypes = m.ServiceTypes
	r.AllocationPools = m.AllocationPools
	r.HostRoutes = m.HostRoutes
	r.EnableDHCP = m.EnableDHCP
	r.TenantID = m.TenantID
	r.ProjectID = m.ProjectID
	r.IPv6AddressMode = m.IPv6AddressMode
	r.IPv6RAMode = m.IPv6RAMode
	r.SubnetPoolID = m.SubnetPoolID
	r.Tags = m.Tags
	r.RevisionNumber = m.RevisionNumber
}

// Build self link (URI).
func (r *Subnet) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		SubnetRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			SubnetParam:        r.ID,
		})
}

// As content.
func (r *Subnet) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
