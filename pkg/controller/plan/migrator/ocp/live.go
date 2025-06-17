package ocp

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/migrator/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
	kubevirtapi "kubevirt.io/api/instancetype"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	multicluster "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

const (
	KubeVirtNamespace        = "kubevirtNamespace"
	AnnDiskSource            = "forklift.konveyor.io/disk-source"
	AnnVolumeName            = "forklift.konveyor.io/volume"
	AnnSource                = "forklift.konveyor.io/source"
	AnnRunStrategy           = "forklift.konveyor.io/run-strategy"
	AnnBindImmediate         = "cdi.kubevirt.io/storage.bind.immediate.requested"
	AnnDeleteAfterCompletion = "cdi.kubevirt.io/storage.deleteAfterCompletion"
)

// Phases
const (
	Started                                = "Started"
	PreHook                                = "PreHook"
	CreateServiceExports                   = "CreateServiceExports"
	SynchronizeCertificates                = "SynchronizeCertificates"
	CreateSecrets                          = "CreateSecrets"
	CreateConfigMaps                       = "CreateConfigMaps"
	EnsurePreference                       = "EnsurePreference"
	EnsureInstanceType                     = "EnsureInstanceType"
	CreateTarget                           = "CreateTarget"
	CreateVirtualMachineInstanceMigrations = "CreateVirtualMachineInstanceMigrations"
	WaitForStateTransfer                   = "WaitForStateTransfer"
	SyncRunStrategy                        = "SyncRunStrategy"
	PostHook                               = "PostHook"
	Completed                              = "Completed"
)

// Pipeline
const (
	PrepareTarget   = "PrepareTarget"
	Synchronization = "Synchronization"
)

// Conditions
const (
	Running = "Running"
)

func New(context *plancontext.Context) (migrator base.Migrator, err error) {
	switch context.Plan.Spec.Type {
	case api.MigrationLive:
		m := LiveMigrator{Context: context}
		err = m.Init()
		if err != nil {
			return
		}
		migrator = &m
	default:
		m := base.BaseMigrator{Context: context}
		err = m.Init()
		if err != nil {
			return
		}
		migrator = &m
	}
	context.Log.Info("Built OCP migrator.", "type", context.Plan.Spec.Type)
	return
}

type LiveMigrator struct {
	*plancontext.Context
	builder      Builder
	ensurer      Ensurer
	sourceClient client.Client
}

func (r *LiveMigrator) Init() (err error) {
	r.sourceClient, err = K8sClient(r.Context.Source.Provider)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.builder = Builder{
		Context:      r.Context,
		sourceClient: r.sourceClient,
	}
	r.ensurer = Ensurer{Context: r.Context}
	return
}

func (r *LiveMigrator) Cleanup(vm *planapi.VMStatus, successful bool) (err error) {
	err = r.DeleteTargetVMIM(vm)
	if err != nil {
		return
	}
	err = r.DeleteSourceVMIM(vm)
	if err != nil {
		return
	}
	err = r.DeleteServiceExports(vm)
	if err != nil {
		return
	}
	return
}

func (r *LiveMigrator) Status(vm planapi.VM) (status *planapi.VMStatus) {
	if current, found := r.Context.Plan.Status.Migration.FindVM(vm.Ref); !found {
		status = &planapi.VMStatus{VM: vm}
		if r.Context.Plan.Spec.Warm {
			status.Warm = &planapi.Warm{}
		}
	} else {
		status = current
	}
	return
}

func (r *LiveMigrator) Reset(status *planapi.VMStatus, pipeline []*planapi.Step) {
	status.DeleteCondition(base.Canceled, base.Failed)
	status.MarkReset()
	itr := r.Itinerary()
	step, _ := itr.First()
	status.Phase = step.Name
	status.Pipeline = pipeline
	status.Error = nil
	status.Warm = nil
	return
}

func (r *LiveMigrator) Itinerary() (itinerary *libitr.Itinerary) {
	itinerary = &libitr.Itinerary{
		Name: "ocp-live",
		Pipeline: libitr.Pipeline{
			{Name: Started},
			{Name: PreHook, All: FlagPreHook},
			{Name: SynchronizeCertificates},
			{Name: CreateSecrets},
			{Name: CreateConfigMaps},
			{Name: EnsurePreference},
			{Name: EnsureInstanceType},
			{Name: CreateTarget},
			{Name: CreateServiceExports, All: FlagSubmariner},
			{Name: CreateVirtualMachineInstanceMigrations},
			{Name: WaitForStateTransfer},
			{Name: SyncRunStrategy},
			{Name: PostHook, All: FlagPostHook},
			{Name: Completed},
		},
	}
	return
}

func (r *LiveMigrator) Next(status *planapi.VMStatus) (next string) {
	itinerary := r.Itinerary()
	step, done, err := itinerary.Next(status.Phase)
	if done || err != nil {
		next = Completed
		if err != nil {
			r.Log.Error(err, "Next phase failed.")
		}
	} else {
		next = step.Name
	}
	r.Log.Info("Itinerary transition", "current phase", status.Phase, "next phase", next)
	return
}

func (r *LiveMigrator) Step(status *planapi.VMStatus) (step string) {
	switch status.Phase {
	case Started:
		step = base.Initialize
	case PreHook, PostHook:
		step = status.Phase
	case CreateSecrets, CreateConfigMaps, EnsurePreference, EnsureInstanceType, CreateTarget, SynchronizeCertificates, CreateServiceExports:
		step = PrepareTarget
	case CreateVirtualMachineInstanceMigrations, WaitForStateTransfer, SyncRunStrategy:
		step = Synchronization
	default:
		step = base.Unknown
	}
	return
}

