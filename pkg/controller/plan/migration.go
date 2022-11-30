package plan

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	"github.com/konveyor/forklift-controller/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Requeue
const (
	NoReQ   = time.Duration(0)
	PollReQ = time.Second * 3
)

// Predicates.
var (
	HasPreHook         libitr.Flag = 0x01
	HasPostHook        libitr.Flag = 0x02
	RequiresConversion libitr.Flag = 0x04
)

// Phases.
const (
	Started                  = "Started"
	PreHook                  = "PreHook"
	StorePowerState          = "StorePowerState"
	PowerOffSource           = "PowerOffSource"
	WaitForPowerOff          = "WaitForPowerOff"
	CreateDataVolumes        = "CreateDataVolumes"
	CreateVM                 = "CreateVM"
	CopyDisks                = "CopyDisks"
	CopyingPaused            = "CopyingPaused"
	AddCheckpoint            = "AddCheckpoint"
	AddFinalCheckpoint       = "AddFinalCheckpoint"
	CreateSnapshot           = "CreateSnapshot"
	CreateInitialSnapshot    = "CreateInitialSnapshot"
	CreateFinalSnapshot      = "CreateFinalSnapshot"
	Finalize                 = "Finalize"
	CreateGuestConversionPod = "CreateGuestConversionPod"
	ConvertGuest             = "ConvertGuest"
	PostHook                 = "PostHook"
	Completed                = "Completed"
	WaitForSnapshot          = "WaitForSnapshot"
	WaitForInitialSnapshot   = "WaitForInitialSnapshot"
	WaitForFinalSnapshot     = "WaitForFinalSnapshot"
)

// Steps.
const (
	Initialize      = "Initialize"
	Cutover         = "Cutover"
	DiskTransfer    = "DiskTransfer"
	ImageConversion = "ImageConversion"
	VMCreation      = "VirtualMachineCreation"
	Unknown         = "Unknown"
)

// Power states.
const (
	On = "On"
)

var (
	coldItinerary = libitr.Itinerary{
		Name: "",
		Pipeline: libitr.Pipeline{
			{Name: Started},
			{Name: PreHook, All: HasPreHook},
			{Name: StorePowerState},
			{Name: PowerOffSource},
			{Name: WaitForPowerOff},
			{Name: CreateDataVolumes},
			{Name: CopyDisks},
			{Name: CreateGuestConversionPod, All: RequiresConversion},
			{Name: ConvertGuest, All: RequiresConversion},
			{Name: CreateVM},
			{Name: PostHook, All: HasPostHook},
			{Name: Completed},
		},
	}
	warmItinerary = libitr.Itinerary{
		Name: "Warm",
		Pipeline: libitr.Pipeline{
			{Name: Started},
			{Name: PreHook, All: HasPreHook},
			{Name: CreateInitialSnapshot},
			{Name: WaitForInitialSnapshot},
			{Name: CreateDataVolumes},
			{Name: CopyDisks},
			{Name: CopyingPaused},
			{Name: CreateSnapshot},
			{Name: WaitForSnapshot},
			{Name: AddCheckpoint},
			{Name: StorePowerState},
			{Name: PowerOffSource},
			{Name: WaitForPowerOff},
			{Name: CreateFinalSnapshot},
			{Name: WaitForFinalSnapshot},
			{Name: AddFinalCheckpoint},
			{Name: Finalize},
			{Name: CreateGuestConversionPod, All: RequiresConversion},
			{Name: ConvertGuest, All: RequiresConversion},
			{Name: CreateVM},
			{Name: PostHook, All: HasPostHook},
			{Name: Completed},
		},
	}
)

// Migration.
type Migration struct {
	*plancontext.Context
	// Builder
	builder adapter.Builder
	// kubevirt.
	kubevirt KubeVirt
	// Source client.
	provider adapter.Client
	// VirtualMachine CRs.
	vmMap VirtualMachineMap
	// VM scheduler
	scheduler scheduler.Scheduler
}

// Type of migration.
func (r *Migration) Type() string {
	return r.Context.Source.Provider.Type().String()
}

// Run the migration.
func (r *Migration) Run() (reQ time.Duration, err error) {
	defer func() {
		if r.provider != nil {
			r.provider.Close()
		}
	}()
	reQ = PollReQ
	err = r.init()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.begin()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	r.resolveCanceledRefs()

	for _, vm := range r.runningVMs() {
		err = r.execute(vm)
		if err != nil {
			return
		}
	}

	vm, hasNext, err := r.scheduler.Next()
	if err != nil {
		return
	}
	if hasNext {
		err = r.execute(vm)
		if err != nil {
			return
		}
	}

	completed, err := r.end()
	if completed {
		reQ = NoReQ
	}

	return
}

