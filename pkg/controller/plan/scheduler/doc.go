package scheduler

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/ocp"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/openstack"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/ova"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler/vsphere"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	ec2scheduler "github.com/kubev2v/forklift/pkg/provider/ec2/controller/scheduler"
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
	case api.Ova, api.HyperV:
		scheduler = &ova.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	case api.EC2:
		scheduler = &ec2scheduler.Scheduler{
			Context:     ctx,
			MaxInFlight: settings.Settings.MaxInFlight,
		}
	default:
		err = liberr.New("provider not supported.")
	}

	return
}