func (r *LiveMigrator) Pipeline(vm planapi.VM) (pipeline []*planapi.Step, err error) {
	itinerary := r.Itinerary()
	step, _ := itinerary.First()
	for {
		switch step.Name {
		case Started:
			pipeline = append(
				pipeline,
				&planapi.Step{
					Task: planapi.Task{
						Name:        base.Initialize,
						Description: "Initialize migration.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       base.Pending,
					},
				})
		case PreHook:
			pipeline = append(
				pipeline,
				&planapi.Step{
					Task: planapi.Task{
						Name:        PreHook,
						Description: "Run pre-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       base.Pending,
					},
				})
		case PostHook:
			pipeline = append(
				pipeline,
				&planapi.Step{
					Task: planapi.Task{
						Name:        PostHook,
						Description: "Run post-migration hook.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       base.Pending,
					},
				})
		case CreateSecrets:
			pipeline = append(
				pipeline,
				&planapi.Step{
					Task: planapi.Task{
						Name:        PrepareTarget,
						Description: "Prepare target namespace.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       base.Pending,
					},
				})
		case CreateVirtualMachineInstanceMigrations:
			pipeline = append(
				pipeline,
				&planapi.Step{
					Task: planapi.Task{
						Name:        Synchronization,
						Description: "Synchronize source and target VMs.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       base.Pending,
					},
				})
		}
		next, done, _ := itinerary.Next(step.Name)
		if !done {
			step = next
		} else {
			break
		}
	}

	r.Log.V(2).Info(
		"Pipeline built.",
		"vm",
		vm.String())
	return
}