// Get/Build resources.
func (r *Migration) init() (err error) {
	adapter, err := adapter.New(r.Context.Source.Provider)
	if err != nil {
		return
	}
	r.provider, err = adapter.Client(r.Context)
	if err != nil {
		return
	}
	r.builder, err = adapter.Builder(r.Context)
	if err != nil {
		return
	}
	r.kubevirt = KubeVirt{
		Context: r.Context,
		Builder: r.builder,
	}
	r.scheduler, err = scheduler.New(r.Context)
	if err != nil {
		return
	}

	return
}

// Begin the migration.
func (r *Migration) begin() (err error) {
	snapshot := r.Plan.Status.Migration.ActiveSnapshot()
	if snapshot.HasAnyCondition(Executing, Succeeded, Failed, Canceled) {
		return
	}
	r.Plan.Status.Migration.MarkReset()
	r.Plan.Status.Migration.MarkStarted()
	snapshot.SetCondition(
		libcnd.Condition{
			Type:     Executing,
			Status:   True,
			Category: Advisory,
			Message:  "The plan is EXECUTING.",
			Durable:  true,
		})
	err = r.kubevirt.EnsureNamespace()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	//
	// Delete
	kept := []*plan.VMStatus{}
	for _, status := range r.Plan.Status.Migration.VMs {

		// resolve the VM ref
		_, err = r.Source.Inventory.VM(&status.Ref)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		if _, found := r.Plan.Spec.FindVM(status.Ref); found {
			kept = append(kept, status)
		}
	}
	r.Plan.Status.Migration.VMs = kept
	//
	// Add/Update.
	list := []*plan.VMStatus{}
	for _, vm := range r.Plan.Spec.VMs {
		var status *plan.VMStatus
		r.itinerary().Predicate = &Predicate{vm: &vm, context: r.Context}
		step, _ := r.itinerary().First()
		if current, found := r.Plan.Status.Migration.FindVM(vm.Ref); !found {
			status = &plan.VMStatus{VM: vm}
			if r.Plan.Spec.Warm {
				status.Warm = &plan.Warm{}
			}
		} else {
			status = current
		}
		if status.Phase != Completed || status.HasAnyCondition(Canceled, Failed) {
			pipeline, pErr := r.buildPipeline(&vm)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			status.DeleteCondition(Canceled, Failed)
			status.MarkReset()
			status.Pipeline = pipeline
			status.Phase = step.Name
			status.Error = nil
			if r.Plan.Spec.Warm {
				status.Warm = &plan.Warm{}
			}
			log.Info(
				"Pipeline reset.",
				"vm",
				vm.String())
		} else {
			log.Info(
				"Pipeline preserved.",
				"vm",
				vm.String())
		}
		list = append(list, status)
	}

	r.Plan.Status.Migration.VMs = list

	r.Log.Info("Migration [STARTED]")

	return
}

// Archive the plan.
// Best effort to remove any retained migration resources.
func (r *Migration) Archive() {
	err := r.init()
	if err != nil {
		r.Log.Error(err, "Archive initialization failed.")
		return
	}

	for _, vm := range r.Plan.Status.Migration.VMs {
		err = r.CleanUp(vm)
		if err != nil {
			r.Log.Error(err,
				"Couldn't clean up VM while archiving plan.",
				"vm",
				vm.String())
		}
	}
	return
}

// Cancel the migration.
// Delete resources associated with VMs that have been marked canceled.
func (r *Migration) Cancel() (err error) {
	err = r.init()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	for _, vm := range r.Plan.Status.Migration.VMs {
		if vm.HasCondition(Canceled) {
			err = r.CleanUp(vm)
			if err != nil {
				r.Log.Error(err,
					"Couldn't clean up after canceled VM migration.",
					"vm",
					vm.String())
				err = nil
			}
			if vm.RestorePowerState == On {
				err = r.provider.PowerOn(vm.Ref)
				if err != nil {
					r.Log.Error(err,
						"Couldn't restore the power state of the source VM.",
						"vm",
						vm.String())
					err = nil
				}
			}
			vm.MarkCompleted()
			for _, step := range vm.Pipeline {
				if step.MarkedStarted() {
					step.MarkCompleted()
				}
			}
		}
	}

	return
}

