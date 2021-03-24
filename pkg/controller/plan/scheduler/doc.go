package scheduler

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler/vsphere"
	"github.com/konveyor/forklift-controller/pkg/settings"
)

type Scheduler interface {
	Next() (*plan.VMStatus, bool, error)
}

//
// Scheduler factory.
func New(ctx *plancontext.Context) (scheduler Scheduler, err error) {
	switch ctx.Source.Provider.Type() {
	case api.VSphere:
		scheduler = &vsphere.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	default:
		liberr.New("provider not supported.")
	}

	return
}