// TODO: make logging consistent
func (r *LiveMigrator) ExecutePhase(vm *planapi.VMStatus) (ok bool, err error) {
	ok = true
	switch vm.Phase {
	case Started:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		vm.MarkedStarted()
		step.Phase = Completed
		vm.Phase = r.Next(vm)
	case PreHook, PostHook:
		// delegate to common pipeline
		return
	case SynchronizeCertificates:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		step.MarkedStarted()
		step.Phase = Running
		vm.Phase = r.Next(vm)
	case CreateSecrets:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var secrets []core.Secret
		secrets, err = r.builder.Secrets(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building secrets", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		err = r.ensurer.EnsureSecrets(vm, secrets)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error ensuring secrets", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		vm.Phase = r.Next(vm)
	case CreateConfigMaps:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var configmaps []core.ConfigMap
		configmaps, err = r.builder.ConfigMaps(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building configmaps", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		err = r.ensurer.EnsureConfigMaps(vm, configmaps)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error ensuring configmaps", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		vm.Phase = r.Next(vm)
	case EnsurePreference:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var required bool
		required, err = r.RequiresLocalPreference(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error checking for Preference", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		if !required {
			var preference *instancetype.VirtualMachinePreference
			preference, err = r.builder.LocalPreference(vm)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error building Preference", "vm", vm.Name)
					step.AddError(err.Error())
					err = nil
				}
				break
			}
			err = r.ensurer.EnsureLocalPreference(vm, preference)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error ensuring Preference", "vm", vm.Name)
					step.AddError(err.Error())
					err = nil
				}
				break
			}
		}
		vm.Phase = r.Next(vm)
	case EnsureInstanceType:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var required bool
		required, err = r.RequiresLocalInstanceType(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error checking for InstanceType", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		if !required {
			var instancetype *instancetype.VirtualMachineInstancetype
			instancetype, err = r.builder.LocalInstanceType(vm)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error building InstanceType", "vm", vm.Name)
					step.AddError(err.Error())
					err = nil
				}
				break
			}
			err = r.ensurer.EnsureLocalInstanceType(vm, instancetype)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error ensuring InstanceType", "vm", vm.Name)
					step.AddError(err.Error())
					err = nil
				}
				break
			}
		}
		vm.Phase = r.Next(vm)
	case CreateTarget:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var dataVolumes []cdi.DataVolume
		dataVolumes, err = r.builder.DataVolumes(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building volumes", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		err = r.ensurer.EnsureDataVolumes(vm, dataVolumes)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		var target *cnv.VirtualMachine
		target, err = r.builder.VirtualMachine(vm, dataVolumes)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
		}
		err = r.ensurer.EnsureVirtualMachine(vm, target)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		vm.Phase = r.Next(vm)
	case CreateServiceExports:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var target *cnv.VirtualMachine
		target, err = r.GetTargetVM(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		err = r.EnsureServiceExports(vm, target)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.Next(vm)
	case CreateVirtualMachineInstanceMigrations:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		step.MarkStarted()
		step.Phase = Running
		var target *cnv.VirtualMachineInstanceMigration
		target, err = r.EnsureTargetMigration(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		if !r.TargetVMIMReady(target) {
			return
		}
		err = r.EnsureSourceMigration(vm, target)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		vm.Phase = r.Next(vm)
	case WaitForStateTransfer:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var done bool
		done, err = r.WaitForStateTransfer(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		if !done {
			return
		}
		vm.Phase = r.Next(vm)
	case SyncRunStrategy:
		step, found := vm.FindStep(r.Step(vm))
		if !found {
			vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
			return
		}
		var target *cnv.VirtualMachine
		target, err = r.GetTargetVM(vm)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		err = r.SyncRunStrategy(target)
		if err != nil {
			step.AddError(err.Error())
			err = nil
			break
		}
		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.Next(vm)
	default:
		ok = false
		r.Log.Info(
			"Phase unknown, defer to base migrator.",
			"vm",
			vm,
			"phase",
			vm.Phase)
	}
	return
}

func (r *LiveMigrator) SyncRunStrategy(vm *cnv.VirtualMachine) (err error) {
	runStrategy := cnv.RunStrategyAlways
	storedStrategy, ok := vm.Annotations[AnnRunStrategy]
	if ok && storedStrategy != "" {
		runStrategy = cnv.VirtualMachineRunStrategy(storedStrategy)
	}
	vm.Spec.RunStrategy = &runStrategy
	err = r.Client.Update(context.TODO(), vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

// TODO: improve error messages
func (r *LiveMigrator) WaitForStateTransfer(vm *planapi.VMStatus) (done bool, err error) {
	source, found, err := r.GetSourceVMIM(vm)
	if err != nil {
		return
	}
	if !found {
		err = liberr.New("Source VMIM not found.")
		return
	}
	target, found, err := r.GetTargetVMIM(vm)
	if err != nil {
		return
	}
	if !found {
		err = liberr.New("Target VMIM not found.")
		return
	}
	if (source.Status.Phase == cnv.MigrationFailed) || (target.Status.Phase == cnv.MigrationFailed) {
		err = liberr.New("Migration failed, check VMIM status for details.")
		return
	}
	if (source.Status.Phase == cnv.MigrationSucceeded) && (target.Status.Phase == cnv.MigrationSucceeded) {
		done = true
	}
	return
}

func (r *LiveMigrator) GetTargetVM(vm *planapi.VMStatus) (target *cnv.VirtualMachine, err error) {
	vms := &cnv.VirtualMachineList{}
	err = r.Context.Destination.Client.List(
		context.TODO(),
		vms,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Context.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(vms.Items) == 0 {
		err = liberr.New("Target VM not found.", "source", vm.String())
		return
	} else {
		target = &vms.Items[0]
		return
	}
}

// TODO:
func (r *LiveMigrator) RequiresLocalPreference(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Preference != nil && virtualMachine.Object.Spec.Preference.Kind != kubevirtapi.ClusterSingularPreferenceResourceName
	return
}

// TODO:
func (r *LiveMigrator) RequiresClusterPreference(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Preference != nil && virtualMachine.Object.Spec.Preference.Kind == kubevirtapi.ClusterSingularPreferenceResourceName
	return
}

// TODO:
func (r *LiveMigrator) RequiresLocalInstanceType(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Instancetype != nil && virtualMachine.Object.Spec.Instancetype.Kind != kubevirtapi.ClusterSingularResourceName
	return
}

// TODO:
func (r *LiveMigrator) RequiresClusterInstanceType(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Instancetype != nil && virtualMachine.Object.Spec.Instancetype.Kind == kubevirtapi.ClusterSingularResourceName
	return
}

func (r *LiveMigrator) GetTargetVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, found bool, err error) {
	vmims := &cnv.VirtualMachineInstanceMigrationList{}
	err = r.Context.Destination.Client.List(
		context.TODO(),
		vmims,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Context.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(vmims.Items) == 0 {
		return
	} else {
		vmim = &vmims.Items[0]
		found = true
	}
	return
}

func (r *LiveMigrator) GetSourceVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, found bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	vmims := &cnv.VirtualMachineInstanceMigrationList{}
	err = r.sourceClient.List(
		context.TODO(),
		vmims,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     virtualMachine.Namespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(vmims.Items) == 0 {
		return
	} else {
		vmim = &vmims.Items[0]
		found = true
	}
	return
}

// TODO: move into ensurer, refactor to use new pattern (receive object to ensure as parameter)
func (r *LiveMigrator) EnsureTargetMigration(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
	vmim, found, err := r.GetTargetVMIM(vm)
	if err != nil {
		return
	}
	if !found {
		vmim, err = r.targetVMIM(vm)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		err = r.Context.Destination.Client.Create(context.TODO(), vmim)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created target VirtualMachineInstanceMigration.",
			"vmim",
			path.Join(vmim.Namespace, vmim.Name),
			"vm",
			vm.String())
	}

	return
}

// TODO: move into ensurer, refactor to use new pattern (receive object to ensure as parameter)
func (r *LiveMigrator) EnsureSourceMigration(vm *planapi.VMStatus, target *cnv.VirtualMachineInstanceMigration) (err error) {
	vmim, found, err := r.GetSourceVMIM(vm)
	if err != nil {
		return
	}

	if !found {
		vmim, err = r.sourceVMIM(vm, target)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		err = r.sourceClient.Create(context.TODO(), vmim)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created source VirtualMachineInstanceMigration.",
			"vmim",
			path.Join(vmim.Namespace, vmim.Name),
			"vm",
			vm.String())
	}

	return
}

// TODO: move into ensurer, clean up
func (r *LiveMigrator) EnsureServiceExports(vm *planapi.VMStatus, target *cnv.VirtualMachine) (err error) {
	for _, kind := range []string{"sync", "migration"} {
		export := &multicluster.ServiceExport{}
		namespace := r.Context.Destination.Provider.Spec.Settings[KubeVirtNamespace]
		name := fmt.Sprintf("%s-%s-%s", kind, target.Name, target.Namespace)
		key := types.NamespacedName{Namespace: namespace, Name: name}
		err = r.Context.Destination.Client.Get(context.TODO(), key, export)
		if err != nil {
			if k8serr.IsNotFound(err) {
				export.Name = name
				export.Namespace = namespace
				export.Labels = r.Labeler.VMLabels(vm.Ref)
				err = r.Context.Destination.Client.Create(context.TODO(), export)
				if err != nil {
					err = liberr.Wrap(err)
					return
				}
			} else {
				err = liberr.Wrap(err)
				return
			}
		}
	}
	return
}

func (r *LiveMigrator) TargetVMIMReady(vmim *cnv.VirtualMachineInstanceMigration) (ready bool) {
	ready = vmim.Status.SynchronizationAddress != nil && *vmim.Status.SynchronizationAddress != ""
	return
}

// TODO: move into builder
func (r *LiveMigrator) targetVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
	target, err := r.GetTargetVM(vm)
	if err != nil {
		return
	}

	vmim = &cnv.VirtualMachineInstanceMigration{}
	vmim.GenerateName = fmt.Sprintf("forklift-")
	vmim.Namespace = r.Context.Plan.Spec.TargetNamespace
	vmim.Labels = r.Labeler.VMLabels(vm.Ref)
	vmim.Spec.Receive.MigrationID = r.KubevirtMigrationID(vm)
	vmim.Spec.VMIName = target.Name
	return
}

// TODO: move into builder
func (r *LiveMigrator) sourceVMIM(vm *planapi.VMStatus, target *cnv.VirtualMachineInstanceMigration) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
	inventoryVm := &model.VM{}
	err = r.Context.Source.Inventory.Find(inventoryVm, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	vmim = &cnv.VirtualMachineInstanceMigration{}
	vmim.GenerateName = fmt.Sprintf("forklift-")
	vmim.Namespace = inventoryVm.Namespace
	vmim.Labels = r.Labeler.VMLabels(vm.Ref)
	vmim.Spec.VMIName = inventoryVm.Name
	vmim.Spec.SendTo.MigrationID = r.KubevirtMigrationID(vm)
	if target.Status.SynchronizationAddress != nil {
		vmim.Spec.SendTo.ConnectURL = *target.Status.SynchronizationAddress
	}
	return
}

func (r *LiveMigrator) KubevirtMigrationID(vm *planapi.VMStatus) string {
	return fmt.Sprintf("%s-%s", vm.ID, r.Migration.UID)
}

// DeleteServiceExports deletes the ServiceExports that were created to expose the sync endpooints on
// the destination cluster.
func (r *LiveMigrator) DeleteServiceExports(vm *planapi.VMStatus) (err error) {
	err = r.Destination.Client.DeleteAllOf(
		context.Background(),
		&multicluster.ServiceExport{},
		&client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
				// TODO: find a better way to determine where Kubevirt is installed on the destination.
				Namespace: r.Context.Destination.Provider.Spec.Settings[KubeVirtNamespace],
			},
		})
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