// Delete left over migration resources associated with a VM.
func (r *Migration) CleanUp(vm *plan.VMStatus) (err error) {
	if vm.HasCondition(Succeeded) {
		err = r.deleteImporterPods(vm)
		if err != nil {
			return
		}
	} else {
		err = r.kubevirt.DeleteVM(vm)
		if err != nil {
			return
		}
	}
	err = r.kubevirt.DeleteGuestConversionPod(vm)
	if err != nil {
		return
	}
	err = r.kubevirt.DeleteSecret(vm)
	if err != nil {
		return
	}
	err = r.kubevirt.DeleteConfigMap(vm)
	if err != nil {
		return
	}
	err = r.kubevirt.DeleteHookJobs(vm)
	if err != nil {
		return
	}
	if vm.Warm != nil {
		_ = r.provider.RemoveSnapshots(vm.Ref, vm.Warm.Precopies)
	}

	return
}

func (r *Migration) deleteImporterPods(vm *plan.VMStatus) (err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}
	pvcs, err := r.kubevirt.getPVCs(vm)
	if err != nil {
		return
	}
	for _, pvc := range pvcs {
		err = r.kubevirt.DeleteImporterPod(pvc)
		if err != nil {
			return
		}
	}
	return
}

// Best effort attempt to resolve canceled refs.
func (r *Migration) resolveCanceledRefs() {
	for i := range r.Context.Migration.Spec.Cancel {
		// resolve the VM ref in place
		ref := &r.Context.Migration.Spec.Cancel[i]
		_, _ = r.Source.Inventory.VM(ref)
	}
}

func (r *Migration) runningVMs() (vms []*plan.VMStatus) {
	vms = make([]*plan.VMStatus, 0)
	for i := range r.Plan.Status.Migration.VMs {
		vm := r.Plan.Status.Migration.VMs[i]
		if vm.Running() {
			vms = append(vms, vm)
		}
	}
	return
}

// Next step in the itinerary.
func (r *Migration) next(phase string) (next string) {
	step, done, err := r.itinerary().Next(phase)
	if done || err != nil {
		next = Completed
		if err != nil {
			r.Log.Error(err, "Next phase failed.")
		}
	} else {
		next = step.Name
	}

	return
}

// Get the itinerary for the migration type.
func (r *Migration) itinerary() *libitr.Itinerary {
	if r.Plan.Spec.Warm {
		return &warmItinerary
	} else {
		return &coldItinerary
	}
}

// Get the name of the pipeline step corresponding to the current VM phase.
func (r *Migration) step(vm *plan.VMStatus) (step string) {
	switch vm.Phase {
	case Started, CreateInitialSnapshot, WaitForInitialSnapshot, CreateDataVolumes:
		step = Initialize
	case CopyDisks, CopyingPaused, CreateSnapshot, WaitForSnapshot, AddCheckpoint:
		step = DiskTransfer
	case CreateFinalSnapshot, WaitForFinalSnapshot, AddFinalCheckpoint, Finalize:
		step = Cutover
	case CreateGuestConversionPod, ConvertGuest:
		step = ImageConversion
	case CreateVM:
		step = VMCreation
	case PreHook, PostHook:
		step = vm.Phase
	case StorePowerState, PowerOffSource, WaitForPowerOff:
		if r.Plan.Spec.Warm {
			step = Cutover
		} else {
			step = Initialize
		}
	default:
		step = Unknown
	}
	return
}

