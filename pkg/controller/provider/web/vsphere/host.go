package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
	"sort"
	"strings"
)

//
// Routes
const (
	HostParam      = "host"
	HostCollection = "hosts"
	HostsRoot      = ProviderRoot + "/" + HostCollection
	HostRoot       = HostsRoot + "/:" + HostParam
)

//
// Host handler.
type HostHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *HostHandler) AddRoutes(e *gin.Engine) {
	e.GET(HostsRoot, h.List)
	e.GET(HostsRoot+"/", h.List)
	e.GET(HostRoot, h.Get)
}

//
// List resources in a REST collection.
func (h HostHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Host{}
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
		r := &Host{}
		r.With(&m)
		err = h.buildAdapters(r)
		if err != nil {
			Log.Trace(err)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h HostHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	h.Detail = true
	m := &model.Host{
		Base: model.Base{
			ID: ctx.Param(HostParam),
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
	r := &Host{}
	r.With(m)
	r.Path, err = m.Path(db)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	err = h.buildAdapters(r)
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
func (h HostHandler) Link(p *api.Provider, m *model.Host) string {
	return h.Handler.Link(
		HostRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			HostParam:          m.ID,
		})
}

//
// Filter result set.
// Filter by path for `name` query.
func (h HostHandler) filter(ctx *gin.Context, list *[]model.Host) (err error) {
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
	kept := []model.Host{}
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
// Build the network adapters.
func (h *HostHandler) buildAdapters(host *Host) (err error) {
	if !h.Detail {
		return
	}
	builder := AdapterBuilder{
		db: h.Reconciler.DB(),
	}

	err = builder.build(host)

	return
}

//
// REST Resource.
type Host struct {
	Resource
	InMaintenanceMode  bool              `json:"inMaintenance"`
	ManagementServerIp string            `json:"managementServerIp"`
	Thumbprint         string            `json:"thumbprint"`
	CpuSockets         int16             `json:"cpuSockets"`
	CpuCores           int16             `json:"cpuCores"`
	ProductName        string            `json:"productName"`
	ProductVersion     string            `json:"productVersion"`
	Network            model.HostNetwork `json:"networking"`
	Networks           []model.Ref       `json:"networks"`
	Datastores         []model.Ref       `json:"datastores"`
	VMs                []model.Ref       `json:"vms"`
	NetworkAdapters    []NetworkAdapter  `json:"networkAdapters"`
}

//
// Build the resource using the model.
func (r *Host) With(m *model.Host) {
	r.Resource.With(&m.Base)
	r.InMaintenanceMode = m.InMaintenanceMode
	r.ManagementServerIp = m.ManagementServerIp
	r.Thumbprint = m.Thumbprint
	r.CpuSockets = m.CpuSockets
	r.CpuCores = m.CpuCores
	r.ProductVersion = m.ProductVersion
	r.ProductName = m.ProductName
	r.Network = m.Network
	r.Networks = m.Networks
	r.Datastores = m.Datastores
	r.VMs = m.Vms
	r.NetworkAdapters = []NetworkAdapter{}
}

//
// As content.
func (r *Host) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}

//
// Host network adapter.
type NetworkAdapter struct {
	Name       string `json:"name"`
	IpAddress  string `json:"ipAddress"`
	SubnetMask string `json:"subnetMask"`
	LinkSpeed  int32  `json:"linkSpeed"`
	MTU        int32  `json:"mtu"`
}

//
// Build (and set) adapter list in the host.
type AdapterBuilder struct {
	db libmodel.DB
}

//
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

//
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

	return
}

//
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
