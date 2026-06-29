package web

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

// REST Resource base.
type Resource struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Path     string `json:"path,omitempty"`
	Name     string `json:"name"`
	SelfLink string `json:"selfLink"`
}

// Provider resource.
type Provider struct {
	Resource
	UID    string                 `json:"uid"`
	Object map[string]interface{} `json:"object,omitempty"`
}

func (r *Provider) With(p *api.Provider) {
	r.UID = string(p.UID)
	r.Name = p.Name
}

func (r *Provider) Link() {
	r.SelfLink = base.Link(
		ProviderRoot,
		base.Params{
			base.ProviderParam: r.UID,
		})
}

// VM Resource.
type VM struct {
	Resource
	PowerState string           `json:"powerState"`
	CpuCount   int32            `json:"cpuCount"`
	MemoryMB   int32            `json:"memoryMB"`
	GuestId    string           `json:"guestId"`
	Disks      []model.VMDisk   `json:"disks,omitempty"`
	Concerns   []interface{}    `json:"concerns"`
	Object     *model.VMDetails `json:"object,omitempty"`
}

func (r *VM) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}

func (r *VM) WithModel(m *model.VM) {
	r.ID = m.UID
	r.Name = m.Name
	r.Revision = m.Revision
	r.PowerState = m.PowerState
	r.CpuCount = m.CpuCount
	r.MemoryMB = m.MemoryMB
	r.GuestId = m.GuestId
	r.Disks = m.Disks
	r.Concerns = []interface{}{}
}

// Disk Resource.
type Disk struct {
	Resource
	Object *model.DiskDetails `json:"object,omitempty"`
}

func (r *Disk) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/disks/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

// Network Resource.
type Network struct {
	Resource
	Variant       string                `json:"variant"`
	AddressPrefix string                `json:"addressPrefix,omitempty"`
	Object        *model.NetworkDetails `json:"object,omitempty"`
}

func (r *Network) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/networks/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

func (r *Network) WithModel(m *model.Network) {
	r.ID = m.UID
	r.Name = m.Name
	r.Revision = m.Revision
	r.Variant = m.NetworkType
	r.AddressPrefix = m.AddressPrefix
}

// Storage Resource (disk types).
type Storage struct {
	Resource
	Object *model.StorageDetails `json:"object,omitempty"`
}

func (r *Storage) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/storages/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

// Workload Resource (same as VM for Azure).
type Workload struct {
	Resource
	Object *model.VMDetails `json:"object,omitempty"`
}

func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}