// Steps a VM through the migration itinerary
// and updates its status.
func (r *Migration) execute(vm *plan.VMStatus) (err error) {
	// check whether the VM has been canceled by the user
	if r.Context.Migration.Spec.Canceled(vm.Ref) {
		vm.SetCondition(
			libcnd.Condition{
				Type:     Canceled,
				Status:   True,
				Category: Advisory,
				Reason:   UserRequested,
				Message:  "The migration has been canceled.",
				Durable:  true,
			})
		vm.Phase = Completed
		r.Log.Info(
			"Migration [CANCELED]",
			"vm",
			vm.String())
		return
	}
	r.itinerary().Predicate = &Predicate{
		vm:      &vm.VM,
		context: r.Context,
	}

	r.Log.Info(
		"Migration [RUN]",
		"vm",
		vm.String(),
		"phase",
		vm.Phase)
	r.Log.V(2).Info(
		"Migrating VM (definition).",
		"vm",
		vm)

	switch vm.Phase {
	case Started:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		vm.MarkStarted()
		step.MarkStarted()
		step.Phase = Running
		err = r.CleanUp(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		vm.Phase = r.next(vm.Phase)
	case PreHook, PostHook:
		runner := HookRunner{Context: r.Context}
		err = runner.Run(vm)
		if err != nil {
			return
		}
		if step, found := vm.FindStep(r.step(vm)); found {
			step.Phase = Running
			if step.MarkedCompleted() && step.Error == nil {
				step.Phase = Completed
				vm.Phase = r.next(vm.Phase)
			}
		} else {
			vm.Phase = Completed
		}
	case CreateDataVolumes:
		step, found := vm.FindStep(r.step(vm))
		if *r.Plan.Provider.Source.Spec.Type == v1beta1.OVirt {
			if vm.Warm == nil {
				err = r.createVolumes(vm.Ref)
				vm.Phase = r.next(vm.Phase)
				return
			}
		}
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		var dataVolumes []cdi.DataVolume
		dataVolumes, err = r.kubevirt.DataVolumes(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}
		if vm.Warm != nil {
			err = r.provider.SetCheckpoints(vm.Ref, vm.Warm.Precopies, dataVolumes, false)
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}
		}
		err = r.kubevirt.EnsureDataVolumes(vm, dataVolumes)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}

		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.next(vm.Phase)
	case CreateVM:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		step.MarkStarted()
		step.Phase = Running
		err = r.kubevirt.EnsureVM(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}
		// Removing unnecessary DataVolumes
		err = r.kubevirt.DeleteDataVolumes(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.next(vm.Phase)
	case CopyDisks:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		step.MarkStarted()
		step.Phase = Running

		if *r.Plan.Provider.Source.Spec.Type == v1beta1.OVirt {
			if vm.Warm == nil {
				ready, err := r.getOvirtPVCs(vm.Ref, step)
				if err != nil {
					step.AddError(err.Error())
					err = nil
					break
				}
				err = r.updateCopyProgressForOvirt(vm, step)

				if err != nil {
					step.AddError(err.Error())
					err = nil
					break
				}

				if ready {
					step.Phase = Completed
					vm.Phase = r.next(vm.Phase)
					break
				} else {
					r.Log.Info("PVCs not ready yet")
					break
				}
			}
		}

		err = r.updateCopyProgress(vm, step)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		if step.MarkedCompleted() && !step.HasError() {
			if r.Plan.Spec.Warm {
				now := meta.Now()
				next := meta.NewTime(now.Add(time.Duration(Settings.PrecopyInterval) * time.Minute))
				n := len(vm.Warm.Precopies)
				vm.Warm.Precopies[n-1].End = &now
				vm.Warm.NextPrecopyAt = &next
				vm.Warm.Successes++
			}
			step.Phase = Completed
			vm.Phase = r.next(vm.Phase)
		}
	case CopyingPaused:
		if r.Migration.Spec.Cutover != nil && !r.Migration.Spec.Cutover.After(time.Now()) {
			vm.Phase = StorePowerState
		} else if vm.Warm.NextPrecopyAt != nil && !vm.Warm.NextPrecopyAt.After(time.Now()) {
			vm.Phase = CreateSnapshot
		}
	case CreateInitialSnapshot, CreateSnapshot, CreateFinalSnapshot:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		var snapshot string
		snapshot, err = r.provider.CreateSnapshot(vm.Ref)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}
		now := meta.Now()
		precopy := plan.Precopy{Snapshot: snapshot, Start: &now}
		vm.Warm.Precopies = append(vm.Warm.Precopies, precopy)
		r.resetPrecopyTasks(vm, step)
		vm.Phase = r.next(vm.Phase)
	case WaitForInitialSnapshot, WaitForSnapshot, WaitForFinalSnapshot:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		snapshot := vm.Warm.Precopies[len(vm.Warm.Precopies)-1].Snapshot
		ready, err := r.provider.CheckSnapshotReady(vm.Ref, snapshot)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		if ready {
			vm.Phase = r.next(vm.Phase)
		}
	case AddCheckpoint, AddFinalCheckpoint:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}

		err = r.setDataVolumeCheckpoints(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}

		switch vm.Phase {
		case AddCheckpoint:
			vm.Phase = CopyDisks
		case AddFinalCheckpoint:
			vm.Phase = Finalize
		}
	case StorePowerState:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		var state string
		state, err = r.provider.PowerState(vm.Ref)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}
		vm.RestorePowerState = state
		vm.Phase = r.next(vm.Phase)
	case PowerOffSource:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		err = r.provider.PowerOff(vm.Ref)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}
		vm.Phase = r.next(vm.Phase)
	case WaitForPowerOff:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		var off bool
		off, err = r.provider.PoweredOff(vm.Ref)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}
		if off {
			vm.Phase = r.next(vm.Phase)
		}
	case Finalize:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		err = r.updateCopyProgress(vm, step)
		if err != nil {
			return
		}
		if step.MarkedCompleted() {
			err = r.provider.RemoveSnapshots(vm.Ref, vm.Warm.Precopies)
			if err != nil {
				r.Log.Info(
					"Failed to clean up warm migration snapshots.",
					"vm",
					vm)
				err = nil
			}
			if !step.HasError() {
				step.Phase = Completed
				vm.Phase = r.next(vm.Phase)
			}
		}
	case CreateGuestConversionPod:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		step.MarkStarted()
		step.Phase = Running
		err = r.ensureGuestConversionPod(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		vm.Phase = r.next(vm.Phase)
	case ConvertGuest:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		err = r.updateConversionProgress(vm, step)
		if err != nil {
			return
		}
		if step.MarkedCompleted() && !step.HasError() {
			step.Phase = Completed
			vm.Phase = r.next(vm.Phase)
		}
	case Completed:
		vm.MarkCompleted()
		r.Log.Info(
			"Migration [COMPLETED]",
			"vm",
			vm.String())
	default:
		r.Log.Info(
			"Phase unknown.",
			"vm",
			vm)
		vm.AddError(
			fmt.Sprintf(
				"Phase [%s] unknown",
				vm.Phase))
		vm.Phase = Completed
	}
	vm.ReflectPipeline()
	if vm.Phase == Completed && vm.Error == nil {
		vm.SetCondition(
			libcnd.Condition{
				Type:     Succeeded,
				Status:   True,
				Category: Advisory,
				Message:  "The VM migration has SUCCEEDED.",
				Durable:  true,
			})
		// Power on the destination VM if the source VM was originally powered on.
		err = r.setRunning(vm, vm.RestorePowerState == On)
		if err != nil {
			r.Log.Error(err,
				"Could not power on destination VM.",
				"vm",
				vm.String())
			err = nil
		}
	} else if vm.Error != nil {
		vm.Phase = Completed
		vm.SetCondition(
			libcnd.Condition{
				Type:     Failed,
				Status:   True,
				Category: Advisory,
				Message:  "The VM migration has FAILED.",
				Durable:  true,
			})
	}

	return
}

