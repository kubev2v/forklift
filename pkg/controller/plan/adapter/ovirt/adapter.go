package ovirt

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
)

//
// oVirt adapter.
type Adapter struct{}

//
// Constructs a oVirt builder.
func (r *Adapter) Builder(ctx *plancontext.Context) (builder base.Builder, err error) {
	builder = &Builder{Context: ctx}
	return
}

//
// Constructs a oVirt validator.
func (r *Adapter) Validator(plan *api.Plan) (validator base.Validator, err error) {
	v := &Validator{plan: plan}
	err = v.Load()
	if err != nil {
		return
	}
	validator = v
	return
}
