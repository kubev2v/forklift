package validator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
)

func (r *Validator) MigrationType() bool {
	if r.Context.Plan.Spec.Type == "" || r.Context.Plan.Spec.Type == api.MigrationCold {
		return true
	}
	return false
}
