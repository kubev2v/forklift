package copyappliance

import (
	"context"
	"errors"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/vmware/govmomi/vim25/types"
	core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeployRunner drives the appliance VM from nothing to running. It holds no
// state of its own: every pass reads where it got to from the appliance status
// and leaves the next phase behind.
type DeployRunner struct {
	context *ApplianceContext
	client  client.Client
}

// begin seeds the deploy pipeline and records the vCenter the appliance VM
// will belong to.
func (r *DeployRunner) begin() (err error) {
	r.context.Appliance.Status.TaskRef = ""
	r.context.Appliance.Status.VCenterInstanceUUID = r.context.InstanceUUID()
	step, err := r.itinerary().First()
	if err != nil {
		return
	}
	r.context.Appliance.Status.Phase = step.Name
	return
}

// Run runs the current deploy phase once and reports how long to wait before
// the next pass. A finished step sets Status.Phase to its successor; the next
// reconcile picks it up.
func (r *DeployRunner) Run(ctx context.Context) (reQ time.Duration, err error) {
	// Ended() swallows the error and controller-runtime applies no backoff of
	// its own, so a failed pass would otherwise retry against vCenter forever
	// at the error cadence.
	defer func() {
		if err != nil {
			reQ = base.LongReQ
		}
	}()
	// Only an appliance that has not started yet is seeded. A phase outside
	// the pipeline is a failed deploy, which execute parks; seeding it would
	// restart the deploy on every pass.
	if r.context.Appliance.Status.Phase == "" {
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
		log := []interface{}{"phase", r.context.Appliance.Status.Phase}
		var detail *liberr.Error
		if errors.As(err, &detail) && len(detail.Context()) > 0 {
			log = append(log, "details", detail.Context())
		}
		r.context.Log.Error(err, "Deploy phase failed.", log...)
	}
	return
}

func (r *DeployRunner) itinerary() *libitr.Itinerary {
	return &libitr.Itinerary{
		Name: "Deploy",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseCloneVM},
			{Name: api.PhaseWaitForClone},
			{Name: api.PhaseWaitForNetwork},
			{Name: api.PhaseSetupAppliance},
			{Name: api.PhaseWaitForExports},
			{Name: api.PhaseDeployCompleted},
		},
	}
}

func (r *DeployRunner) failedPhase() string {
	return api.PhaseDeployFailed
}

// NextPhase sets Status.Phase to itinerary.Next of the current phase.
func (r *DeployRunner) NextPhase() {
	nextPhase(r.context.Appliance, r.itinerary())
}

// nextPhase sets Status.Phase to itinerary.Next of the current phase.
func nextPhase(appliance *api.CopyAppliance, itinerary *libitr.Itinerary) {
	step, done, err := itinerary.Next(appliance.Status.Phase)
	if done || err != nil {
		return
	}
	appliance.Status.Phase = step.Name
}

// execute runs the phase the appliance is on and reports how long to wait
// before running the next one. A step that advances the phase asks for no wait:
// writing the new phase to the status fires the watch, which brings the next
// pass back at once. A step still waiting writes nothing, so nothing wakes us,
// and it names the interval it wants to be polled at. Every wait here is on
// vSphere, so those intervals are deliberately slow: each reconcile opens and
// closes a vCenter session, and FastReQ would mean two logins per second per CR.
func (r *DeployRunner) execute(ctx context.Context) (reQ time.Duration, err error) {
	switch r.context.Appliance.Status.Phase {
	case api.PhaseCloneVM:
		task, cloneErr := r.context.CloneVM(ctx)
		if cloneErr != nil {
			err = cloneErr
			return
		}
		r.context.SetTask(task)
		r.NextPhase()
	case api.PhaseWaitForClone:
		done, waitErr := r.WaitForClone(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if done {
			r.NextPhase()
			return
		}
		// A vSphere task, which settles in seconds.
		reQ = base.SlowReQ
	case api.PhaseWaitForNetwork:
		done, waitErr := r.WaitForNetwork(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if done {
			r.NextPhase()
			return
		}
		// Slower than the task waits. Those are waiting on vSphere, which
		// settles in seconds; this one is waiting on a guest to boot and on
		// VMware Tools to start answering, which takes a minute or more.
		// Polling it at the task cadence buys nothing but vCenter logins.
		reQ = base.LongReQ
	case api.PhaseSetupAppliance:
		done, setupErr := r.SetupAppliance(ctx)
		if setupErr != nil {
			err = setupErr
			return
		}
		if done {
			r.NextPhase()
			return
		}
		// The pod watch requeues on every change the pod makes, so this is a
		// backstop for a missed event rather than the poll that drives the
		// step.
		reQ = base.LongReQ
	case api.PhaseLoadImage, api.PhaseConfigure:
		// Both steps now run in the setup pod, which does the whole job and is
		// safe to re-enter whatever state the appliance is in. An appliance an
		// older controller left on either phase restarts there.
		r.context.Appliance.Status.Phase = api.PhaseSetupAppliance
	case api.PhaseWaitForExports:
		done, waitErr := r.context.WaitForExports(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if done {
			r.NextPhase()
			// Ready must be set on this pass: the reconciler idles on
			// DeployCompleted, so the case below would never run.
			r.context.Appliance.Status.SetCondition(libcnd.Condition{
				Type:     libcnd.Ready,
				Status:   libcnd.True,
				Category: libcnd.Required,
				Message:  "Deploying the copy appliance has succeeded.",
			})
			return
		}
		// Waiting on the guest to enumerate its disks and bring up a container
		// for each, which is tens of seconds. Same reasoning as
		// api.PhaseWaitForNetwork.
		reQ = base.LongReQ
	case api.PhaseDeployCompleted:
		r.context.Appliance.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.True,
			Category: libcnd.Required,
			Message:  "Deploying the copy appliance has succeeded.",
		})
	case api.PhaseDeployFailed:
		// Keep the detailed fault from setFailed when present; only fall back
		// to a generic message if nothing recorded the root cause.
		msg := FailureReason(r.context.Appliance)
		if msg == "the appliance did not record why it failed" {
			msg = "Deploying the copy appliance has failed."
		}
		r.context.Appliance.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.False,
			Reason:   api.PhaseDeployFailed,
			Category: libcnd.Critical,
			Message:  msg,
			Durable:  true,
		})
		// Ended() swallows the error and controller-runtime applies no backoff
		// of its own, so a failed appliance would otherwise retry against
		// vCenter forever at the error cadence.
		reQ = base.LongReQ
	default:
		err = liberr.New("unknown phase", "phase", r.context.Appliance.Status.Phase)
	}
	return
}