// DeleteSourceVMIM deletes the VMIM resource from the source cluster.
func (r *LiveMigrator) DeleteSourceVMIM(vm *planapi.VMStatus) (err error) {
	inventoryVm := &model.VM{}
	err = r.Context.Source.Inventory.Find(inventoryVm, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	err = r.sourceClient.DeleteAllOf(
		context.Background(),
		&cnv.VirtualMachineInstanceMigration{},
		&client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
				Namespace:     inventoryVm.Namespace,
			},
		})
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

// DeleteTargetVMIM deletes the VMIM resource from the target cluster.
func (r *LiveMigrator) DeleteTargetVMIM(vm *planapi.VMStatus) (err error) {
	err = r.Destination.Client.DeleteAllOf(
		context.Background(),
		&cnv.VirtualMachineInstanceMigration{},
		&client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
				Namespace:     r.Plan.Spec.TargetNamespace,
			},
		})
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

// Ensurer has the limited responsibility of ensuring resources
// are present in the destination cluster and namespace.
type Ensurer struct {
	*plancontext.Context
}

// EnsureDataVolumes have been created on the destination cluster. Although we build DataVolumes with the same
// names they had on the source cluster, we search by label so that we notice conflicts with existing DVs.
func (r *Ensurer) EnsureDataVolumes(vm *planapi.VMStatus, dvs []cdi.DataVolume) (err error) {
	list := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	exists := make(map[string]bool)
	for _, dv := range list.Items {
		exists[dv.Annotations[AnnDiskSource]] = true
	}

	for _, dv := range dvs {
		if !exists[dv.Annotations[AnnDiskSource]] {
			err = r.Destination.Client.Create(context.TODO(), &dv)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			r.Log.Info("Created DataVolume.",
				"dv",
				path.Join(
					dv.Namespace,
					dv.Name),
				"vm",
				vm.String())
		}
	}
	return
}

