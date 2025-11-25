package validator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

// MigrationType validates migration type. EC2 only supports cold migration (or empty/default).
// Warm/live migration not supported - EC2 requires instance shutdown for consistent EBS snapshots.
func (r *Validator) MigrationType() bool {
	if r.Context.Plan.Spec.Type == "" || r.Context.Plan.Spec.Type == api.MigrationCold {
		return true
	}
	return false
}
