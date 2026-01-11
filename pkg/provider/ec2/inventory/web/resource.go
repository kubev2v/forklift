package web

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// REST Resource base.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Revision
	Revision int64 `json:"revision"`
	// Path
	Path string `json:"path,omitempty"`
	// Object name.
	Name string `json:"name"`
	// Self link.
	SelfLink string `json:"selfLink"`
}

// Provider resource.
type Provider struct {
	Resource
	UID    string                 `json:"uid"`
	Object map[string]interface{} `json:"object,omitempty"`
}

// Build the resource.
func (r *Provider) With(p *api.Provider) {
	r.UID = string(p.UID)
	r.Name = p.Name
}

// Build self link (URI).
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
	Object *model.InstanceDetails `json:"object,omitempty"`
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

// Network Resource.
type Network struct {
	Resource
	Object *model.NetworkDetails `json:"object,omitempty"`
}

// Build self link (URI).
func (r *Network) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/networks/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

// Storage Resource (volume types).
type Storage struct {
	Resource
	Object *model.StorageDetails `json:"object,omitempty"`
}

// Build self link (URI).
func (r *Storage) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/storages/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

// Volume Resource.
type Volume struct {
	Resource
	Object *model.VolumeDetails `json:"object,omitempty"`
}

// Build self link (URI).
func (r *Volume) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ProviderRoot+"/volumes/:id",
		base.Params{
			base.ProviderParam: string(p.UID),
			"id":               r.ID,
		})
}

// Workload Resource (same as VM for EC2).
type Workload struct {
	Resource
	Object *model.InstanceDetails `json:"object,omitempty"`
}

// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}