func (r *Migration) resetPrecopyTasks(vm *plan.VMStatus, step *plan.Step) {
	step.Completed = nil
	for _, task := range step.Tasks {
		task.Annotations["Precopy"] = fmt.Sprintf("%v", len(vm.Warm.Precopies))
		task.MarkReset()
		task.MarkStarted()
	}
}

func (r *Migration) createVolumes(vm ref.Ref) (err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}
	ovirtVm := &ovirt.Workload{}
	err = r.Source.Inventory.Find(ovirtVm, vm)
	if err != nil {
		return
	}
	url, err := url.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		return
	}

	storageName := &r.Context.Map.Storage.Spec.Map[0].Destination.StorageClass
	for _, da := range ovirtVm.DiskAttachments {
		populatorCr := v1beta1.OvirtImageIOPopulator{
			ObjectMeta: meta.ObjectMeta{
				Name:      da.DiskAttachment.ID,
				Namespace: r.Plan.Spec.TargetNamespace,
			},
			Spec: v1beta1.OvirtImageIOPopulatorSpec{
				EngineURL:        fmt.Sprintf("https://%s", url.Host),
				EngineSecretName: r.Source.Secret.Name,
				DiskID:           da.Disk.ID,
			},
		}

		err = r.Client.Create(context.Background(), &populatorCr, &client.CreateOptions{})
		if err != nil {
			return
		}
		apiGroup := "forklift.konveyor.io"

		pvc := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "PersistentVolumeClaim",
				"apiVersion": "v1",
				"metadata": map[string]interface{}{
					"name":      da.DiskAttachment.ID,
					"namespace": r.Plan.Spec.TargetNamespace,
				},
				"spec": map[string]interface{}{
					"storageClassName": storageName,
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"storage": resource.NewQuantity(da.Disk.ProvisionedSize, resource.BinarySI).String(),
						},
					},
					"accessModes": []string{"ReadWriteOnce"},
					"dataSourceRef": map[string]interface{}{
						"apiGroup": apiGroup,
						"kind":     "OvirtImageIOPopulator",
						"name":     populatorCr.Name,
					},
				},
			},
		}

		config, configErr := config.GetConfig()
		if configErr != nil {
			return configErr
		}

		dynamicClient, configErr := dynamic.NewForConfig(config)
		if configErr != nil {
			return configErr
		}

		pvcResource := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}

		_, err = dynamicClient.Resource(pvcResource).Namespace(r.Plan.Spec.TargetNamespace).Create(context.TODO(), pvc, meta.CreateOptions{})

		if err != nil {
			return
		}
	}

	return
}