// EnsureConfigMaps exist in the destination cluster's target namespace. We attempt to create ConfigMaps
// with the same name that they have on the source cluster because they are likely to be shared between
// multiple VMs. If one with a matching name already exists, we assume it's the intended ConfigMap for
// the VM to mount.
// TODO: consider raising a concern at the VM or plan level if a configmap with the desired
// name already exists but does not have the annotation indicating that Forklift created it.
func (r *Ensurer) EnsureConfigMaps(vm *planapi.VMStatus, configMaps []core.ConfigMap) (err error) {
	for _, configMap := range configMaps {
		err = r.Destination.Client.Create(context.Background(), &configMap)
		if err != nil {
			if k8serr.IsAlreadyExists(err) {
				_, found := configMap.Annotations[AnnSource]
				if !found {
					r.Log.Info("Matching ConfigMap already present in destination namespace.", "configMap",
						path.Join(
							configMap.Namespace,
							configMap.Name),
						"forklift-created", false)
				}
				continue
			}
			err = liberr.Wrap(err, "Failed to create ConfigMap.", "configMap",
				path.Join(
					configMap.Namespace,
					configMap.Name))
			return
		}
		r.Log.Info("Created ConfigMap.",
			"configMap",
			path.Join(
				configMap.Namespace,
				configMap.Name),
			"vm",
			vm.String())
	}
	return
}

// EnsureSecrets exist in the destination cluster's target namespace. We attempt to create Secrets
// with the same name that they have on the source cluster because they are likely to be shared between
// multiple VMs. If one with a matching name already exists, we assume it's the intended Secret for
// the VM to mount.
// TODO: consider raising a concern at the VM or plan level if a secret with the desired
// name already exists but does not have the annotation indicating that Forklift created it.
func (r *Ensurer) EnsureSecrets(vm *planapi.VMStatus, secrets []core.Secret) (err error) {
	for _, secret := range secrets {
		err = r.Destination.Client.Create(context.Background(), &secret)
		if err != nil {
			if k8serr.IsAlreadyExists(err) {
				_, found := secret.Annotations[AnnSource]
				if !found {
					r.Log.Info("Matching Secret already present in destination namespace.", "secret",
						path.Join(
							secret.Namespace,
							secret.Name),
						"forklift-created", false)
				}
				continue
			}
			err = liberr.Wrap(err, "Failed to create Secret.", "secret",
				path.Join(
					secret.Namespace,
					secret.Name))
			return
		}
		r.Log.Info("Created Secret.",
			"secret",
			path.Join(
				secret.Namespace,
				secret.Name),
			"vm",
			vm.String())
	}
	return
}

// EnsureLocalPreference ensures that the target local Preference has been created in the destination cluster.
// If one already exists, we assume it's the intended preference to use.
func (r *Ensurer) EnsureLocalPreference(vm *planapi.VMStatus, target *instancetype.VirtualMachinePreference) (err error) {
	err = r.Destination.Client.Create(context.Background(), target)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			_, found := target.Annotations[AnnSource]
			if !found {
				r.Log.Info("Matching local VirtualMachinePreference already present in destination namespace.", "preference",
					path.Join(
						target.Namespace,
						target.Name),
					"forklift-created", false)
			}
			return
		}
		err = liberr.Wrap(err, "Failed to create VirtualMachinePreference.", "preference",
			path.Join(
				target.Namespace,
				target.Name))
		return
	}
	r.Log.Info("Created VirtualMachinePreference.",
		"preference",
		path.Join(
			target.Namespace,
			target.Name),
		"vm",
		vm.String())
	return
}

// EnsureLocalInstanceType ensures that the target local InstanceType has been created in the destination cluster.
// If one already exists, we assume it's the intended preference to use.
func (r *Ensurer) EnsureLocalInstanceType(vm *planapi.VMStatus, target *instancetype.VirtualMachineInstancetype) (err error) {
	err = r.Destination.Client.Create(context.Background(), target)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			_, found := target.Annotations[AnnSource]
			if !found {
				r.Log.Info("Matching local VirtualMachineInstancetype already present in destination namespace.", "instancetype",
					path.Join(
						target.Namespace,
						target.Name),
					"forklift-created", false)
			}
			return
		}
		err = liberr.Wrap(err, "Failed to create VirtualMachineInstancetype.", "instancetype",
			path.Join(
				target.Namespace,
				target.Name))
		return
	}
	r.Log.Info("Created VirtualMachineInstancetype.",
		"instancetype",
		path.Join(
			target.Namespace,
			target.Name),
		"vm",
		vm.String())
	return
}

// EnsureVirtualMachine ensures that the target VirtualMachine has been created in the destination cluster.
// Labels are used to search for the VM in order to be sure that Forklift is what created it.
func (r *Ensurer) EnsureVirtualMachine(vm *planapi.VMStatus, target *cnv.VirtualMachine) (err error) {
	vms := &cnv.VirtualMachineList{}
	err = r.Destination.Client.List(
		context.TODO(),
		vms,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(vms.Items) == 0 {
		err = r.Destination.Client.Create(context.TODO(), target)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created destination VM.",
			"vm",
			path.Join(
				target.Namespace,
				target.Name),
			"source",
			vm.String())
	} else {
		target = &vms.Items[0]
	}
	return
}

type Builder struct {
	*plancontext.Context
	sourceClient client.Client
}

