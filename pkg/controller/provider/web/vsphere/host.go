package vsphere

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"

	"github.com/vmware/govmomi/vim25/mo"
)

// Routes
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

// Add routes to the `gin` router.
func (h *HostHandler) AddRoutes(e *gin.Engine) {
	e.GET(HostsRoot, h.List)
	e.GET(HostsRoot+"/", h.List)
	e.GET(HostRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
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
	err = h.filter(ctx, &list)
	if err != nil {
		return
	}
	content := []interface{}{}
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &Host{}
		r.With(&m)
		err = h.buildAdapters(r)
		if err != nil {
			return
		}
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
	h.Detail = model.MaxDetail
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
	err = h.buildAdapters(r)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// Parse advancedOption query parameter
	h.parseAdvancedOptions(ctx, r, m)

	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(h.Detail)

	ctx.JSON(http.StatusOK, content)
}

func (h *HostHandler) parseAdvancedOptions(ctx *gin.Context, r *Host, m *model.Host) {
	// Check the advanced option is passed
	advancedOption := ctx.Query("advancedOption")
	if advancedOption == "" {
		return
	}

	// Get settings of option manager
	optManagers := mo.OptionManager{}
	moRef := types.ManagedObjectReference{Type: m.AdvancedOptions.Kind, Value: m.AdvancedOptions.ID}
	if err := h.Collector.Follow(moRef, []string{"setting"}, &optManagers); err != nil {
		return
	}

	// Find the option we are interested in:
	for _, option := range optManagers.Setting {
		if option.GetOptionValue().Key == advancedOption {
			r.AdvancedOptions = []AdvancedOptions{{Key: advancedOption, Value: fmt.Sprintf("%v", option.GetOptionValue().Value)}}
		}
	}
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
// Filter by path for `name` query.
func (h *HostHandler) filter(ctx *gin.Context, list *[]model.Host) (err error) {
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
	kept := []model.Host{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// Build the network adapters.
func (h *HostHandler) buildAdapters(host *Host) (err error) {
	if h.Detail == 0 {
		return
	}
	builder := AdapterBuilder{
		db: h.Collector.DB(),
	}

	err = builder.build(host)

	return
}

// REST Resource.
type Host struct {
	Resource
	Cluster            string               `json:"cluster"`
	Status             string               `json:"status"`
	InMaintenanceMode  bool                 `json:"inMaintenance"`
	ManagementServerIp string               `json:"managementServerIp"`
	Thumbprint         string               `json:"thumbprint"`
	Timezone           string               `json:"timezone"`
	CpuSockets         int16                `json:"cpuSockets"`
	CpuCores           int16                `json:"cpuCores"`
	ProductName        string               `json:"productName"`
	ProductVersion     string               `json:"productVersion"`
	Network            model.HostNetwork    `json:"networking"`
	Networks           []model.Ref          `json:"networks"`
	Datastores         []model.Ref          `json:"datastores"`
	VMs                []model.Ref          `json:"vms"`
	NetworkAdapters    []NetworkAdapter     `json:"networkAdapters"`
	HostScsiDisks      []model.HostScsiDisk `json:"hostScsiDisks"`
	AdvancedOptions    []AdvancedOptions    `json:"advancedOptions"`
}

type AdvancedOptions struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Build the resource using the model.
func (r *Host) With(m *model.Host) {
	r.Resource.With(&m.Base)
	r.Cluster = m.Cluster
	r.Status = m.Status
	r.InMaintenanceMode = m.InMaintenanceMode
	r.ManagementServerIp = m.ManagementServerIp
	r.Thumbprint = m.Thumbprint
	r.Timezone = m.Timezone
	r.CpuSockets = m.CpuSockets
	r.CpuCores = m.CpuCores
	r.ProductVersion = m.ProductVersion
	r.ProductName = m.ProductName
	r.Network = m.Network
	r.Networks = m.Networks
	r.Datastores = m.Datastores
	r.NetworkAdapters = []NetworkAdapter{}
	r.HostScsiDisks = append(r.HostScsiDisks, m.HostScsiDisks...)
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
	if detail == 0 {
		return r.Resource
	}

	return r
}

// Host network adapter.
type NetworkAdapter struct {
	Name       string `json:"name"`
	IpAddress  string `json:"ipAddress"`
	SubnetMask string `json:"subnetMask"`
	LinkSpeed  int32  `json:"linkSpeed"`
	MTU        int32  `json:"mtu"`
}

// Build (and set) adapter list in the host.
type AdapterBuilder struct {
	db libmodel.DB
}

// Build the network adapters.
// Encapsulates the complexity of vSphere host network.
func (r *AdapterBuilder) build(host *Host) (err error) {
	list := []NetworkAdapter{}
	networking := host.Network
	for _, vNIC := range networking.VNICs {
		adapter := NetworkAdapter{
			IpAddress:  vNIC.IpAddress,
			SubnetMask: vNIC.SubnetMask,
			MTU:        vNIC.MTU,
		}
		if vNIC.PortGroup != "" {
			r.withPG(host, &vNIC, &adapter)
			list = append(list, adapter)
			continue
		}
		if vNIC.DPortGroup != "" {
			err = r.withDPG(host, &vNIC, &adapter)
			if err != nil {
				return
			}
			list = append(list, adapter)
			continue
		}
		list = append(list, adapter)
	}
	sort.Slice(
		list,
		func(i, j int) bool {
			if list[i].LinkSpeed != list[j].LinkSpeed {
				return list[i].LinkSpeed > list[j].LinkSpeed
			} else {
				return list[i].MTU > list[j].MTU
			}
		})

	host.NetworkAdapters = list

	return
}

// Build with PortGroup.
func (r AdapterBuilder) withPG(host *Host, vNIC *model.VNIC, adapter *NetworkAdapter) {
	net := host.Network
	portGroup, found := net.PortGroup(vNIC.PortGroup)
	if !found {
		return
	}
	adapter.Name = portGroup.Name
	vSwitch, found := net.Switch(portGroup.Switch)
	if !found {
		return
	}
	for _, key := range vSwitch.PNICs {
		if pNIC, found := net.PNIC(key); found {
			adapter.LinkSpeed = pNIC.LinkSpeed
			break
		}
	}
}

// Build with distributed virtual Switch & PortGroup.
func (r AdapterBuilder) withDPG(host *Host, vNIC *model.VNIC, adapter *NetworkAdapter) (err error) {
	portGroup := &model.Network{
		Base: model.Base{
			ID: vNIC.DPortGroup,
		},
	}
	err = r.db.Get(portGroup)
	if err != nil {
		if errors.Is(err, model.NotFound) {
			err = nil
		}
		return
	}
	ref := portGroup.DVSwitch
	vSwitch := &model.Network{
		Base: model.Base{
			ID: ref.ID,
		},
	}
	err = r.db.Get(vSwitch)
	if err != nil {
		if errors.Is(err, model.NotFound) {
			err = nil
		}
		return
	}
	adapter.Name = vSwitch.Name
	for _, dvsHost := range vSwitch.Host {
		hostRef := dvsHost.Host
		if hostRef.ID != host.ID {
			continue
		}
		host := &model.Host{
			Base: model.Base{
				ID: hostRef.ID,
			},
		}
		err = r.db.Get(host)
		if err != nil {
			if errors.Is(err, model.NotFound) {
				err = nil
				continue
			} else {
				return
			}
		}
		network := host.Network
		for _, pnic := range network.PNICs {
			adapter.LinkSpeed = pnic.LinkSpeed
			return
		}
	}

	return
}
