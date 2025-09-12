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

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/scheduler"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"

	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"

	"github.com/konveyor/forklift-controller/pkg/settings"
	batchv1 "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// VirtV2vDiskCopy         libitr.Flag = 0x10
	OvaImageMigration       libitr.Flag = 0x10
	OpenstackImageMigration libitr.Flag = 0x20
	VSphere                 libitr.Flag = 0x40
)

// Phases.
const (
	Started                       = "Started"
	PreHook                       = "PreHook"
	StorePowerState               = "StorePowerState"
	PowerOffSource                = "PowerOffSource"
	WaitForPowerOff               = "WaitForPowerOff"
	CreateDataVolumes             = "CreateDataVolumes"
	WaitForDataVolumesStatus      = "WaitForDataVolumesStatus"
	WaitForFinalDataVolumesStatus = "WaitForFinalDataVolumesStatus"
	CreateVM                      = "CreateVM"
	CopyDisks                     = "CopyDisks"
	// AllocateDisks                     = "AllocateDisks"
	CopyingPaused                     = "CopyingPaused"
	AddCheckpoint                     = "AddCheckpoint"
	AddFinalCheckpoint                = "AddFinalCheckpoint"
	CreateSnapshot                    = "CreateSnapshot"
	CreateInitialSnapshot             = "CreateInitialSnapshot"
	CreateFinalSnapshot               = "CreateFinalSnapshot"
	Finalize                          = "Finalize"
	CreateGuestConversionPod          = "CreateGuestConversionPod"
	ConvertGuest                      = "ConvertGuest"
	CopyDisksVirtV2V                  = "CopyDisksVirtV2V"
	PostHook                          = "PostHook"
	Completed                         = "Completed"
	WaitForSnapshot                   = "WaitForSnapshot"
	WaitForInitialSnapshot            = "WaitForInitialSnapshot"
	WaitForFinalSnapshot              = "WaitForFinalSnapshot"
	ConvertOpenstackSnapshot          = "ConvertOpenstackSnapshot"
	StoreSnapshotDeltas               = "StoreSnapshotDeltas"
	StoreInitialSnapshotDeltas        = "StoreInitialSnapshotDeltas"
	RemovePreviousSnapshot            = "RemovePreviousSnapshot"
	RemovePenultimateSnapshot         = "RemovePenultimateSnapshot"
	RemoveFinalSnapshot               = "RemoveFinalSnapshot"
	WaitForFinalSnapshotRemoval       = "WaitForFinalSnapshotRemoval"
	WaitForPreviousSnapshotRemoval    = "WaitForPreviousSnapshotRemoval"
	WaitForPenultimateSnapshotRemoval = "WaitForPenultimateSnapshotRemoval"
)

// Steps.
const (
	Initialize = "Initialize"
	Cutover    = "Cutover"
	// DiskAllocation  = "DiskAllocation"
	DiskTransfer    = "DiskTransfer"
	ImageConversion = "ImageConversion"
	DiskTransferV2v = "DiskTransferV2v"
	VMCreation      = "VirtualMachineCreation"
	Unknown         = "Unknown"
)