// VirtualMachine builds a cnv.VirtualMachine resource that is a copy of the source VirtualMachine with
// its resources remapped to ones present on the destination. It is important that all DataVolumes are built
// and applied to the destination before the VirtualMachine is so that KubeVirt doesn't try to act on
// any DataVolumeTemplates.
// We assume that DataVolumes, PVCs, Secrets, ConfigMaps, etc on the destination are created with the
// same names as on the source so that they stay consistent, especially across multiple migrations.
// This means that the volume mappings on the target VM spec do not need to change.
func (r *Builder) VirtualMachine(vm *planapi.VMStatus, dvs []cdi.DataVolume) (obj *cnv.VirtualMachine, err error) {
	source := &model.VM{}
	err = r.Source.Inventory.Find(source, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	halted := cnv.RunStrategyHalted
	object := &cnv.VirtualMachine{
		Spec: cnv.VirtualMachineSpec{
			DataVolumeTemplates: source.Object.Spec.DataVolumeTemplates,
			// TODO: decide how to handle missing instance types and preferences.
			// The least painful option is probably to assume that any instance type
			// or preference with the same name is the same and use it, and if there
			// isn't one with the same name then create it. This situation exists with
			// secrets and configmaps as well, and like that situations it could result
			// in failed migrations if the CRs are actually different.
			// We don't want to create our own with different names, because that will
			// result in the VM spec changing every time the VM is migrated between clusters.
			//
			// If the VM refers to cluster scope preferences and instance types rather than
			// local scope, we should probably not attempt to create missing ones.
			Instancetype: source.Object.Spec.Instancetype,
			Preference:   source.Object.Spec.Preference,
			Template:     source.Object.Spec.Template.DeepCopy(),
			Running:      nil,
			RunStrategy:  &halted,
		},
	}
	key := types.NamespacedName{Namespace: vm.Namespace, Name: source.Name}
	object.Name = source.Name
	object.Namespace = r.Plan.Spec.TargetNamespace
	r.Labeler.SetLabels(object, r.Labeler.VMLabels(vm.Ref))
	r.Labeler.SetAnnotations(object, r.Labeler.VMLabels(vm.Ref))
	r.Labeler.SetAnnotation(object, AnnSource, key.String())

	// preserve the original runstrategy so that it can be applied
	// once the migration is complete.
	runStrategy, _ := source.Object.RunStrategy()
	if source.Object.Spec.RunStrategy != nil {
		r.Labeler.SetAnnotation(object, AnnRunStrategy, string(runStrategy))
	}
	r.mapNetworks(object)
	return
}

func (r *Builder) mapNetworks(target *cnv.VirtualMachine) {
	networkMap := make(map[string]api.DestinationNetwork)
	for _, network := range r.Map.Network.Spec.Map {
		networkMap[network.Source.Name] = network.Destination
	}
	for i := range target.Spec.Template.Spec.Networks {
		network := &target.Spec.Template.Spec.Networks[i]
		switch {
		case network.Multus != nil:
			destination := networkMap[network.Multus.NetworkName]
			network.Multus.NetworkName = path.Join(destination.Namespace, destination.Name)
		case network.Pod != nil:
		}
	}
}

// DataVolumes builds CRs for each of the DataVolumes specified in the source VM. The
// destination CRs should have a `blank` DataVolumeSource as the live migration mechanism
// in KubeVirt will deal with populating them.
// These need to be applied to the target cluster before the VM that refers to them.
// (in part because if the VM spec has DataVolumeTemplates and the specified DVs do not exist yet,
// then KubeVirt will try to create and populate them based on the template source which we don't want.)
func (r *Builder) DataVolumes(vm *planapi.VMStatus) (dvs []cdi.DataVolume, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	storageMap := make(map[string]api.DestinationStorage)
	for _, storage := range r.Map.Storage.Spec.Map {
		storageMap[storage.Source.Name] = storage.Destination
	}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		source := &model.DataVolume{}
		pvc := &model.PersistentVolumeClaim{}
		switch {
		case vol.DataVolume != nil:
			dvRef := ref.Ref{Name: vol.DataVolume.Name, Namespace: vm.Namespace}
			err = r.Source.Inventory.Find(&source, dvRef)
			if err != nil {
				return
			}
			pvcRef := ref.Ref{Name: source.Object.Status.ClaimName, Namespace: vm.Namespace}
			err = r.Source.Inventory.Find(pvc, pvcRef)
			if err != nil {
				return
			}
		default:
			continue
		}
		var storageClass string
		if source.Object.Spec.Storage != nil && source.Object.Spec.Storage.StorageClassName != nil {
			storageClass = *source.Object.Spec.Storage.StorageClassName
		} else if pvc.Object.Spec.StorageClassName != nil {
			storageClass = *pvc.Object.Spec.StorageClassName
		}
		storage, ok := storageMap[storageClass]
		if !ok {
			err = liberr.New(
				"Couldn't find destination storage mapping for volume.",
				"sc", storageClass,
				"volume", vol.Name,
				"kind", "dv",
			)
			return
		}
		target := r.targetDataVolume(source, pvc, storage)
		r.Labeler.SetLabels(&target, r.Labeler.VMLabels(vm.Ref))
		r.Labeler.SetAnnotations(&target, r.Labeler.VMLabels(vm.Ref))
		r.Labeler.SetAnnotation(&target, AnnDiskSource, path.Join(pvc.Namespace, pvc.Name))
		r.Labeler.SetAnnotation(&target, AnnVolumeName, vol.Name)
		dvs = append(dvs, target)
	}
	return
}

