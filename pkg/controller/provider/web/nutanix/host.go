package nutanix

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	HostParam      = "host"
	HostCollection = "hosts"
	HostsRoot      = ProviderRoot + "/" + HostCollection
	HostRoot       = HostsRoot + "/:" + HostParam
)

// Host handler.
type HostHandler struct {
	Handler
}

// Add routes.
func (h *HostHandler) AddRoutes(e *gin.Engine) {
	e.GET(HostsRoot, h.List)
	e.GET(HostsRoot+"/", h.List)
	e.GET(HostRoot, h.Get)
}

// List resources.
func (h HostHandler) List(ctx *gin.Context) {
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
	list := []model.Host{}
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
		r := &Host{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h HostHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Host{
		Base: model.Base{
			ID: ctx.Param(HostParam),
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
	r := &Host{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *HostHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Host{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Host)
			host := &Host{}
			host.With(m)
			host.Link(h.Provider)
			host.Path = pb.Path(m)
			r = host
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
func (h *HostHandler) filter(ctx *gin.Context, list *[]model.Host) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	kept := []model.Host{}
	for _, m := range *list {
		path := pb.Path(&m)
		if path == name || strings.HasSuffix(path, "/"+name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// REST resource.
type Host struct {
	Resource
	HostUUID          string `json:"hostUuid"`
	Cluster           string `json:"cluster"`
	SerialNumber      string `json:"serialNumber"`
	BlockModel        string `json:"blockModel"`
	HypervisorType    string `json:"hypervisorType"`
	NumVMs            int    `json:"numVms"`
	State             string `json:"state"`
	HostType          string `json:"hostType"`
	CPUModel          string `json:"cpuModel"`
	CPUCapacityHz     int64  `json:"cpuCapacityHz"`
	NumCpuSockets     int    `json:"numCpuSockets"`
	NumCpuCores       int    `json:"numCpuCores"`
	NumCpuThreads     int    `json:"numCpuThreads"`
	MemoryCapacityMiB int64  `json:"memoryCapacityMib"`
	IPMIAddress       string `json:"ipmiAddress"`
}

// Build the resource using the model.
func (r *Host) With(m *model.Host) {
	r.Resource.With(&m.Base)
	r.HostUUID = m.HostUUID
	r.Cluster = m.Cluster
	r.SerialNumber = m.SerialNumber
	r.BlockModel = m.BlockModel
	r.HypervisorType = m.HypervisorType
	r.NumVMs = m.NumVMs
	r.State = m.State
	r.HostType = m.HostType
	r.CPUModel = m.CPUModel
	r.CPUCapacityHz = m.CPUCapacityHz
	r.NumCpuSockets = m.NumCpuSockets
	r.NumCpuCores = m.NumCpuCores
	r.NumCpuThreads = m.NumCpuThreads
	r.MemoryCapacityMiB = m.MemoryCapacityMiB
	r.IPMIAddress = m.IPMIAddress
}

// Build self link (URI).
func (r *Host) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		HostRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			HostParam:          r.ID,
		})
}

// As content.
func (r *Host) Content(detail int) interface{} {
	return r
}
