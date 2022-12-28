package openstack

import libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"

// Routes.
const (
	WorkloadCollection = "workloads"
	WorkloadsRoot      = ProviderRoot + "/" + WorkloadCollection
	WorkloadRoot       = WorkloadsRoot + "/:" + VMParam
)

// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	XVM
}

// Expanded: VM.
type XVM struct {
	VM
}

// Expand references.
func (r *XVM) Expand(db libmodel.DB) (err error) {
	return nil
}
