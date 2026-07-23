package web

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

type Resource struct {
	ID            string `json:"id"`
	Revision      int64  `json:"revision"`
	Path          string `json:"path,omitempty"`
	Name          string `json:"name"`
	ResourceGroup string `json:"resourceGroup"`
	SelfLink      string `json:"selfLink"`
}

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

type VM struct {
	Resource
	Object *model.VMDetails `json:"object,omitempty"`
}

func (r *VM) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}

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

type Network struct {
	Resource
	Object *model.NetworkDetails `json:"object,omitempty"`
}

func (r *Network) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/networks/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

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
