package ocp

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
)

// Validator
type Validator struct {
	plan      *api.Plan
	inventory web.Client
}

// MaintenanceMode implements base.Validator
func (*Validator) MaintenanceMode(vmRef ref.Ref) (bool, error) {
	return false, nil
}

// PodNetwork implements base.Validator
func (*Validator) PodNetwork(vmRef ref.Ref) (bool, error) {
	return false, nil
}

// WarmMigration implements base.Validator
func (*Validator) WarmMigration() bool {
	return false
}

// Load.
func (r *Validator) Load() (err error) {
	r.inventory, err = web.NewClient(r.plan.Referenced.Provider.Source)
	return
}

func (r *Validator) StorageMapped(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}
