package copyappliance

import (
	"context"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

// ExportRunner drives disk release and re-export on an already-deployed appliance.
type ExportRunner struct {
	context *ApplianceContext
}

// wantExport reports whether Spec.AttachDisks should be attached and exporting.
func wantExport(appliance *api.CopyAppliance) bool {
	return len(appliance.Spec.AttachDisks) > 0
}

// PendingExportRequest reports whether a terminal appliance must re-enter the
// export pipeline. DeployCompleted means disks are attached; Released means
// they are not. Spec.AttachDisks is the desired attachment set, so only a
// mismatch is pending. Warm precopy always Release→Export.
func PendingExportRequest(appliance *api.CopyAppliance) bool {
	switch appliance.Status.Phase {
	case api.PhaseDeployCompleted:
		return !wantExport(appliance)
	case api.PhaseReleased:
		return wantExport(appliance)
	default:
		return false
	}
}

// begin seeds the export pipeline from Spec.AttachDisks.
func (r *ExportRunner) begin() (err error) {
	r.context.Appliance.Status.TaskRef = ""
	itinerary := r.itinerary()
	step, err := itinerary.First()
	if err != nil {
		r.context.Appliance.Status.Phase = r.failedPhase()
		return
	}
	r.context.Appliance.Status.Phase = step.Name
	return
}

// Run runs the current export phase once and reports how long to wait before
// the next pass. A finished step sets Status.Phase to its successor; the next
// reconcile picks it up.
func (r *ExportRunner) Run(ctx context.Context) (reQ time.Duration, err error) {
	// Ended() swallows the error and controller-runtime applies no backoff of
	// its own, so a failed pass would otherwise retry against vCenter forever
	// at the error cadence.
	defer func() {
		if err != nil {
			reQ = base.LongReQ
		}
	}()
	// Seed only from a stable phase. WaitForExports is shared with deploy and
	// is not an export phase, so seeding there would restart attach every pass.
	if PendingExportRequest(r.context.Appliance) {
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
		r.context.Log.Error(err, "Export phase failed.", "phase", r.context.Appliance.Status.Phase)
	}
	return
}

func (r *ExportRunner) itinerary() *libitr.Itinerary {
	if wantExport(r.context.Appliance) {
		return &libitr.Itinerary{
			Name: "Export",
			Pipeline: libitr.Pipeline{
				{Name: api.PhaseAttachDisks},
				{Name: api.PhaseWaitForAttachDisks},
				{Name: api.PhaseRestartOrchestrator},
				{Name: api.PhaseWaitForExports},
				{Name: api.PhaseDeployCompleted},
			},
		}
	}
	return &libitr.Itinerary{
		Name: "Release",
		Pipeline: libitr.Pipeline{
			{Name: api.PhaseReleaseDisks},
			{Name: api.PhaseWaitForReleaseDisks},
			{Name: api.PhaseReleased},
		},
	}
}

func (r *ExportRunner) failedPhase() string {
	return api.PhaseDeployFailed
}

func (r *ExportRunner) NextPhase() {
	nextPhase(r.context.Appliance, r.itinerary())
}

// execute runs the phase the appliance is on and reports how long to wait
// before running the next one. A step that advances the phase asks for no wait:
// writing the new phase to the status fires the watch, which brings the next
// pass back at once. A step still waiting names the interval it wants to be
// polled at, and those are deliberately slow because each pass costs a vCenter
// session.
func (r *ExportRunner) execute(ctx context.Context) (reQ time.Duration, err error) {
	phase := r.context.Appliance.Status.Phase
	switch phase {
	case api.PhaseReleaseDisks:
		detachVM := r.context.VM(r.context.Appliance.Status.MoRef)
		detachTask, detachErr := r.context.DetachAttachedDisks(ctx, detachVM)
		if detachErr != nil {
			err = detachErr
			return
		}
		r.context.SetTask(detachTask)
		r.NextPhase()
	case api.PhaseWaitForReleaseDisks:
		done, _, waitErr := r.context.WaitForTask(ctx)
		if waitErr != nil {
			err = waitErr
			return
		}
		if !done {
			// A vSphere task, which settles in seconds.
			reQ = base.SlowReQ
			return
		}
		r.context.Appliance.Status.Exports = nil
		r.NextPhase()
		// Ready must be set on this pass: the reconciler idles on Released.
		r.context.Appliance.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.True,
			Category: libcnd.Required,
			Message:  "Copy appliance disks have been released.",
		})
	case api.PhaseAttachDisks:
		attachVM := r.context.VM(r.context.Appliance.Status.MoRef)
		attachTask, attachErr := r.context.AttachDisks(ctx, attachVM)
		if attachErr != nil {
			err = attachErr
			return
		}
		r.context.SetTask(attachTask)
		r.NextPhase()
	case api.PhaseWaitForAttachDisks:
		done, _, waitErr := r.context.WaitForTask(ctx)
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
	case api.PhaseRestartOrchestrator:
		address, ok := r.context.Appliance.Address()
		if !ok {
			err = liberr.New(
				"the appliance reports no address to reach it on",
				"appliance", r.context.Appliance.Name)
			return
		}
		orch, ready, loginErr := NewOrchestrator(ctx, r.context, SSHFileTransferTimeout)
		if loginErr != nil {
			err = loginErr
			return
		}
		if !ready {
			r.context.Log.Info("The appliance is not answering on SSH yet.", "address", address)
			// Waiting on sshd, which is seconds away on an appliance that was
			// answering a moment ago.
			reQ = base.SlowReQ
			return
		}
		defer func() {
			_ = orch.Close()
		}()
		err = orch.Restart()
		if err != nil {
			if !IsExitError(err) {
				r.context.Log.Info(
					"Lost the connection to the appliance while restarting the supervisor.",
					"address", address,
					"error", err.Error())
				err = nil
				reQ = base.SlowReQ
			}
			return
		}
		r.NextPhase()
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
				Message:  "Copy appliance disk export has succeeded.",
			})
			return
		}
		// Waiting on the guest to enumerate its disks and bring up a container
		// for each, which is tens of seconds. Polling that at the task cadence
		// buys nothing but vCenter logins.
		reQ = base.LongReQ
	case api.PhaseReleased, api.PhaseDeployCompleted:
		msg := "Copy appliance disk export has succeeded."
		if phase == api.PhaseReleased {
			msg = "Copy appliance disks have been released."
		}
		r.context.Appliance.Status.SetCondition(libcnd.Condition{
			Type:     libcnd.Ready,
			Status:   libcnd.True,
			Category: libcnd.Required,
			Message:  msg,
		})
	default:
		err = liberr.New("unexpected phase for export", "phase", phase)
	}
	return
}