// PersistentVolumeClaims builds CRs for each of the PersistentVolumeClaims specified in the source VM.
func (r *Builder) PersistentVolumeClaims(vm *planapi.VMStatus) (pvcs []core.PersistentVolumeClaim, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	storageMap := make(map[string]api.DestinationStorage)
	for _, storage := range r.Map.Storage.Spec.Map {
		storageMap[storage.Source.Name] = storage.Destination
	}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		source := &model.PersistentVolumeClaim{}
		switch {
		case vol.PersistentVolumeClaim != nil:
			pvcRef := ref.Ref{Name: vol.PersistentVolumeClaim.ClaimName, Namespace: vm.Namespace}
			err = r.Source.Inventory.Find(source, pvcRef)
			if err != nil {
				err = liberr.Wrap(err, "vm", vm.Ref.String(), "volume", vol.Name)
				return
			}
		case vol.Ephemeral != nil && vol.Ephemeral.PersistentVolumeClaim != nil:
			pvcRef := ref.Ref{Name: vol.Ephemeral.PersistentVolumeClaim.ClaimName, Namespace: vm.Namespace}
			err = r.Source.Inventory.Find(source, pvcRef)
			if err != nil {
				err = liberr.Wrap(err, "vm", vm.Ref.String(), "volume", vol.Name)
				return
			}
		default:
			continue
		}
		var storageClass string
		if source.Object.Spec.StorageClassName != nil {
			storageClass = *source.Object.Spec.StorageClassName
		}
		storage, ok := storageMap[storageClass]
		if !ok {
			err = liberr.New(
				"Couldn't find destination storage mapping for volume.",
				"sc", storageClass,
				"volume", vol.Name,
				"kind", "pvc",
			)
			return
		}
		target := r.targetPvc(source, storage)
		r.Labeler.SetLabels(&target, r.Labeler.VMLabels(vm.Ref))
		r.Labeler.SetAnnotations(&target, r.Labeler.VMLabels(vm.Ref))
		r.Labeler.SetAnnotation(&target, AnnDiskSource, path.Join(source.Namespace, source.Name))
		r.Labeler.SetAnnotation(&target, AnnVolumeName, vol.Name)
		pvcs = append(pvcs, target)
	}
	return
}

// targetPvc creates a target CR based on the source CR.
func (r *Builder) targetPvc(source *model.PersistentVolumeClaim, storage api.DestinationStorage) (pvc core.PersistentVolumeClaim) {
	pvc = core.PersistentVolumeClaim{}
	pvc.Namespace = r.Plan.Spec.TargetNamespace
	pvc.Name = source.Name
	pvc.Labels = source.Object.Labels
	pvc.Annotations = source.Object.Annotations
	pvc.Spec = core.PersistentVolumeClaimSpec{
		Selector:         source.Object.Spec.Selector,
		Resources:        source.Object.Spec.Resources,
		StorageClassName: &storage.StorageClass,
	}
	if storage.AccessMode != "" {
		pvc.Spec.AccessModes = []core.PersistentVolumeAccessMode{storage.AccessMode}
	}
	if storage.VolumeMode != "" {
		pvc.Spec.VolumeMode = &storage.VolumeMode
	}
	return
}

// targetDataVolume creates a target CR based on the source CR.
func (r *Builder) targetDataVolume(source *model.DataVolume, pvc *model.PersistentVolumeClaim, storage api.DestinationStorage) (dv cdi.DataVolume) {
	size := pvc.Object.Spec.Resources.Requests["storage"]
	dv = cdi.DataVolume{}
	dv.Namespace = r.Plan.Spec.TargetNamespace
	dv.Name = source.Name
	dv.Labels = source.Object.Labels
	dv.Annotations = source.Object.Annotations
	dv.Annotations[AnnBindImmediate] = "true"
	dv.Annotations[AnnDeleteAfterCompletion] = "false"
	dv.Spec = cdi.DataVolumeSpec{
		Source: &cdi.DataVolumeSource{
			Blank: &cdi.DataVolumeBlankImage{},
		},
		Storage: &cdi.StorageSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: size,
				},
			},
			StorageClassName: &storage.StorageClass,
		},
	}
	if source.Object.Spec.Storage != nil {
		if len(source.Object.Spec.Storage.Resources.Limits) != 0 {
			dv.Spec.Storage.Resources.Limits = source.Object.Spec.Storage.Resources.Limits
		}
		if len(source.Object.Spec.Storage.Resources.Requests) != 0 {
			dv.Spec.Storage.Resources.Requests = source.Object.Spec.Storage.Resources.Requests
		}
	}
	if storage.AccessMode != "" {
		dv.Spec.Storage.AccessModes = []core.PersistentVolumeAccessMode{storage.AccessMode}
	}
	if storage.VolumeMode != "" {
		dv.Spec.Storage.VolumeMode = &storage.VolumeMode
	}
	return
}

// LocalInstanceType builds a copy of a namespace-local InstanceType from the source.
func (r *Builder) LocalInstanceType(vm *planapi.VMStatus) (target *instancetype.VirtualMachineInstancetype, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	if virtualMachine.Object.Spec.Instancetype == nil || virtualMachine.Object.Spec.Instancetype.Kind == kubevirtapi.ClusterSingularResourceName {
		err = liberr.New("VM does not have a reference to a local InstanceType.")
		return
	}
	source := instancetype.VirtualMachineInstancetype{}
	key := types.NamespacedName{Namespace: vm.Namespace, Name: virtualMachine.Object.Spec.Instancetype.Name}
	err = r.sourceClient.Get(context.Background(), key, &source)
	if err != nil {
		err = liberr.Wrap(err, "instancetype", virtualMachine.Object.Spec.Instancetype.Name)
	}
	target = &instancetype.VirtualMachineInstancetype{}
	target.Namespace = r.Plan.Spec.TargetNamespace
	target.Name = source.Name
	target.SetLabels(source.GetLabels())
	target.SetAnnotations(source.GetAnnotations())
	r.Labeler.SetAnnotation(target, AnnSource, key.String())
	source.Spec.DeepCopyInto(&target.Spec)
	return
}

