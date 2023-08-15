package plan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	"github.com/konveyor/forklift-controller/pkg/settings"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
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
	CDIDiskCopy        libitr.Flag = 0x08
	VirtV2vDiskCopy    libitr.Flag = 0x10
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
	AllocateDisks            = "AllocateDisks"
	CopyingPaused            = "CopyingPaused"
	AddCheckpoint            = "AddCheckpoint"
	AddFinalCheckpoint       = "AddFinalCheckpoint"
	CreateSnapshot           = "CreateSnapshot"
	CreateInitialSnapshot    = "CreateInitialSnapshot"
	CreateFinalSnapshot      = "CreateFinalSnapshot"
	Finalize                 = "Finalize"
	CreateGuestConversionPod = "CreateGuestConversionPod"
	ConvertGuest             = "ConvertGuest"
	CopyDisksVirtV2V         = "CopyDisksVirtV2V"
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
	DiskAllocation  = "DiskAllocation"
	DiskTransfer    = "DiskTransfer"
	ImageConversion = "ImageConversion"
	DiskTransferV2v = "DiskTransferV2v"
	VMCreation      = "VirtualMachineCreation"
	Unknown         = "Unknown"
)

// Power states.
const (
	On = "On"
)

const (
	TransferCompleted = "Transfer completed."
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
			{Name: CopyDisks, All: CDIDiskCopy},
			{Name: AllocateDisks, All: VirtV2vDiskCopy},
			{Name: CreateGuestConversionPod, All: RequiresConversion},
			{Name: ConvertGuest, All: RequiresConversion},
			{Name: CopyDisksVirtV2V, All: RequiresConversion},
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
	// destination client.
	destinationClient adapter.DestinationClient
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
	r.destinationClient, err = adapter.DestinationClient(r.Context)
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

func (r *Migration) SetPopulatorDataSourceLabels() {
	err := r.init()
	if err != nil {
		r.Log.Error(err, "Setting Populator Data Source labels failed.")
		return
	}

	for _, vm := range r.Plan.Status.Migration.VMs {
		pvcs, err := r.kubevirt.getPVCs(vm.Ref)
		if err != nil {
			r.Log.Error(err,
				"Couldn't get VM's PVCs.",
				"vm",
				vm.String())
		}
		err = r.builder.SetPopulatorDataSourceLabels(vm.Ref, pvcs)
		if err != nil {
			r.Log.Error(err, "Couldn't set the labels.", "vm", vm.String())
		}
		migrationID := string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)
		// populator pods
		r.setPopulatorPodsWithLabels(vm, migrationID)
	}
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
	if !vm.HasCondition(Succeeded) {
		err = r.kubevirt.DeleteVM(vm)
		if err != nil {
			return
		}
	}
	err = r.deleteImporterPods(vm)
	if err != nil {
		return
	}
	err = r.kubevirt.DeletePVCConsumerPod(vm)
	if err != nil {
		return
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
	err = r.destinationClient.DeletePopulatorDataSource(vm)
	if err != nil {
		return
	}
	err = r.kubevirt.DeletePopulatorPods(vm)
	if err != nil {
		return
	}
	if vm.Warm != nil {
		if errLocal := r.provider.RemoveSnapshots(vm.Ref, vm.Warm.Precopies); errLocal != nil {
			r.Log.Error(
				errLocal,
				"Failed to clean up warm migration snapshots.",
				"vm", vm)
		}
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
	pvcs, err := r.kubevirt.getPVCs(vm.Ref)
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
	r.Log.Info("Itinerary transition", "current phase", phase, "next phase", next)

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
	case AllocateDisks:
		step = DiskAllocation
	case CopyDisks, CopyingPaused, CreateSnapshot, WaitForSnapshot, AddCheckpoint:
		step = DiskTransfer
	case CreateFinalSnapshot, WaitForFinalSnapshot, AddFinalCheckpoint, Finalize:
		step = Cutover
	case CreateGuestConversionPod, ConvertGuest:
		step = ImageConversion
	case CopyDisksVirtV2V:
		step = DiskTransferV2v
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
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}

		var ready bool
		ready, err = r.provider.PreTransferActions(vm.Ref)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
				break
			} else {
				return
			}
		}

		if r.builder.SupportsVolumePopulators() {
			var pvcNames []string
			pvcNames, err = r.kubevirt.PopulatorVolumes(vm.Ref)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					step.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
			err = r.kubevirt.EnsurePopulatorVolumes(vm, pvcNames)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					step.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
		}

		if !ready {
			r.Log.Info("PreTransferActions hook isn't ready yet")
			return
		}

		if !r.builder.SupportsVolumePopulators() {
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
		// set ownership to populator Crs
		err = r.destinationClient.SetPopulatorCrOwnership()
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		// set ownership to populator pods
		err = r.kubevirt.SetPopulatorPodOwnership(vm)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		// Removing unnecessary DataVolumes
		err = r.kubevirt.DeleteDataVolumes(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		err = r.kubevirt.DeletePVCConsumerPod(vm)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.next(vm.Phase)
	case AllocateDisks, CopyDisks:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		step.MarkStarted()
		step.Phase = Running

		if r.builder.SupportsVolumePopulators() {
			err = r.updatePopulatorCopyProgress(vm, step)
		} else {
			// Fallback to non-volume populator path
			err = r.updateCopyProgress(vm, step)
		}
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
			if errors.As(err, &web.ProviderNotReadyError{}) || errors.As(err, &web.ConflictError{}) {
				return
			}
			step.AddError(err.Error())
			err = nil
			break
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
	case ConvertGuest, CopyDisksVirtV2V:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		step.MarkStarted()
		step.Phase = Running
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
		err = r.provider.DetachDisks(vm.Ref)
		if err != nil {
			step, found := vm.FindStep(r.step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			}
			step.AddError(err.Error())
			r.Log.Error(err,
				"Could not detach LUN disk(s) from the source VM.",
				"vm",
				vm.String())
			err = nil
			return
		}
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
		case AllocateDisks, CopyDisks, CopyDisksVirtV2V:
			tasks, pErr := r.builder.Tasks(vm.Ref)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			total := int64(0)
			for _, task := range tasks {
				total += task.Progress.Total
			}
			var task_description, task_name string
			switch step.Name {
			case CopyDisks:
				task_name = DiskTransfer
				task_description = "Transfer disks."
			case AllocateDisks:
				task_name = DiskAllocation
				task_description = "Allocate disks."
			case CopyDisksVirtV2V:
				task_name = DiskTransferV2v
				task_description = "Copy disks."
			default:
				err = liberr.New(fmt.Sprintf("Unknown step '%s'. Not implemented.", step.Name))
				return
			}
			pipeline = append(
				pipeline,
				&plan.Step{
					Task: plan.Task{
						Name:        task_name,
						Description: task_description,
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
	go r.provider.Finalize(r.Plan.Status.Migration.VMs, r.Migration.Name)
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
		pvcs, err = r.kubevirt.getPVCs(vm.Ref)
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
		// Recreate the map and check again, the map may be stale
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}

		if vmCr, found = r.vmMap[vm.ID]; !found {
			msg := "VirtualMachine CR not found."
			vm.AddError(msg)
			return
		}
	}

	if vmCr.Spec.Running != nil && *vmCr.Spec.Running == running {
		return
	}

	err = r.kubevirt.SetRunning(&vmCr, running)
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
		pvcs, err = r.kubevirt.getPVCs(vm.Ref)
		if err != nil {
			return
		}
		for _, pvc := range pvcs {
			if _, ok := pvc.Annotations["lun"]; ok {
				// skip LUNs
				continue
			}
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
				task.Reason = TransferCompleted
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
				task.Reason = TransferCompleted
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

				if dv.Status.ClaimName == "" {
					found = false
				} else {
					pvc := &core.PersistentVolumeClaim{}
					err = r.Destination.Client.Get(context.TODO(), types.NamespacedName{
						Namespace: r.Plan.Spec.TargetNamespace,
						Name:      dv.Status.ClaimName,
					}, pvc)
					if err != nil {
						log.Error(
							err,
							"Could not get PVC for DataVolume.",
							"vm",
							vm.String(),
							"dv",
							path.Join(dv.Namespace, dv.Name))
						continue
					}

					importer, found, kErr = r.kubevirt.GetImporterPod(*pvc)
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
				if RestartLimitExceeded(importer) {
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
		default:
			el9, el9Err := r.Context.Plan.VSphereUsesEl9VirtV2v()
			if el9Err != nil {
				err = el9Err
				return
			}
			if el9 {
				err = r.updateConversionProgressEl9(pod, step)
				if err != nil {
					// Just log it. Missing progress is not fatal.
					log.Error(err, "Failed to update conversion progress")
					err = nil
					return
				}
			}
		}
	} else {
		step.MarkCompleted()
		step.AddError("Guest conversion pod not found")
	}
	return
}

func (r *Migration) updateConversionProgressEl9(pod *core.Pod, step *plan.Step) (err error) {
	if pod.Status.PodIP == "" {
		return
	}

	var diskRegex = regexp.MustCompile(`v2v_disk_transfers\{disk_id="(\d+)"\} (\d{1,3}\.?\d*)`)
	url := fmt.Sprintf("http://%s:2112/metrics", pod.Status.PodIP)
	resp, err := http.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil
		}
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	matches := diskRegex.FindAllStringSubmatch(string(body), -1)
	if matches == nil {
		return
	}
	someProgress := false
	for _, match := range matches {
		diskNumber, _ := strconv.ParseUint(string(match[1]), 10, 0)
		progress, _ := strconv.ParseFloat(string(match[2]), 64)
		r.Log.Info("Progress update", "disk", diskNumber, "progress", progress, "tasks", step.Tasks)
		if progress > 100 {
			r.Log.Info("Progress seems out of range", "progress", progress)
			progress = 100
		}

		someProgress = someProgress || progress > 0
		if diskNumber > uint64(len(step.Tasks)) {
			r.Log.Info("Ignoring progress update", "disk", diskNumber, "disks count", len(step.Tasks), "step", step.Name)
			continue
		}
		task := step.Tasks[diskNumber-1]
		if step.Name == DiskTransferV2v {
			// Update copy progress if we're in CopyDisksVirtV2V step.
			task.Progress.Completed = int64(float64(task.Progress.Total) * progress / 100)
		}
	}
	step.ReflectTasks()
	if step.Name == ImageConversion && someProgress {
		// Disk copying has already started. Transition from
		// ConvertGuest to CopyDisksVirtV2V .
		step.MarkCompleted()
		step.Progress.Completed = step.Progress.Total
		return
	}
	return
}

func (r *Migration) setDataVolumeCheckpoints(vm *plan.VMStatus) (err error) {
	disks, err := r.kubevirt.getDVs(vm)
	if err != nil {
		return
	}
	dvs := make([]cdi.DataVolume, 0)
	for _, disk := range disks {
		dvs = append(dvs, *disk.DataVolume)
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

func (r *Migration) updatePopulatorCopyProgress(vm *plan.VMStatus, step *plan.Step) (err error) {
	pvcs, err := r.kubevirt.getPVCs(vm.Ref)
	if err != nil {
		return
	}

	for _, pvc := range pvcs {
		if _, ok := pvc.Annotations["lun"]; ok {
			// skip LUNs
			continue
		}
		var task *plan.Task
		var taskName string
		taskName, err = r.builder.GetPopulatorTaskName(&pvc)
		if err != nil {
			return
		}

		found := false
		if task, found = step.FindTask(taskName); !found {
			continue
		}

		if pvc.Status.Phase == core.ClaimBound {
			task.Phase = Completed
			task.Reason = TransferCompleted
			task.Progress.Completed = task.Progress.Total
			task.MarkCompleted()
			continue
		}

		var transferredBytes int64
		transferredBytes, err = r.builder.PopulatorTransferredBytes(&pvc)
		if err != nil {
			return
		}

		percent := float64(transferredBytes/0x100000) / float64(task.Progress.Total)
		task.Progress.Completed = int64(percent * float64(task.Progress.Total))
	}

	step.ReflectTasks()
	return
}

func (r *Migration) setPopulatorPodsWithLabels(vm *plan.VMStatus, migrationID string) {
	podList, err := r.kubevirt.GetPodsWithLabels(map[string]string{})
	if err != nil {
		return
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "populate-") {
			// it's populator pod
			if _, ok := pod.Labels["migration"]; !ok {
				// un-labeled pod, we need to set it
				err = r.kubevirt.setPopulatorPodLabels(pod, migrationID)
				if err != nil {
					r.Log.Error(err, "couldn't update the Populator pod labels.", "vm", vm.String(), "migration", migrationID, "pod", pod.Name)
					continue
				}
			}
		}
	}
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
	el9, el9Err := r.context.Plan.VSphereUsesEl9VirtV2v()
	if el9Err != nil {
		err = el9Err
		return
	}
	switch flag {
	case HasPreHook:
		_, allowed = r.vm.FindHook(PreHook)
	case HasPostHook:
		_, allowed = r.vm.FindHook(PostHook)
	case RequiresConversion:
		allowed = r.context.Source.Provider.RequiresConversion()
	case CDIDiskCopy:
		allowed = !el9
	case VirtV2vDiskCopy:
		allowed = el9
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
func RestartLimitExceeded(pod *core.Pod) (exceeded bool) {
	if len(pod.Status.ContainerStatuses) == 0 {
		return
	}
	cs := pod.Status.ContainerStatuses[0]
	exceeded = int(cs.RestartCount) > settings.Settings.ImporterRetry
	return
}
