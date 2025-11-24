package scheduler

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/dynamic"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/ocp"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/openstack"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/ova"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/vsphere"
	dynamicregistry "github.com/kubev2v/forklift/pkg/controller/provider/web/dynamic"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
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
		// Check if this is a registered dynamic provider
		if dynamicregistry.Registry.IsDynamic(string(ctx.Plan.Provider.Source.Type())) {
			scheduler = &dynamic.Scheduler{
				Context:     ctx,
				MaxInFlight: settings.Settings.MaxInFlight,
			}
		} else {
			err = liberr.New("provider not supported.")
		}
	}

	return
}