const (
	TransferCompleted              = "Transfer completed."
	PopulatorPodPrefix             = "populate-"
	DvStatusCheckRetriesAnnotation = "dvStatusCheckRetries"
	SnapshotRemovalCheckRetries    = "snapshotRemovalCheckRetries"
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
			// {Name: AllocateDisks, All: VirtV2vDiskCopy},
			{Name: CreateGuestConversionPod, All: RequiresConversion},
			{Name: ConvertGuest, All: RequiresConversion},
			// {Name: CopyDisksVirtV2V, All: RequiresConversion},
			{Name: CopyDisksVirtV2V, All: OvaImageMigration},
			{Name: ConvertOpenstackSnapshot, All: OpenstackImageMigration},
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
			{Name: StoreInitialSnapshotDeltas, All: VSphere},
			{Name: CreateDataVolumes},
			{Name: WaitForDataVolumesStatus},
			{Name: CopyDisks},
			{Name: CopyingPaused},
			{Name: RemovePreviousSnapshot, All: VSphere},
			{Name: WaitForPreviousSnapshotRemoval, All: VSphere},
			{Name: CreateSnapshot},
			{Name: WaitForSnapshot},
			{Name: StoreSnapshotDeltas, All: VSphere},
			{Name: AddCheckpoint},
			{Name: StorePowerState},
			{Name: PowerOffSource},
			{Name: WaitForPowerOff},
			{Name: RemovePenultimateSnapshot, All: VSphere},
			{Name: WaitForPenultimateSnapshotRemoval, All: VSphere},
			{Name: CreateFinalSnapshot},
			{Name: WaitForFinalSnapshot},
			{Name: AddFinalCheckpoint},
			{Name: WaitForFinalDataVolumesStatus},
			{Name: Finalize},
			{Name: RemoveFinalSnapshot, All: VSphere},
			{Name: WaitForFinalSnapshotRemoval, All: VSphere},
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
	// pvc converter
	converter *adapter.Converter
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
	for {
		var hasNext bool
		var vm *plan.VMStatus
		vm, hasNext, err = r.scheduler.Next()
		if err != nil {
			return
		}
		if hasNext {
			err = r.execute(vm)
			if err != nil {
				return
			}
		} else {
			r.Log.Info("The scheduler does not have any additional VMs.")
			break
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
	err = r.kubevirt.EnsureExtraV2vConfConfigMap()
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
	defer func() {
		if r.provider != nil {
			r.provider.Close()
		}
	}()
	if err := r.init(); err != nil {
		r.Log.Error(err, "Archive initialization failed.")
		return
	}

	switch r.Plan.Provider.Source.Type() {
	case v1beta1.Ova:
		if err := r.deletePvcPvForOva(); err != nil {
			r.Log.Error(err, "Failed to clean up the PVC and PV for the OVA plan")
		}
	case v1beta1.VSphere:
		if err := r.deleteValidateVddkJob(); err != nil {
			r.Log.Error(err, "Failed to clean up validate-VDDK job(s)")
		}
	}

	for _, vm := range r.Plan.Status.Migration.VMs {
		dontFailOnError := func(err error) bool {
			if err != nil {
				r.Log.Error(liberr.Wrap(err),
					"Couldn't clean up VM while archiving plan.",
					"vm",
					vm.String())
			}
			return false
		}
		_ = r.cleanup(vm, dontFailOnError)
	}
}

func (r *Migration) SetPopulatorDataSourceLabels() {
	defer func() {
		if r.provider != nil {
			r.provider.Close()
		}
	}()
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
func (r *Migration) Cancel() error {
	defer func() {
		if r.provider != nil {
			r.provider.Close()
		}
	}()
	if err := r.init(); err != nil {
		return liberr.Wrap(err)
	}

	for _, vm := range r.Plan.Status.Migration.VMs {
		if vm.HasCondition(Canceled) {
			dontFailOnError := func(err error) bool {
				if err != nil {
					r.Log.Error(liberr.Wrap(err),
						"Couldn't clean up after canceled VM migration.",
						"vm",
						vm.String())
				}
				return false
			}
			_ = r.cleanup(vm, dontFailOnError)
			if vm.RestorePowerState == plan.VMPowerStateOn {
				if err := r.provider.PowerOn(vm.Ref); err != nil {
					r.Log.Error(err,
						"Couldn't restore the power state of the source VM.",
						"vm",
						vm.String())
				}
			}
			vm.MarkCompleted()
			markStartedStepsCompleted(vm)
		}
	}

	return nil
}

func markStartedStepsCompleted(vm *plan.VMStatus) {
	for _, step := range vm.Pipeline {
		if step.MarkedStarted() {
			step.MarkCompleted()
		}
	}
}

func (r *Migration) deletePopulatorPVCs(vm *plan.VMStatus) (err error) {
	if r.builder.SupportsVolumePopulators() {
		err = r.kubevirt.DeletePopulatedPVCs(vm)
	}
	return
}

// Delete left over migration resources associated with a VM.
func (r *Migration) cleanup(vm *plan.VMStatus, failOnErr func(error) bool) error {
	if !vm.HasCondition(Succeeded) {
		if err := r.kubevirt.DeleteVM(vm); failOnErr(err) {
			return err
		}
		if err := r.deletePopulatorPVCs(vm); failOnErr(err) {
			return err
		}
		if err := r.kubevirt.DeleteDataVolumes(vm); failOnErr(err) {
			return err
		}
	}
	if err := r.deleteImporterPods(vm); failOnErr(err) {
		return err
	}
	if err := r.kubevirt.DeletePVCConsumerPod(vm); failOnErr(err) {
		return err
	}
	if err := r.kubevirt.DeleteGuestConversionPod(vm); failOnErr(err) {
		return err
	}
	if err := r.kubevirt.DeleteSecret(vm); failOnErr(err) {
		return err
	}
	if err := r.kubevirt.DeleteConfigMap(vm); failOnErr(err) {
		return err
	}
	if err := r.kubevirt.DeleteHookJobs(vm); failOnErr(err) {
		return err
	}
	if r.Plan.Provider.Destination.IsHost() {
		if err := r.destinationClient.DeletePopulatorDataSource(vm); failOnErr(err) {
			return err
		}
	}
	if err := r.kubevirt.DeletePopulatorPods(vm); failOnErr(err) {
		return err
	}
	if err := r.kubevirt.DeleteJobs(vm); failOnErr(err) {
		return err
	}

	r.removeLastWarmSnapshot(vm)

	return nil
}

func (r *Migration) removeLastWarmSnapshot(vm *plan.VMStatus) {
	if vm.Warm == nil {
		return
	}
	n := len(vm.Warm.Precopies)
	if n < 1 {
		return
	}
	snapshot := vm.Warm.Precopies[n-1].Snapshot
	if err := r.provider.RemoveSnapshot(vm.Ref, snapshot, r.kubevirt.loadHosts); err != nil {
		r.Log.Error(
			err,
			"Failed to clean up warm migration snapshots.",
			"vm", vm)
	}
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
		err = r.kubevirt.DeleteImporterPods(pvc)
		if err != nil {
			return
		}
	}
	return
}

func (r *Migration) deletePvcPvForOva() (err error) {
	pvcs, _, err := GetOvaPvcListNfs(r.Destination.Client, r.Plan.Name, r.Plan.Spec.TargetNamespace)
	if err != nil {
		r.Log.Error(err, "Failed to get the plan PVCs")
		return
	}
	// The PVCs was already deleted
	if len(pvcs.Items) == 0 {
		return
	}

	for _, pvc := range pvcs.Items {
		err = r.Destination.Client.Delete(context.TODO(), &pvc)
		if err != nil {
			r.Log.Error(err, "Failed to delete the plan PVC", pvc)
			return
		}
	}

	pvs, _, err := GetOvaPvListNfs(r.Destination.Client, string(r.Plan.UID))
	if err != nil {
		r.Log.Error(err, "Failed to get the plan PVs")
		return
	}
	// The PVs was already deleted
	if len(pvs.Items) == 0 {
		return
	}

	for _, pv := range pvs.Items {
		err = r.Destination.Client.Delete(context.TODO(), &pv)
		if err != nil {
			r.Log.Error(err, "Failed to delete the plan PV", pv)
			return
		}
	}
	return
}

func (r *Migration) deleteValidateVddkJob() (err error) {
	selector := labels.SelectorFromSet(map[string]string{"plan": string(r.Plan.UID)})
	jobs := &batchv1.JobList{}
	err = r.Destination.Client.List(
		context.TODO(),
		jobs,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return
	}
	for _, job := range jobs.Items {
		background := meta.DeletePropagationBackground
		opts := &client.DeleteOptions{PropagationPolicy: &background}
		err = r.Destination.Client.Delete(context.TODO(), &job, opts)
		if err != nil {
			r.Log.Error(err, "Failed to delete validate-vddk job", "job", job)
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
	case Started, CreateInitialSnapshot, WaitForInitialSnapshot, StoreInitialSnapshotDeltas, CreateDataVolumes:
		step = Initialize
	// case AllocateDisks:
	// 	step = DiskAllocation
	case CopyDisks, CopyingPaused, RemovePreviousSnapshot, WaitForPreviousSnapshotRemoval, CreateSnapshot, WaitForSnapshot, StoreSnapshotDeltas, AddCheckpoint, ConvertOpenstackSnapshot, WaitForDataVolumesStatus:
		step = DiskTransfer
	case RemovePenultimateSnapshot, WaitForPenultimateSnapshotRemoval, CreateFinalSnapshot, WaitForFinalSnapshot, AddFinalCheckpoint, Finalize, RemoveFinalSnapshot, WaitForFinalSnapshotRemoval, WaitForFinalDataVolumesStatus:
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
		err = r.cleanup(vm, func(err error) bool { return err != nil })
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		if errs := k8svalidation.IsDNS1123Label(vm.Name); len(errs) > 0 {
			vm.NewName, err = r.kubevirt.changeVmNameDNS1123(vm.Name, r.Plan.Spec.TargetNamespace)
			if err != nil {
				r.Log.Error(err, "Failed to update the VM name to meet DNS1123 protocol requirements.")
				return
			}
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
			var pvcs []*core.PersistentVolumeClaim
			if pvcs, err = r.kubevirt.PopulatorVolumes(vm.Ref); err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error creating volumes", "vm", vm.Name)
					step.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
			err = r.kubevirt.EnsurePopulatorVolumes(vm, pvcs)
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
					r.Log.Error(err, "error creating volumes", "vm", vm.Name)
					step.AddError(err.Error())
					err = nil
					break
				} else {
					return
				}
			}
			if vm.Warm != nil {
				err = r.provider.SetCheckpoints(vm.Ref, vm.Warm.Precopies, dataVolumes, false, r.kubevirt.loadHosts)
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
		if r.Plan.Provider.Destination.IsHost() {
			err = r.destinationClient.SetPopulatorCrOwnership()
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
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
		err = r.deleteImporterPods(vm)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.next(vm.Phase)
	// case AllocateDisks, CopyDisks:
	case CopyDisks:
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
	case ConvertOpenstackSnapshot:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}

		if r.converter == nil {
			labels := map[string]string{
				"plan":      string(r.Plan.GetUID()),
				"migration": string(r.Context.Migration.UID),
				"vmID":      vm.ID,
				"app":       "forklift",
			}
			r.converter = adapter.NewConverter(&r.Context.Destination, r.Log.WithName("converter"), labels)
			r.converter.FilterFn = func(pvc *core.PersistentVolumeClaim) bool {
				val, ok := pvc.Annotations[base.AnnRequiresConversion]
				return ok && val == "true"
			}
		}

		step.MarkStarted()
		step.Phase = Running
		pvcs, err := r.kubevirt.getPVCs(vm.Ref)
		if err != nil {
			r.Log.Error(err,
				"Couldn't get VM's PVCs.",
				"vm",
				vm.String())
			break
		}

		srcFormatFn := func(pvc *core.PersistentVolumeClaim) string {
			return pvc.Annotations[base.AnnSourceFormat]
		}

		ready, err := r.converter.ConvertPVCs(pvcs, srcFormatFn, "raw")
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}

		if !ready {
			r.Log.Info("Conversion isn't ready yet")
			return nil
		}

		if step.MarkedCompleted() && !step.HasError() {
			step.Phase = Completed
			vm.Phase = r.next(vm.Phase)
		}
	case CopyingPaused:
		if r.Migration.Spec.Cutover != nil && !r.Migration.Spec.Cutover.After(time.Now()) {
			vm.Phase = StorePowerState
		} else if vm.Warm.NextPrecopyAt != nil && !vm.Warm.NextPrecopyAt.After(time.Now()) {
			vm.Phase = r.next(vm.Phase)
		}
	case RemovePreviousSnapshot, RemovePenultimateSnapshot, RemoveFinalSnapshot:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		n := len(vm.Warm.Precopies)
		err = r.provider.RemoveSnapshot(vm.Ref, vm.Warm.Precopies[n-1].Snapshot, r.kubevirt.loadHosts)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		vm.Phase = r.next(vm.Phase)
	case WaitForPreviousSnapshotRemoval, WaitForPenultimateSnapshotRemoval, WaitForFinalSnapshotRemoval:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		// FIXME: This is just temporary timeout to unblock the migrations which get stuck on issue https://issues.redhat.com/browse/MTV-1753
		// This should be fixed properly by adding the task manager inside the inventory and monitor the task status
		// from the main controller.
		var retries int
		retriesAnnotation := step.Annotations[SnapshotRemovalCheckRetries]
		if retriesAnnotation == "" {
			step.Annotations[SnapshotRemovalCheckRetries] = "1"
		} else {
			retries, err = strconv.Atoi(retriesAnnotation)
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}
			if retries >= settings.Settings.SnapshotRemovalCheckRetries {
				vm.Phase = r.next(vm.Phase)
				// Reset for next precopy
				step.Annotations[SnapshotRemovalCheckRetries] = "1"
			} else {
				step.Annotations[SnapshotRemovalCheckRetries] = strconv.Itoa(retries + 1)
			}
		}
	case CreateInitialSnapshot, CreateSnapshot, CreateFinalSnapshot:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		var snapshot string
		if snapshot, err = r.provider.CreateSnapshot(vm.Ref, r.kubevirt.loadHosts); err != nil {
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
	case WaitForDataVolumesStatus, WaitForFinalDataVolumesStatus:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}

		dvs, err := r.kubevirt.getDVs(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		if !r.hasPausedDv(dvs) {
			vm.Phase = r.next(vm.Phase)
			// Reset for next precopy
			step.Annotations[DvStatusCheckRetriesAnnotation] = "1"
		} else {
			var retries int
			retriesAnnotation := step.Annotations[DvStatusCheckRetriesAnnotation]
			if retriesAnnotation == "" {
				step.Annotations[DvStatusCheckRetriesAnnotation] = "1"
			} else {
				retries, err = strconv.Atoi(retriesAnnotation)
				if err != nil {
					step.AddError(err.Error())
					err = nil
					break
				}
				if retries >= settings.Settings.DvStatusCheckRetries {
					// Do not fail the step as this can happen when the user runs the warm migration but the VM is already shutdown
					// In that case we don't create any delta and don't change the CDI DV status.
					r.Log.Info(
						"DataVolume status check exceeded the retry limit."+
							"If this causes the problems with the snapshot removal in the CDI please bump the controller_dv_status_check_retries.",
						"vm",
						vm.String())
					vm.Phase = r.next(vm.Phase)
					// Reset for next precopy
					step.Annotations[DvStatusCheckRetriesAnnotation] = "1"
				} else {
					step.Annotations[DvStatusCheckRetriesAnnotation] = strconv.Itoa(retries + 1)
				}
			}
		}
	case StoreInitialSnapshotDeltas, StoreSnapshotDeltas:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}

		n := len(vm.Warm.Precopies)
		snapshot := vm.Warm.Precopies[n-1].Snapshot
		var deltas map[string]string
		deltas, err = r.provider.GetSnapshotDeltas(vm.Ref, snapshot, r.kubevirt.loadHosts)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		vm.Warm.Precopies[n-1].WithDeltas(deltas)
		vm.Phase = r.next(vm.Phase)
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
			vm.Phase = WaitForDataVolumesStatus
		case AddFinalCheckpoint:
			vm.Phase = WaitForFinalDataVolumesStatus
		}
	case StorePowerState:
		step, found := vm.FindStep(r.step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.step(vm)))
			break
		}
		var state plan.VMPowerState
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
		var ready bool
		if ready, err = r.ensureGuestConversionPod(vm); err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		if !ready {
			r.Log.Info("virt-v2v pod isn't ready yet")
			return
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

		switch r.Source.Provider.Type() {
		case v1beta1.Ova, v1beta1.VSphere:
			// fetch config from the conversion pod
			pod, err := r.kubevirt.GetGuestConversionPod(vm)
			if err != nil {
				return err
			}

			if pod != nil && pod.Status.Phase == core.PodRunning {
				err := r.kubevirt.UpdateVmByConvertedConfig(vm, pod, step)
				if err != nil {
					return liberr.Wrap(err)
				}
			}
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

func (r *Migration) hasPausedDv(dvs []ExtendedDataVolume) bool {
	for _, dv := range dvs {
		if dv.Status.Phase == Paused {
			return true
		}
	}
	return false
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
		// case AllocateDisks, CopyDisks, CopyDisksVirtV2V, ConvertOpenstackSnapshot:
		case CopyDisks, CopyDisksVirtV2V, ConvertOpenstackSnapshot:
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
			// case AllocateDisks:
			// 	task_name = DiskAllocation
			// 	task_description = "Allocate disks."
			case CopyDisksVirtV2V:
				task_name = DiskTransferV2v
				task_description = "Copy disks."
			case ConvertOpenstackSnapshot:
				task_name = ConvertOpenstackSnapshot
				task_description = "Convert OpenStack snapshot."
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
func (r *Migration) ensureGuestConversionPod(vm *plan.VMStatus) (ready bool, err error) {
	if r.vmMap == nil {
		r.vmMap, err = r.kubevirt.VirtualMachineMap()
		if err != nil {
			return
		}
	}
	var vmCr VirtualMachine
	var pvcs []*core.PersistentVolumeClaim
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

	err = r.kubevirt.EnsureGuestConversionPod(vm, &vmCr, pvcs)
	if err != nil {
		return
	}

	switch r.Source.Provider.Type() {
	case v1beta1.Ova:
		ready, err = r.kubevirt.EnsureOVAVirtV2VPVCStatus(vm.ID)
	case v1beta1.VSphere:
		ready = true
	}

	return
}

func (r *Migration) setTaskCompleted(task *plan.Task) {
	task.Phase = Completed
	task.Reason = TransferCompleted
	task.Progress.Completed = task.Progress.Total
	task.MarkCompleted()
}

// Update the progress of the appropriate disk copy step. (DiskTransfer, Cutover)
func (r *Migration) updateCopyProgress(vm *plan.VMStatus, step *plan.Step) (err error) {
	var pendingReason string
	var pending int
	var completed int
	var running int
	var pvcs []*core.PersistentVolumeClaim
	dvs, err := r.kubevirt.getDVs(vm)
	if err != nil {
		return
	}
	if len(dvs) == 0 {
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
			name := r.builder.ResolvePersistentVolumeClaimIdentifier(pvc)
			found := false
			task, found = step.FindTask(name)
			if !found {
				continue
			}
			if pvc.Status.Phase == core.ClaimBound {
				completed++
				r.setTaskCompleted(task)
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
			if dv.Status.Phase == cdi.PendingPopulation && r.Source.Provider.RequiresConversion() {
				// in migrations that involve conversion, the conversion pod serves as the
				// first consumer of the PVCs so we can treat PendingPopulation as Succeeded
				dv.Status.Phase = cdi.Succeeded
			}
			conditions := dv.Conditions()
			switch dv.Status.Phase {
			case cdi.Succeeded, cdi.Paused:
				completed++
				r.setTaskCompleted(task)
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
					err = r.Destination.Client.Get(context.TODO(), types.NamespacedName{
						Namespace: r.Plan.Spec.TargetNamespace,
						Name:      fmt.Sprintf("prime-%s", pvc.UID),
					}, pvc)
					if err != nil {
						if k8serr.IsNotFound(err) {
							log.Info("Could not find prime PVC")
							// Ignore error
							err = nil
						} else {
							log.Error(
								err,
								"Could not get prime PVC for DataVolume.",
								"vm",
								vm.String(),
								"dv",
								path.Join(dv.Namespace, dv.Name))
							continue
						}
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
func (r *Migration) updateConversionProgress(vm *plan.VMStatus, step *plan.Step) error {
	pod, err := r.kubevirt.GetGuestConversionPod(vm)
	switch {
	case err != nil:
		return liberr.Wrap(err)
	case pod == nil:
		step.MarkCompleted()
		step.AddError("Guest conversion pod not found")
		return nil
	}

	switch pod.Status.Phase {
	case core.PodSucceeded:
		step.MarkCompleted()
		step.Progress.Completed = step.Progress.Total
	case core.PodFailed:
		step.MarkCompleted()
		step.AddError("Guest conversion failed. See pod logs for details.")
	default:
		if pod.Status.PodIP == "" {
			// we get the progress from the pod and we cannot connect to the pod without PodIP
			break
		}

		// coldLocal, err := r.Context.Plan.VSphereColdLocal()
		// switch {
		// case err != nil:
		// 	return liberr.Wrap(err)
		// case coldLocal:
		if r.Context.Plan.IsSourceProviderOVA() {
			if err := r.updateConversionProgressV2vMonitor(pod, step); err != nil {
				// Just log it. Missing progress is not fatal.
				log.Error(err, "Failed to update conversion progress")
			}
		}
	}

	return nil
}

func (r *Migration) updateConversionProgressV2vMonitor(pod *core.Pod, step *plan.Step) (err error) {
	var diskRegex = regexp.MustCompile(`v2v_disk_transfers\{disk_id="(\d+)"\} (\d{1,3}\.?\d*)`)
	url := fmt.Sprintf("http://%s:2112/metrics", pod.Status.PodIP)
	resp, err := http.Get(url)
	switch {
	case err == nil:
		defer resp.Body.Close()
	case strings.Contains(err.Error(), "connection refused"):
		return nil
	default:
		return
	}

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
	if step.Name == ImageConversion && someProgress && r.Source.Provider.Type() != v1beta1.Ova {
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
	err = r.provider.SetCheckpoints(vm.Ref, vm.Warm.Precopies, dvs, vm.Phase == AddFinalCheckpoint, r.kubevirt.loadHosts)
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
		taskName, err = r.builder.GetPopulatorTaskName(pvc)
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
		transferredBytes, err = r.builder.PopulatorTransferredBytes(pvc)
		if err != nil {
			return
		}

		percent := float64(transferredBytes/0x100000) / float64(task.Progress.Total)
		newProgress := int64(percent * float64(task.Progress.Total))
		if newProgress == task.Progress.Completed {
			pvcId := string(pvc.UID)
			populatorFailed := r.isPopulatorPodFailed(pvcId)
			if populatorFailed {
				return fmt.Errorf("populator pod failed for PVC %s. Please check the pod logs", pvcId)
			}
		}
		task.Progress.Completed = newProgress
	}

	step.ReflectTasks()
	return
}

// Checks if the populator pod failed when the progress didn't change
func (r *Migration) isPopulatorPodFailed(givenPvcId string) bool {
	populatorPods, err := r.kubevirt.getPopulatorPods()
	if err != nil {
		r.Log.Error(err, "couldn't get the populator pods")
		return false
	}
	for _, pod := range populatorPods {
		pvcId := pod.Name[len(PopulatorPodPrefix):]
		if givenPvcId != pvcId {
			continue
		}
		if pod.Status.Phase == core.PodFailed {
			return true
		}
		break
	}
	return false
}

func (r *Migration) setPopulatorPodsWithLabels(vm *plan.VMStatus, migrationID string) {
	podList, err := r.kubevirt.GetPodsWithLabels(map[string]string{})
	if err != nil {
		return
	}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, PopulatorPodPrefix) {
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
	// coldLocal, vErr := r.context.Plan.VSphereColdLocal()
	// if vErr != nil {
	// 	err = vErr
	// 	return
	// }

	switch flag {
	case HasPreHook:
		_, allowed = r.vm.FindHook(PreHook)
	case HasPostHook:
		_, allowed = r.vm.FindHook(PostHook)
	case RequiresConversion:
		allowed = r.context.Source.Provider.RequiresConversion()
	case OvaImageMigration:
		allowed = r.context.Plan.IsSourceProviderOVA()
	case CDIDiskCopy:
		// allowed = !coldLocal
		allowed = !r.context.Plan.IsSourceProviderOVA()
	// case VirtV2vDiskCopy:
	// 	allowed = coldLocal
	case OpenstackImageMigration:
		allowed = r.context.Plan.IsSourceProviderOpenstack()
	case VSphere:
		allowed = r.context.Plan.IsSourceProviderVSphere()
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