func (r *Migration) getOvirtPVCs(vm ref.Ref, step *plan.Step) (ready bool, err error) {
	ovirtVm := &ovirt.Workload{}
	err = r.Source.Inventory.Find(ovirtVm, vm)
	if err != nil {
		return
	}
	ready = true

	for _, da := range ovirtVm.DiskAttachments {
		obj := client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: da.Disk.ID}
		pvc := core.PersistentVolumeClaim{}
		err = r.Client.Get(context.Background(), obj, &pvc)
		if err != nil {
			return
		}

		if pvc.Status.Phase != core.ClaimBound {
			ready = false
			continue
		}

		var task *plan.Task
		found := false
		task, found = step.FindTask(da.Disk.ID)
		if !found {
			continue
		}

		task.MarkCompleted()
	}

	return
}

// Build the pipeline for a VM status.
func (r *Migration) buildPipeline(vm *plan.VM) (pipeline []*plan.Step, err error) {
	r.itinerary().Predicate = &Predicate{vm: vm, context: r.Context}
	step, _ := r.itinerary().First()
	for {
		switch step.Name {
		case Started:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        Initialize,
						Description: "Initialize migration.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case PreHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PreHook,
						Description: "Run pre-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case CopyDisks:
			tasks, pErr := r.builder.Tasks(vm.Ref)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			total := int64(0)
			for _, task := range tasks {
				total += task.Progress.Total
			}
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        DiskTransfer,
						Description: "Transfer disks.",
						Progress: libitr.Progress{
							Total: total,
						},
						Annotations: map[string]string{
							"unit": "MB",
						},
						Phase: Pending,
					},
					Tasks: tasks,
				})
		case Finalize:
			tasks, pErr := r.builder.Tasks(vm.Ref)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			total := int64(0)
			for _, task := range tasks {
				total += task.Progress.Total
			}
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        Cutover,
						Description: "Finalize disk transfer.",
						Progress: libitr.Progress{
							Total: total,
						},
						Annotations: map[string]string{
							"unit": "MB",
						},
					},
					Tasks: tasks,
				})
		case ConvertGuest:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        ImageConversion,
						Description: "Convert image to kubevirt.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case PostHook:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        PostHook,
						Description: "Run post-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       Pending,
					},
				})
		case CreateVM:
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        VMCreation,
						Description: "Create VM.",
						Phase:       Pending,
						Progress:    libitr.Progress{Total: 1},
					},
				})
		}
		next, done, _ := r.itinerary().Next(step.Name)
		if !done {
			step = next
		} else {
			break
		}
	}

	log.V(2).Info(
		"Pipeline built.",
		"vm",
		vm.String())

	return
}

// End the migration.
func (r *Migration) end() (completed bool, err error) {
	failed := 0
	succeeded := 0
	for _, vm := range r.Plan.Status.Migration.VMs {
		if !vm.MarkedCompleted() {
			return
		}
		if vm.HasCondition(Failed) {
			failed++
		}
		if vm.HasCondition(Succeeded) {
			succeeded++
		}
	}
	r.Plan.Status.Migration.MarkCompleted()
	snapshot := r.Plan.Status.Migration.ActiveSnapshot()
	snapshot.DeleteCondition(Executing)

	if failed > 0 {
		// if any VMs failed, the migration failed.
		r.Log.Info("Migration [FAILED]")
		snapshot.SetCondition(
			libcnd.Condition{
				Type:     Failed,
				Status:   True,
				Category: Advisory,
				Message:  "The plan execution has FAILED.",
				Durable:  true,
			})
	} else if succeeded > 0 {
		// if the migration didn't fail and at least one VM succeeded,
		// then the migration succeeded.
		r.Log.Info("Migration [SUCCEEDED]")
		snapshot.SetCondition(
			libcnd.Condition{
				Type:     Succeeded,
				Status:   True,
				Category: Advisory,
				Message:  "The plan execution has SUCCEEDED.",
				Durable:  true,
			})
	} else {
		// if there were no failures or successes, but
		// all the VMs are complete, then the migration must
		// have been canceled.
		r.Log.Info("Migration [CANCELED]")
		snapshot.SetCondition(
			libcnd.Condition{
				Type:     Canceled,
				Status:   True,
				Category: Advisory,
				Message:  "The plan execution has been CANCELED.",
				Durable:  true,
			})
	}

	completed = true
	return
}