// LocalPreference builds a copy of a namespace-local Preference from the source.
func (r *Builder) LocalPreference(vm *planapi.VMStatus) (target *instancetype.VirtualMachinePreference, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	if virtualMachine.Object.Spec.Preference == nil || virtualMachine.Object.Spec.Preference.Kind == kubevirtapi.ClusterSingularPreferenceResourceName {
		err = liberr.New("VM does not have a reference to a local Preference.")
		return
	}
	source := instancetype.VirtualMachinePreference{}
	key := types.NamespacedName{Namespace: vm.Namespace, Name: virtualMachine.Object.Spec.Preference.Name}
	err = r.sourceClient.Get(context.Background(), key, &source)
	if err != nil {
		err = liberr.Wrap(err, "preference", virtualMachine.Object.Spec.Preference.Name)
	}
	target = &instancetype.VirtualMachinePreference{}
	target.Namespace = r.Plan.Spec.TargetNamespace
	target.Name = source.Name
	target.SetLabels(source.GetLabels())
	target.SetAnnotations(source.GetAnnotations())
	r.Labeler.SetAnnotation(target, AnnSource, key.String())
	source.Spec.DeepCopyInto(&target.Spec)
	return
}

// ConfigMaps builds CRs for each of the ConfigMaps that the source VM depends upon.
// Migration labels are set to track when they were first created, but since these may be
// used by more than one VM they are not labeled with the VM id.
func (r *Builder) ConfigMaps(vm *planapi.VMStatus) (list []core.ConfigMap, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	sources := []types.NamespacedName{}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		switch {
		case vol.ConfigMap != nil:
			key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.ConfigMap.Name}
			sources = append(sources, key)
		default:
			continue
		}
	}
	for _, key := range sources {
		source := &core.ConfigMap{}
		err = r.sourceClient.Get(context.Background(), key, source)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		target := core.ConfigMap{}
		target.Name = source.Name
		target.Namespace = r.Plan.Spec.TargetNamespace
		target.Data = source.Data
		target.BinaryData = source.BinaryData
		target.Immutable = source.Immutable
		target.SetLabels(source.GetLabels())
		r.Labeler.SetLabels(&target, r.Labeler.MigrationLabels())
		target.SetAnnotations(source.GetAnnotations())
		r.Labeler.SetAnnotation(&target, AnnSource, key.String())
		list = append(list, target)
	}
	return
}

// Secrets builds CRs for each of the Secrets that the source VM depends upon.
// Migration labels are set to track when they were first created, but since these may be
// used by more than one VM they are not labeled with the VM id.
func (r *Builder) Secrets(vm *planapi.VMStatus) (list []core.Secret, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	sources := []types.NamespacedName{}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		switch {
		case vol.Secret != nil:
			key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.Secret.SecretName}
			sources = append(sources, key)
		case vol.CloudInitNoCloud != nil:
			if vol.CloudInitNoCloud.UserDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitNoCloud.UserDataSecretRef.Name}
				sources = append(sources, key)
			}
			if vol.CloudInitNoCloud.NetworkDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitNoCloud.NetworkDataSecretRef.Name}
				sources = append(sources, key)
			}
		case vol.CloudInitConfigDrive != nil:
			if vol.CloudInitConfigDrive.UserDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitConfigDrive.UserDataSecretRef.Name}
				sources = append(sources, key)
			}
			if vol.CloudInitConfigDrive.NetworkDataSecretRef != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.CloudInitConfigDrive.NetworkDataSecretRef.Name}
				sources = append(sources, key)
			}
		default:
			continue
		}
	}
	for _, key := range sources {
		source := &core.Secret{}
		err = r.sourceClient.Get(context.Background(), key, source)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		target := core.Secret{}
		target.Name = source.Name
		target.Namespace = r.Plan.Spec.TargetNamespace
		target.Data = source.Data
		target.Immutable = source.Immutable
		target.SetLabels(source.GetLabels())
		target.SetAnnotations(source.GetAnnotations())
		r.Labeler.SetAnnotation(&target, AnnSource, key.String())
		list = append(list, target)
	}
	return
}

const (
	FlagPreHook    libitr.Flag = 0x01
	FlagPostHook   libitr.Flag = 0x02
	FlagSubmariner libitr.Flag = 0x04
)

// Step predicate.
type Predicate struct {
	// VM listed on the plan.
	vm *planapi.VM
	// Plan context
	context *plancontext.Context
}

// Evaluate predicate flags.
func (r *Predicate) Evaluate(flag libitr.Flag) (allowed bool, err error) {
	switch flag {
	case FlagPreHook:
		_, allowed = r.vm.FindHook(PreHook)
	case FlagPostHook:
		_, allowed = r.vm.FindHook(PostHook)
	case FlagSubmariner:
		allowed = Submariner(r.context)
	}
	return
}

func (r *Predicate) Count() int {
	return 0x04
}

func Submariner(context *plancontext.Context) (submariner bool) {
	submariner, _ = strconv.ParseBool(context.Source.Provider.Spec.Settings["submariner"])
	return
}
