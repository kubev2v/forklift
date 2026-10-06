package copyappliance

import (
	"context"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	libvsphere "github.com/kubev2v/forklift/pkg/lib/vsphere"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TeardownRunner drives the appliance VM from running to gone. It holds no
// state of its own: every pass reads where it got to from the appliance status
// and leaves the next phase behind.
type TeardownRunner struct {
	context *ApplianceContext
	client  client.Client
}

// begin seeds the teardown pipeline. An appliance with no recorded VM has
// nothing to tear down and is already complete.
func (r *TeardownRunner) begin() (err error) {
	r.context.Appliance.Status.TaskRef = ""
	itinerary := r.itinerary()
	if r.context.Appliance.Status.MoRef == "" {
		list, listErr := itinerary.List()
		if listErr != nil || len(list) == 0 {
			err = listErr
			return
		}
		r.context.Appliance.Status.Phase = list[len(list)-1].Name
		return
	}
	step, err := itinerary.First()
	if err != nil {
		return
	}
	r.context.Appliance.Status.Phase = step.Name
	return
}

// Run runs the current teardown phase once and reports how long to wait before
// the next pass. A finished step sets Status.Phase to its successor; the next
// reconcile picks it up.
func (r *TeardownRunner) Run(ctx context.Context) (reQ time.Duration, err error) {
	// Ended() swallows the error and controller-runtime applies no backoff of
	// its own, so a failed pass would otherwise retry against vCenter forever
	// at the error cadence.
	defer func() {
		if err != nil {
			reQ = base.LongReQ
		}
	}()
	// A deploy phase, an empty phase, or a previous teardown failure is not a
	// step of this pipeline. Giving up means an undeletable CR and source vmdks
	// locked forever, so a failed teardown restarts rather than parking.
	if _, gErr := r.itinerary().Get(r.context.Appliance.Status.Phase); gErr != nil {
		err = r.begin()
		if err != nil {
			return
		}
	}
	err = r.context.CheckInstance()
	if err != nil {
		return
	}
	reQ, err = r.execute(ctx)
	if err != nil {
		r.context.Appliance.Status.Phase = r.failedPhase()
		r.context.Log.Error(err, "Teardown phase failed.", "phase", r.context.Appliance.Status.Phase)
	}
	return
}

func (r *TeardownRunner) itinerary() *libitr.Itinerary {
	return &libitr.Itinerary{
		Name: "Teardown",
		Pipeline: libitr.Pipeline{
			{Name: api.PhasePowerOff},
			{Name: api.PhaseWaitForPowerOff},
			{Name: api.PhaseDetachDisks},
			{Name: api.PhaseWaitForDetachDisks},
			{Name: api.PhaseDestroyVM},
			{Name: api.PhaseWaitForDestroyVM},
			{Name: api.PhaseTeardownCompleted},
		},
	}
}

func (r *TeardownRunner) failedPhase() string {
	return api.PhaseTeardownFailed
}

func (r *TeardownRunner) NextPhase() {
	nextPhase(r.context.Appliance, r.itinerary())
}

// execute runs the phase the appliance is on and reports how long to wait
// before running the next one. A step that advances the phase asks for no wait:
// writing the new phase to the status fires the watch, which brings the next
// pass back at once. A step still waiting names the interval it wants to be
// polled at. Every wait here is on a vSphere task, which settles in seconds,
// and each pass costs a vCenter session.
func (r *TeardownRunner) execute(ctx context.Context) (reQ time.Duration, err error) {
	switch r.context.Appliance.Status.Phase {
	case api.PhasePowerOff:
		// A setup pod still streaming at a VM about to be destroyed is holding
		// a login into it open for nothing. Failing the teardown over it would
		// leave an undeletable CR and read locks on the source vmdks, so the
		// error is logged and the teardown goes on.
		if podErr := r.deleteSetupPods(ctx); podErr != nil {
			r.context.Log.Error(podErr, "Could not delete the appliance setup pods.")
		}
		vm := r.context.VM(r.context.Appliance.Status.MoRef)
		task, powerErr := libvsphere.PowerOff(ctx, vm)
		if powerErr != nil {
			err = powerErr
			return
		}
		r.context.SetTask(task)
		r.NextPhase()
	case api.PhaseWaitForPowerOff:
		done, _, waitErr := r.context.WaitForTask(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if done {
			r.NextPhase()
			return
		}
		reQ = base.SlowReQ
	case api.PhaseDetachDisks:
		vm := r.context.VM(r.context.Appliance.Status.MoRef)
		task, detachErr := r.context.DetachDisks(ctx, vm)
		if detachErr != nil {
			err = detachErr
			return
		}
		r.context.SetTask(task)
		r.NextPhase()
	case api.PhaseWaitForDetachDisks:
		done, _, waitErr := r.context.WaitForTask(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if done {
			r.NextPhase()
			return
		}
		reQ = base.SlowReQ
	case api.PhaseDestroyVM:
		vm := r.context.VM(r.context.Appliance.Status.MoRef)
		task, destroyErr := libvsphere.DestroyVM(ctx, vm)
		if destroyErr != nil {
			err = destroyErr
			return
		}
		r.context.SetTask(task)
		r.NextPhase()
	case api.PhaseWaitForDestroyVM:
		done, _, waitErr := r.context.WaitForTask(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if done {
			r.NextPhase()
			return
		}
		reQ = base.SlowReQ
	case api.PhaseTeardownCompleted:
		// The VM is gone, so the moRef names nothing and the addresses reach
		// nothing. Clearing them makes a repeated teardown a no-op rather than
		// a second destroy attempt.
		if r.context.Appliance.Status.MoRef != "" {
			r.context.Log.Info("Deleted appliance VM.", "vm", r.context.Appliance.Status.MoRef)
			r.context.Appliance.Status.MoRef = ""
		}
		r.context.Appliance.Status.Addresses = nil
		r.context.Appliance.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.True,
			Category: libcnd.Required,
			Message:  "Tearing down the copy appliance has succeeded.",
		})
	case api.PhaseTeardownFailed:
		r.context.Appliance.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.False,
			Category: libcnd.Critical,
			Message:  "Tearing down the copy appliance has failed.",
		})
		// Defensive: Run re-seeds any phase outside the itinerary, and this one
		// is not in it, so a failed teardown restarts rather than arriving here.
		reQ = base.LongReQ
	default:
		err = liberr.New("unknown phase", "phase", r.context.Appliance.Status.Phase)
	}
	return
}