// Ensure the guest conversion pod is present.
func (r *Migration) ensureGuestConversionPod(vm *plan.VMStatus) (err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}
	var vmCr VirtualMachine
	var pvcs []core.PersistentVolumeClaim
	found := false
	if vmCr, found = r.vmMap[vm.ID]; !found {
		vmCr.VirtualMachine, err = r.kubevirt.virtualMachine(vm)
		if err != nil {
			return
		}
		pvcs, err = r.kubevirt.getPVCs(vm)
		if err != nil {
			return
		}
	}

	err = r.kubevirt.EnsureGuestConversionPod(vm, &vmCr, &pvcs)
	return
}

// Set the running state of the kubevirt VM.
func (r *Migration) setRunning(vm *plan.VMStatus, running bool) (err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}
	var vmCr VirtualMachine
	found := false
	if vmCr, found = r.vmMap[vm.ID]; !found {
		msg := "VirtualMachine CR not found."
		vm.AddError(msg)
		return
	}

	if vmCr.Spec.Running != nil && *vmCr.Spec.Running == running {
		return
	}

	err = r.kubevirt.SetRunning(&vmCr, running)
	return
}

func (r *Migration) updateCopyProgressForOvirt(vm *plan.VMStatus, step *plan.Step) (err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}

	var vmCr VirtualMachine
	found := false
	if vmCr, found = r.vmMap[vm.ID]; !found {
		msg := "VirtualMachine CR not found."
		vm.AddError(msg)
		return
	}

	for _, volume := range vmCr.Spec.Template.Spec.Volumes {
		claim := volume.PersistentVolumeClaim.ClaimName
		var task *plan.Task
		found = false
		task, found = step.FindTask(claim)
		if !found {
			continue
		}

		populatorCr := v1beta1.OvirtImageIOPopulator{}
		err = r.Client.Get(context.TODO(), client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: claim}, &populatorCr)
		if err != nil {
			return
		}

		progress, parseErr := strconv.ParseInt(populatorCr.Status.Progress, 10, 64)
		if err != nil {
			return parseErr
		}

		percent := float64(progress/0x100000) / float64(task.Progress.Total)
		task.Progress.Completed = int64(percent * float64(task.Progress.Total))
	}

	step.ReflectTasks()
	return
}

// Update the progress of the appropriate disk copy step. (DiskTransfer, Cutover)
func (r *Migration) updateCopyProgress(vm *plan.VMStatus, step *plan.Step) (err error) {
	var pendingReason string
	var pending int
	var completed int
	var running int
	var pvcs []core.PersistentVolumeClaim
	dvs, err := r.kubevirt.getDVs(vm)
	if err != nil {
		return
	}
	if dvs == nil || len(dvs) == 0 {
		pvcs, err = r.kubevirt.getPVCs(vm)
		if err != nil {
			return
		}
		for _, pvc := range pvcs {
			var task *plan.Task
			name := r.builder.ResolvePersistentVolumeClaimIdentifier(&pvc)
			found := false
			task, found = step.FindTask(name)
			if !found {
				continue
			}
			if pvc.Status.Phase == core.ClaimBound {
				completed++
				task.Phase = Completed
				task.Reason = "Transfer completed."
				task.Progress.Completed = task.Progress.Total
				task.MarkCompleted()
			}
		}
	} else {
		for _, dv := range dvs {
			var task *plan.Task
			name := r.builder.ResolveDataVolumeIdentifier(dv.DataVolume)
			found := false
			task, found = step.FindTask(name)
			if !found {
				continue
			}

			conditions := dv.Conditions()
			switch dv.Status.Phase {
			case cdi.Succeeded, cdi.Paused:
				completed++
				task.Phase = Completed
				task.Reason = "Transfer completed."
				task.Progress.Completed = task.Progress.Total
				task.MarkCompleted()
			case cdi.Pending, cdi.ImportScheduled:
				pending++
				task.Phase = Pending
				cnd := conditions.FindCondition("Bound")
				if cnd != nil && cnd.Status == True {
					cnd = conditions.FindCondition("Running")
				}
				if cnd != nil {
					pendingReason = fmt.Sprintf("%s; %s", cnd.Reason, cnd.Message)
				}
				task.Reason = pendingReason
			case cdi.ImportInProgress:
				running++
				task.Phase = Running
				task.MarkStarted()
				cnd := conditions.FindCondition("Running")
				if cnd != nil {
					task.Reason = fmt.Sprintf("%s; %s", cnd.Reason, cnd.Message)
				}
				pct := dv.PercentComplete()
				transferred := pct * float64(task.Progress.Total)
				task.Progress.Completed = int64(transferred)

				// The importer pod is recreated by CDI if it is removed for some
				// reason while the import is in progress, so we can assume that if
				// we can't find it or retrieve it for some reason that this will
				// be a transient issue, and we should be able to find it on subsequent
				// reconciles.
				var importer *core.Pod
				var found bool
				var kErr error
				if dv.PVC == nil {
					found = false
				} else {
					importer, found, kErr = r.kubevirt.GetImporterPod(*dv.PVC)
				}
				if kErr != nil {
					log.Error(
						kErr,
						"Could not get CDI importer pod for DataVolume.",
						"vm",
						vm.String(),
						"dv",
						path.Join(dv.Namespace, dv.Name))
					continue
				}

				if !found {
					log.Info(
						"Did not find CDI importer pod for DataVolume.",
						"vm",
						vm.String(),
						"dv",
						path.Join(dv.Namespace, dv.Name))
					continue
				}

				if r.Plan.Spec.Warm && len(importer.Status.ContainerStatuses) > 0 {
					vm.Warm.Failures = int(importer.Status.ContainerStatuses[0].RestartCount)
				}
				if restartLimitExceeded(importer) {
					task.MarkedCompleted()
					msg, _ := terminationMessage(importer)
					task.AddError(msg)
				}
			}
		}
	}

	step.ReflectTasks()
	if pending > 0 {
		step.Phase = Pending
		step.Reason = pendingReason
	} else if running > 0 {
		step.Phase = Running
		step.Reason = ""
	} else if (len(dvs) > 0 && completed == len(dvs)) || completed == len(pvcs) {
		step.Phase = Completed
		step.Reason = ""
	}
	return
}

