package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ base.Validator = &Validator{}

type Validator struct {
	*plancontext.Context
}

func New(ctx *plancontext.Context) *Validator {
	return &Validator{Context: ctx}
}

func (r *Validator) StorageMapped(vmRef ref.Ref) (bool, error)       { return true, nil }
func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error)       { return true, nil }
func (r *Validator) NetworksMapped(vmRef ref.Ref) (bool, error)      { return true, nil }
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (bool, error)     { return true, nil }
func (r *Validator) WarmMigration() bool                             { return false }
func (r *Validator) MigrationType() bool                             { return true }
func (r *Validator) NICNetworkRefs(vmRef ref.Ref) ([]ref.Ref, error) { return nil, nil }
func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error)           { return true, nil }
func (r *Validator) UdnStaticIPs(vmRef ref.Ref, c client.Client) (bool, error) {
	return true, nil
}
func (r *Validator) SharedDisks(vmRef ref.Ref, c client.Client) (bool, string, string, error) {
	return true, "", "", nil
}
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) { return true, nil }
func (r *Validator) HasSnapshot(vmRef ref.Ref) (bool, string, string, error) {
	return true, "", "", nil
}
func (r *Validator) PowerState(vmRef ref.Ref) (bool, error)                 { return true, nil }
func (r *Validator) VMMigrationType(vmRef ref.Ref) (bool, error)            { return true, nil }
func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error)       { return nil, nil }
func (r *Validator) MacConflicts(vmRef ref.Ref) ([]base.MacConflict, error) { return nil, nil }
func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (bool, error) {
	return true, nil
}
func (r *Validator) GuestToolsInstalled(vmRef ref.Ref) (bool, error) { return true, nil }
func (r *Validator) ConsolidationNeeded(vmRef ref.Ref) (bool, error) { return false, nil }
