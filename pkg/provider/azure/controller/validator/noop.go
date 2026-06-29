package validator

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Validator) GuestToolsInstalled(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

func (r *Validator) MaintenanceMode(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

func (r *Validator) NICNetworkRefs(vmRef ref.Ref) ([]ref.Ref, error) {
	return nil, nil
}

func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) HasSnapshot(vmRef ref.Ref) (ok bool, msg string, category string, err error) {
	return true, "", "", nil
}

func (r *Validator) InvalidDiskSizes(vmRef ref.Ref) ([]string, error) {
	return nil, nil
}

func (r *Validator) MacConflicts(vmRef ref.Ref) ([]base.MacConflict, error) {
	return nil, nil
}

func (r *Validator) PowerState(vmRef ref.Ref) (ok bool, err error) {
	return true, nil
}

func (r *Validator) PVCNameTemplate(vmRef ref.Ref, pvcNameTemplate string) (ok bool, err error) {
	return true, nil
}

func (r *Validator) SharedDisks(vmRef ref.Ref, c client.Client) (ok bool, msg string, category string, err error) {
	ok = true
	return
}

func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) UdnStaticIPs(vmRef ref.Ref, c client.Client) (ok bool, err error) {
	return true, nil
}

func (r *Validator) VMMigrationType(vmRef ref.Ref) (bool, error) {
	return true, nil
}

func (r *Validator) WarmMigration() bool {
	return false
}

func (r *Validator) ConsolidationNeeded(vmRef ref.Ref) (bool, error) {
	return false, nil
}