// Wait for guest conversion to complete, and update the ImageConversion pipeline step.
func (r *Migration) updateConversionProgress(vm *plan.VMStatus, step *plan.Step) (err error) {
	pod, err := r.kubevirt.GetGuestConversionPod(vm)
	if err != nil {
		return
	}

	if pod != nil {
		switch pod.Status.Phase {
		case core.PodSucceeded:
			step.MarkCompleted()
			step.Progress.Completed = step.Progress.Total
		case core.PodFailed:
			step.MarkCompleted()
			step.AddError("Guest conversion failed. See pod logs for details.")
		}
	} else {
		step.MarkCompleted()
		step.AddError("Guest conversion pod not found")
	}
	return
}

func (r *Migration) setDataVolumeCheckpoints(vm *plan.VMStatus) (err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}
	var vmCr VirtualMachine
	found := false
	if vmCr, found = r.vmMap[vm.ID]; !found {
		vmCr.DataVolumes, err = r.kubevirt.getDVs(vm)
		return
	}
	dvs := make([]cdi.DataVolume, 0)
	for i := range vmCr.DataVolumes {
		dv := vmCr.DataVolumes[i].DataVolume
		dvs = append(dvs, *dv)
	}
	err = r.provider.SetCheckpoints(vm.Ref, vm.Warm.Precopies, dvs, vm.Phase == AddFinalCheckpoint)
	if err != nil {
		return
	}
	for i := range dvs {
		err = r.Destination.Client.Update(context.TODO(), &dvs[i])
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

// Step predicate.
type Predicate struct {
	// VM listed on the plan.
	vm *plan.VM
	// Plan context
	context *plancontext.Context
}

// Evaluate predicate flags.
func (r *Predicate) Evaluate(flag libitr.Flag) (allowed bool, err error) {
	switch flag {
	case HasPreHook:
		_, allowed = r.vm.FindHook(PreHook)
	case HasPostHook:
		_, allowed = r.vm.FindHook(PostHook)
	case RequiresConversion:
		allowed = r.context.Source.Provider.RequiresConversion()
	}

	return
}

// Retrieve the termination message from a pod's first container.
func terminationMessage(pod *core.Pod) (msg string, ok bool) {
	if len(pod.Status.ContainerStatuses) > 0 &&
		pod.Status.ContainerStatuses[0].LastTerminationState.Terminated != nil &&
		pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode > 0 {
		msg = pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Message
		ok = true
	}
	return
}

// Return whether the pod has failed and restarted too many times.
func restartLimitExceeded(pod *core.Pod) (exceeded bool) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return
	}
	cs := pod.Status.ContainerStatuses[0]
	exceeded = int(cs.RestartCount) > settings.Settings.ImporterRetry
	return
}
