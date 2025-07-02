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

	k8serr "k8s.io/apimachinery/pkg/api/errors"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator"
	"github.com/kubev2v/forklift/pkg/controller/plan/scheduler"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"

	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	batchv1 "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
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

const (
	TransferCompleted              = "Transfer completed."
	PopulatorPodPrefix             = "populate-"
	DvStatusCheckRetriesAnnotation = "dvStatusCheckRetries"
	// TODO: ImageConversion and DiskTransferV2v step names remain here
	// until remaining cold/warm migration flow details can be
	// moved into base migrators.
	ImageConversion = "ImageConversion"
	DiskTransferV2v = "DiskTransferV2v"
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
	// vm migrator
	migrator migrator.Migrator
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
	r.migrator, err = migrator.New(r.Context)
	if err != nil {
		return
	}

	return
}

// Begin the migration.
func (r *Migration) begin() (err error) {
	snapshot := r.Plan.Status.Migration.ActiveSnapshot()
	if snapshot.HasAnyCondition(api.ConditionExecuting, api.ConditionSucceeded, api.ConditionFailed, api.ConditionCanceled) {
		return
	}
	r.Plan.Status.Migration.MarkReset()
	r.Plan.Status.Migration.MarkStarted()
	snapshot.SetCondition(
		libcnd.Condition{
			Type:     api.ConditionExecuting,
			Status:   True,
			Category: api.CategoryAdvisory,
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
		status := r.migrator.Status(vm)
		if status.Phase != api.PhaseCompleted || status.HasAnyCondition(api.ConditionCanceled, api.ConditionFailed) {
			pipeline, pErr := r.migrator.Pipeline(vm)
			if pErr != nil {
				err = liberr.Wrap(pErr)
				return
			}
			r.migrator.Reset(status, pipeline)
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
	case api.Ova:
		if err := r.deletePvcPvForOva(); err != nil {
			r.Log.Error(err, "Failed to clean up the PVC and PV for the OVA plan")
		}
	case api.VSphere:
		if err := r.deleteValidateVddkJob(); err != nil {
			r.Log.Error(err, "Failed to clean up validate-VDDK job(s)")
		}
		if err := r.deleteConfigMap(); err != nil {
			r.Log.Error(err, "Failed to clean up vddk configmap")
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
		if vm.HasCondition(api.ConditionCanceled) && !vm.MarkedCompleted() {
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

// NextPhase transitions the VM to the next migration phase.
// If this was the last phase in the current pipeline step, the pipeline step
// is marked complete.
func (r *Migration) NextPhase(vm *plan.VMStatus) {
	currentStep, found := vm.FindStep(r.migrator.Step(vm))
	if !found {
		vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
		return
	}
	vm.Phase = r.migrator.Next(vm)
	switch vm.Phase {
	case api.PhaseCompleted:
		// `Completed` is a terminal phase that does not belong
		// to a pipeline step. If it is the next VM phase, then
		// mark the current pipeline step complete without
		// looking for a following step.
		currentStep.MarkCompleted()
		currentStep.Phase = api.StepCompleted
	default:
		nextStep, found := vm.FindStep(r.migrator.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Next step '%s' not found", r.migrator.Step(vm)))
			return
		}
		if currentStep.Name != nextStep.Name {
			currentStep.MarkCompleted()
			currentStep.Phase = api.StepCompleted
		}
	}
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
	if !vm.HasCondition(api.ConditionSucceeded) {
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
	if _, err := r.provider.RemoveSnapshot(vm.Ref, snapshot, r.kubevirt.loadHosts); err != nil {
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

func (r *Migration) deleteConfigMap() (err error) {
	selector := labels.SelectorFromSet(map[string]string{
		kPlan: string(r.Plan.UID),
		kUse:  VddkConf,
	})
	list := &core.ConfigMapList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return
	}
	for _, configmap := range list.Items {
		background := meta.DeletePropagationBackground
		opts := &client.DeleteOptions{PropagationPolicy: &background}
		err = r.Destination.Client.Delete(context.TODO(), &configmap, opts)
		if err != nil {
			r.Log.Error(err, "Failed to delete vddk-config", "configmap", configmap)
		} else {
			r.Log.Info("ConfigMap vddk-config deleted", "configmap", configmap.Name)
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

// Steps a VM through the migration itinerary
// and updates its status.
func (r *Migration) execute(vm *plan.VMStatus) (err error) {
	// check whether the VM has been canceled by the user
	if r.Context.Migration.Spec.Canceled(vm.Ref) {
		vm.SetCondition(
			libcnd.Condition{
				Type:     api.ConditionCanceled,
				Status:   True,
				Category: api.CategoryAdvisory,
				Reason:   UserRequested,
				Message:  "The migration has been canceled.",
				Durable:  true,
			})
		vm.Phase = api.PhaseCompleted
		r.Log.Info(
			"Migration [CANCELED]",
			"vm",
			vm.String())
		return
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

	// delegate to a provider-specific implementation of a phase
	// if one exists, otherwise run through the default implementation.
	ok, err := r.migrator.ExecutePhase(vm)
	if ok {
		r.Log.Info("Delegated phase implementation to migrator.", "vm", vm.String(), "phase", vm.Phase)
		if err != nil {
			r.Log.Error(err, "Delegated execution error.", "vm", vm.String(), "phase", vm.Phase)
			return
		}
	} else {
		switch vm.Phase {
		case api.PhaseStarted:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			vm.MarkStarted()
			step.MarkStarted()
			step.Phase = api.StepRunning
			err = r.cleanup(vm, func(err error) bool { return err != nil })
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}

			// Check if user provided explicit a target virtual machine name
			if vm.TargetName != "" {
				vm.NewName = vm.TargetName

				// Check if the new VM name meets DNS1123 protocol requirements
				if errs := k8svalidation.IsDNS1123Subdomain(vm.NewName); len(errs) > 0 {
					err = fmt.Errorf("VM name '%s' does not meet DNS1123 protocol requirements", vm.NewName)
					r.Log.Error(err, "Failed to update the VM name to targetName.")
					return
				}

				// Verify target VM name uniqueness in the destination namespace.
				// Return error if name exists since we do not want to mutate explicit name assignments.
				nameExist, errName := r.kubevirt.checkIfVmNameExistsInNamespace(vm.NewName, r.Plan.Spec.TargetNamespace)
				if errName != nil {
					err = liberr.Wrap(errName)
					return
				}
				if nameExist {
					err = fmt.Errorf("VM name '%s' already exists in the target namespace '%s'", vm.NewName, r.Plan.Spec.TargetNamespace)
					r.Log.Error(err, "Failed to update the VM name to targetName.")
					return
				}
			} else {
				// Check if the VM name meets DNS1123 protocol requirements
				if errs := k8svalidation.IsDNS1123Subdomain(vm.Name); len(errs) > 0 {
					vm.NewName, err = r.kubevirt.changeVmNameDNS1123(vm.Name, r.Plan.Spec.TargetNamespace)
					if err != nil {
						r.Log.Error(err, "Failed to update the VM name to meet DNS1123 protocol requirements.")
						return
					}
				}
			}
			r.NextPhase(vm)
		case api.PhasePreHook, api.PhasePostHook:
			runner := HookRunner{Context: r.Context}
			err = runner.Run(vm)
			if err != nil {
				return
			}
			if step, found := vm.FindStep(r.migrator.Step(vm)); found {
				step.Phase = api.StepRunning
				if step.MarkedCompleted() && step.Error == nil {
					r.NextPhase(vm)
				}
			} else {
				vm.Phase = api.PhaseCompleted
			}
		case api.PhaseCreateDataVolumes:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
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
			r.NextPhase(vm)
		case api.PhaseCreateVM:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			step.MarkStarted()
			step.Phase = api.StepRunning
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
			r.NextPhase(vm)
		case api.PhaseAllocateDisks, api.PhaseCopyDisks:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			step.MarkStarted()
			step.Phase = api.StepRunning

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
				r.NextPhase(vm)
			}
		case api.PhaseConvertOpenstackSnapshot:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
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
			step.Phase = api.StepRunning
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
				r.NextPhase(vm)
			}
		case api.PhaseCopyingPaused:
			if r.Migration.Spec.Cutover != nil && !r.Migration.Spec.Cutover.After(time.Now()) {
				vm.Phase = api.PhaseStorePowerState
			} else if vm.Warm.NextPrecopyAt != nil && !vm.Warm.NextPrecopyAt.After(time.Now()) {
				r.NextPhase(vm)
			}
		case api.PhaseRemovePreviousSnapshot, api.PhaseRemovePenultimateSnapshot, api.PhaseRemoveFinalSnapshot:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			n := len(vm.Warm.Precopies)
			var taskId string
			taskId, err = r.provider.RemoveSnapshot(vm.Ref, vm.Warm.Precopies[n-1].Snapshot, r.kubevirt.loadHosts)
			vm.Warm.Precopies[len(vm.Warm.Precopies)-1].RemoveTaskId = taskId
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}
			r.NextPhase(vm)
		case api.PhaseWaitForPreviousSnapshotRemoval, api.PhaseWaitForPenultimateSnapshotRemoval, api.PhaseWaitForFinalSnapshotRemoval:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			precopy := vm.Warm.Precopies[len(vm.Warm.Precopies)-1]
			ready, err := r.provider.CheckSnapshotRemove(vm.Ref, precopy, r.kubevirt.loadHosts)
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}
			if ready {
				r.NextPhase(vm)
			}
		case api.PhaseCreateInitialSnapshot, api.PhaseCreateSnapshot, api.PhaseCreateFinalSnapshot:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			var snapshot, taskId string
			if snapshot, taskId, err = r.provider.CreateSnapshot(vm.Ref, r.kubevirt.loadHosts); err != nil {
				if errors.As(err, &web.ProviderNotReadyError{}) || errors.As(err, &web.ConflictError{}) {
					return
				}
				step.AddError(err.Error())
				err = nil
				break
			}
			now := meta.Now()
			precopy := plan.Precopy{Snapshot: snapshot, CreateTaskId: taskId, Start: &now}
			vm.Warm.Precopies = append(vm.Warm.Precopies, precopy)
			r.resetPrecopyTasks(vm, step)
			r.NextPhase(vm)
		case api.PhaseWaitForInitialSnapshot, api.PhaseWaitForSnapshot, api.PhaseWaitForFinalSnapshot:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			precopy := vm.Warm.Precopies[len(vm.Warm.Precopies)-1]
			ready, snapshotId, err := r.provider.CheckSnapshotReady(vm.Ref, precopy, r.kubevirt.loadHosts)
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}
			if ready {
				if snapshotId != "" {
					vm.Warm.Precopies[len(vm.Warm.Precopies)-1].Snapshot = snapshotId
				}
				r.NextPhase(vm)
			}
		case api.PhaseWaitForDataVolumesStatus, api.PhaseWaitForFinalDataVolumesStatus:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}

			dvs, err := r.kubevirt.getDVs(vm)
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}
			if !r.hasPausedDv(dvs) {
				r.NextPhase(vm)
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
						r.NextPhase(vm)
						// Reset for next precopy
						step.Annotations[DvStatusCheckRetriesAnnotation] = "1"
					} else {
						step.Annotations[DvStatusCheckRetriesAnnotation] = strconv.Itoa(retries + 1)
					}
				}
			}
		case api.PhaseStoreInitialSnapshotDeltas, api.PhaseStoreSnapshotDeltas:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
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
			r.NextPhase(vm)
		case api.PhaseAddCheckpoint, api.PhaseAddFinalCheckpoint:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}

			err = r.setDataVolumeCheckpoints(vm)
			if err != nil {
				step.AddError(err.Error())
				err = nil
				break
			}

			switch vm.Phase {
			case api.PhaseAddCheckpoint:
				vm.Phase = api.PhaseWaitForDataVolumesStatus
			case api.PhaseAddFinalCheckpoint:
				vm.Phase = api.PhaseWaitForFinalDataVolumesStatus
			}
		case api.PhaseStorePowerState:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
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
			r.NextPhase(vm)
		case api.PhasePowerOffSource:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
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
			r.NextPhase(vm)
		case api.PhaseWaitForPowerOff:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
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
				r.NextPhase(vm)
			}
		case api.PhaseFinalize:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			err = r.updateCopyProgress(vm, step)
			if err != nil {
				return
			}
			if step.MarkedCompleted() {
				if !step.HasError() {
					r.NextPhase(vm)
				}
			}
		case api.PhaseCreateGuestConversionPod:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			step.MarkStarted()
			step.Phase = api.StepRunning
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
			r.NextPhase(vm)
		case api.PhaseConvertGuest, api.PhaseCopyDisksVirtV2V:
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
				break
			}
			step.MarkStarted()
			step.Phase = api.StepRunning

			err = r.updateConversionProgress(vm, step)
			if err != nil {
				return
			}

			switch r.Source.Provider.Type() {
			case api.Ova, api.VSphere:
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
				r.NextPhase(vm)
			}
		case api.PhaseCompleted:
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
			vm.Phase = api.PhaseCompleted
		}
	}
	vm.ReflectPipeline()
	if vm.Phase == api.PhaseCompleted && vm.Error == nil {
		err = r.provider.DetachDisks(vm.Ref)
		if err != nil {
			step, found := vm.FindStep(r.migrator.Step(vm))
			if !found {
				vm.AddError(fmt.Sprintf("Step '%s' not found", r.migrator.Step(vm)))
			}
			step.AddError(err.Error())
			r.Log.Error(err,
				"Could not detach LUN disk(s) from the source VM.",
				"vm",
				vm.String())
			err = nil
			return
		}
		// Delete pod if user specified that they want to remove it after successful migration.
		if r.Plan.Spec.DeleteGuestConversionPod {
			r.Log.Info("Removing guest conversion pod for finished VM.", "vm", vm.String())
			err = r.kubevirt.DeleteGuestConversionPod(vm)
			if err != nil {
				r.Log.Error(
					err,
					"Could not remove guest conversion pod for finished VM.",
					"vm",
					vm.String(),
				)
				err = nil
			}
		}
		vm.SetCondition(
			libcnd.Condition{
				Type:     api.ConditionSucceeded,
				Status:   True,
				Category: api.CategoryAdvisory,
				Message:  "The VM migration has SUCCEEDED.",
				Durable:  true,
			})

	} else if vm.Error != nil {
		vm.Phase = api.PhaseCompleted
		vm.SetCondition(
			libcnd.Condition{
				Type:     api.ConditionFailed,
				Status:   True,
				Category: api.CategoryAdvisory,
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

// End the migration.
func (r *Migration) end() (completed bool, err error) {
	failed := 0
	succeeded := 0
	for _, vm := range r.Plan.Status.Migration.VMs {
		if !vm.MarkedCompleted() {
			return
		}
		if vm.HasCondition(api.ConditionFailed) {
			failed++
		}
		if vm.HasCondition(api.ConditionSucceeded) {
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
				Type:     api.ConditionFailed,
				Status:   True,
				Category: api.CategoryAdvisory,
				Message:  "The plan execution has FAILED.",
				Durable:  true,
			})
	} else if succeeded > 0 {
		// if the migration didn't fail and at least one VM succeeded,
		// then the migration succeeded.
		r.Log.Info("Migration [SUCCEEDED]")
		snapshot.SetCondition(
			libcnd.Condition{
				Type:     api.ConditionSucceeded,
				Status:   True,
				Category: api.CategoryAdvisory,
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
				Type:     api.ConditionCanceled,
				Status:   True,
				Category: api.CategoryAdvisory,
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
		vmCr.VirtualMachine, err = r.kubevirt.virtualMachine(vm, true)
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
	case api.Ova:
		ready, err = r.kubevirt.EnsureOVAVirtV2VPVCStatus(vm.ID)
	case api.VSphere:
		ready = true
	}

	return
}

func (r *Migration) setTaskCompleted(task *plan.Task) {
	task.Phase = api.StepCompleted
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
				task.Phase = api.StepPending
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
				task.Phase = api.StepRunning
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
		step.Phase = api.StepPending
		step.Reason = pendingReason
	} else if running > 0 {
		step.Phase = api.StepRunning
		step.Reason = ""
	} else if (len(dvs) > 0 && completed == len(dvs)) || completed == len(pvcs) {
		step.Phase = api.StepCompleted
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

		useV2vForTransfer, err := r.Context.Plan.ShouldUseV2vForTransfer()
		switch {
		case err != nil:
			return liberr.Wrap(err)
		case useV2vForTransfer:
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
	if step.Name == ImageConversion && someProgress && r.Source.Provider.Type() != api.Ova {
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
	err = r.provider.SetCheckpoints(vm.Ref, vm.Warm.Precopies, dvs, vm.Phase == api.PhaseAddFinalCheckpoint, r.kubevirt.loadHosts)
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
			task.Phase = api.StepCompleted
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
