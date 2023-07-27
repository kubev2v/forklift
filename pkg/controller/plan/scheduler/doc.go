package scheduler

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler/vsphere"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/settings"
)

// Scheduler API
// Determines which of the plan's VMs can have their migrations started.
type Scheduler interface {
	// Return the next VM that can be migrated.
	Next() (vm *plan.VMStatus, hasNext bool, err error)
}

// Scheduler factory.
func New(ctx *plancontext.Context) (scheduler Scheduler, err error) {
	switch ctx.Source.Provider.Type() {
	case api.VSphere:
		scheduler = &vsphere.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	case api.OVirt:
		scheduler = &ovirt.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	case api.OpenStack:
		scheduler = &openstack.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	case api.OpenShift:
		scheduler = &ocp.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	case api.Ova:
		scheduler = &ova.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	default:
		liberr.New("provider not supported.")
	}

	return
}