// WaitForClone reports whether the appliance VM has finished cloning, and
// records the moRef. Adopt (MoRef set, no task) counts as done.
func (r *DeployRunner) WaitForClone(ctx context.Context) (done bool, err error) {
	done, result, err := r.context.WaitForTask(ctx)
	if err != nil {
		return
	}
	if !done {
		return
	}
	if r.context.Appliance.Status.MoRef != "" && result == nil {
		return
	}
	moRef, ok := result.(types.ManagedObjectReference)
	if !ok {
		err = liberr.New("task result is not a ManagedObjectRef", "result", result)
		return
	}
	r.context.Appliance.Status.MoRef = moRef.Reference().Value
	return
}

// WaitForNetwork reports whether the appliance can be reached, and records the
// addresses the guest reports itself on. CloneSpec.PowerOn is asynchronous to
// the clone task, so a powered-off VM may still be coming up; a failed power-on
// task is reported instead of waiting forever for an address.
func (r *DeployRunner) WaitForNetwork(ctx context.Context) (done bool, err error) {
	vm := r.context.VM(r.context.Appliance.Status.MoRef)

	state, err := vm.PowerState(ctx)
	if err != nil {
		err = liberr.Wrap(err, "vm", r.context.Appliance.Status.MoRef)
		return
	}
	if state == types.VirtualMachinePowerStatePoweredOff {
		if fault := r.context.recentTaskFault(ctx, vm); fault != "" {
			err = liberr.New("appliance VM failed to power on: " + fault)
		}
		return
	}

	addresses, err := r.context.GuestAddresses(ctx, vm)
	if err != nil {
		return
	}
	r.context.Appliance.Status.Addresses = addresses

	_, done = r.context.Appliance.Address()
	return
}

// SetupAppliance loads the appliance's container image into its podman store
// and installs the supervisor, in a pod, and reports whether that pod has
// finished. The pod is found by label, so it is created once and then only
// read.
func (r *DeployRunner) SetupAppliance(ctx context.Context) (done bool, err error) {
	pod, err := r.setupPod(ctx)
	if err != nil {
		return
	}
	if pod == nil {
		err = r.startSetup(ctx)
		return
	}
	switch pod.Status.Phase {
	case core.PodSucceeded:
		done = true
	case core.PodFailed:
		err = liberr.New(
			"setting the appliance up failed",
			"pod", pod.Name,
			"reason", setupFailure(pod))
	}
	return
}

// startSetup resolves the appliance's container image and starts the pod that
// loads it. The image is resolved here rather than in the pod so that one that
// is missing or unreadable is reported on the CopyAppliance as an error rather
// than as a pod that failed, and so the pod can be handed a digest.
func (r *DeployRunner) startSetup(ctx context.Context) (err error) {
	img, spec, err := resolveImage(ctx, r.context.Appliance.Spec.ContainerImage)
	if err != nil {
		return
	}
	tag, err := makeTag(img)
	if err != nil {
		return
	}
	// Recorded before the pod exists. The orchestrator unit the pod installs
	// runs this reference, and the export steps read it back.
	r.context.Appliance.Status.ExporterImage = tag.Name()
	err = r.createSetupPod(ctx, spec, tag.Name())
	if err != nil {
		return
	}
	r.context.Log.Info("Setting the appliance up in a pod.",
		"image", r.context.Appliance.Spec.ContainerImage,
		"as", tag.Name())
	return
}
