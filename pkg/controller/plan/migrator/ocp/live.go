package ocp

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/plan/ensurer"
	"github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	batch "k8s.io/api/batch/v1"
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

// Annotations
const (
	AnnDiskSource            = "forklift.konveyor.io/disk-source"
	AnnVolumeName            = "forklift.konveyor.io/volume"
	AnnSource                = "forklift.konveyor.io/source"
	AnnRestoreRunStrategy    = "kubevirt.io/restore-run-strategy"
	AnnBindImmediate         = "cdi.kubevirt.io/storage.bind.immediate.requested"
	AnnDeleteAfterCompletion = "cdi.kubevirt.io/storage.deleteAfterCompletion"
)

// Phases
const (
	Started                                = api.PhaseStarted
	PreHook                                = api.PhasePreHook
	CreateServiceExports                   = "CreateServiceExports"
	CreateSecrets                          = "CreateSecrets"
	CreateConfigMaps                       = "CreateConfigMaps"
	EnsurePreference                       = "EnsurePreference"
	EnsureInstanceType                     = "EnsureInstanceType"
	EnsureDataVolumes                      = "EnsureDataVolumes"
	EnsurePersistentVolumeClaims           = "EnsurePersistentVolumeClaims"
	CreateTarget                           = "CreateTarget"
	SetOwnerReferences                     = "SetOwnerReferences"
	WaitForTargetVMI                       = "WaitForTargetVMI"
	CreateVirtualMachineInstanceMigrations = "CreateVirtualMachineInstanceMigrations"
	WaitForStateTransfer                   = "WaitForStateTransfer"
	PostHook                               = api.PhasePostHook
	Completed                              = api.PhaseCompleted
)

// Pipeline
const (
	PrepareTarget   = "PrepareTarget"
	Synchronization = "Synchronization"
)

// KubeVirt CA configmaps
const (
	KubeVirtCA         = "kubevirt-ca"
	ExternalKubeVirtCA = "kubevirt-external-ca"
	KeyCABundle        = "ca-bundle"
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
	r.ensurer = Ensurer{
		Ensurer:      &ensurer.Ensurer{Context: r.Context},
		SourceClient: r.sourceClient,
	}
	return
}

func (r *LiveMigrator) Logger() (logger logging.LevelLogger) {
	return r.Log
}

// Begin the migration process. This is called once at the beginning
// of a migration plan, before any of the VMs begin to migrate.
func (r *LiveMigrator) Begin() (err error) {
	if r.Source.Provider.UID != r.Destination.Provider.UID {
		err = r.SynchronizeCertificateBundles()
		return
	}
	return
}

func (r *LiveMigrator) Complete(vm *planapi.VMStatus) {
	err := r.DeleteTargetVMIM(vm)
	if err != nil {
		r.Log.Error(err, "Unable to clean up target VMIM.", "vm", vm.String())
	}
	err = r.DeleteSourceVMIM(vm)
	if err != nil {
		r.Log.Error(err, "Unable to clean up source VMIM.", "vm", vm.String())
	}
	err = r.DeleteServiceExports(vm)
	if err != nil {
		r.Log.Error(err, "Unable to clean up service exports.", "vm", vm.String())
	}
	err = r.DeleteJobs(vm)
	if err != nil {
		r.Log.Error(err, "Unable to clean up jobs.", "vm", vm.String())
	}
	if !vm.HasCondition(api.ConditionSucceeded) {
		err = r.DeleteDataVolumes(vm)
		if err != nil {
			r.Log.Error(err, "Unable to clean up datavolumes.", "vm", vm.String())
		}
		err = r.DeleteVirtualMachine(vm)
		if err != nil {
			r.Log.Error(err, "Unable to clean up target VM.", "vm", vm.String())
		}
	}
}

func (r *LiveMigrator) Status(vm planapi.VM) (status *planapi.VMStatus) {
	if current, found := r.Context.Plan.Status.Migration.FindVM(vm.Ref); !found {
		status = &planapi.VMStatus{VM: vm}
	} else {
		status = current
	}
	return
}

func (r *LiveMigrator) Reset(vm *planapi.VMStatus, pipeline []*planapi.Step) {
	vm.DeleteCondition(api.ConditionCanceled, api.ConditionFailed)
	vm.MarkReset()
	itr := r.Itinerary(vm.VM)
	step, _ := itr.First()
	vm.Phase = step.Name
	vm.Pipeline = pipeline
	vm.Error = nil
	vm.Warm = nil
}

func (r *LiveMigrator) Itinerary(vm planapi.VM) (itinerary *libitr.Itinerary) {
	itinerary = &libitr.Itinerary{
		Name: "ocp-live",
		Pipeline: libitr.Pipeline{
			{Name: Started},
			{Name: PreHook, All: FlagPreHook},
			{Name: CreateSecrets},
			{Name: CreateConfigMaps},
			{Name: EnsurePreference},
			{Name: EnsureInstanceType},
			{Name: EnsureDataVolumes},
			{Name: EnsurePersistentVolumeClaims},
			{Name: CreateTarget},
			{Name: SetOwnerReferences},
			{Name: CreateServiceExports, All: FlagSubmariner | FlagIntercluster},
			{Name: WaitForTargetVMI},
			{Name: CreateVirtualMachineInstanceMigrations},
			{Name: WaitForStateTransfer},
			{Name: PostHook, All: FlagPostHook},
			{Name: Completed},
		},
		Predicate: &Predicate{vm: &vm, context: r.Context},
	}
	return
}

func (r *LiveMigrator) Next(vm *planapi.VMStatus) (next string) {
	itinerary := r.Itinerary(vm.VM)
	step, done, err := itinerary.Next(vm.Phase)
	if done || err != nil {
		next = Completed
		if err != nil {
			r.Log.Error(err, "Next phase failed.")
		}
	} else {
		next = step.Name
	}
	r.Log.Info("Itinerary transition", "current phase", vm.Phase, "next phase", next)
	return
}

func (r *LiveMigrator) Step(vm *planapi.VMStatus) (step string) {
	switch vm.Phase {
	case Started:
		step = base.Initialize
	case PreHook, PostHook:
		step = vm.Phase
	case CreateSecrets, CreateConfigMaps, EnsurePreference, EnsureInstanceType, EnsureDataVolumes, EnsurePersistentVolumeClaims, CreateTarget, SetOwnerReferences, CreateServiceExports:
		step = PrepareTarget
	case WaitForTargetVMI, CreateVirtualMachineInstanceMigrations, WaitForStateTransfer:
		step = Synchronization
	default:
		step = base.Unknown
	}
	return
}

func (r *LiveMigrator) Pipeline(vm planapi.VM) (pipeline []*planapi.Step, err error) {
	itinerary := r.Itinerary(vm)
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
						Phase:       api.StepPending,
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
						Phase:       api.StepPending,
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
						Phase:       api.StepPending,
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
						Phase:       api.StepPending,
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
						Phase:       api.StepPending,
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

// NextPhase transitions the VM to the next migration phase.
func (r *LiveMigrator) NextPhase(vm *planapi.VMStatus) {
	base.NextPhase(r, vm)
}

func (r *LiveMigrator) StepError(vm *planapi.VMStatus, err error) {
	step, found := vm.FindStep(r.Step(vm))
	if !found {
		vm.AddError(fmt.Sprintf("Step '%s' not found", r.Step(vm)))
		return
	}
	step.AddError(err.Error())
}

// ExecutePhase provides implementations of VM migration phases.
func (r *LiveMigrator) ExecutePhase(vm *planapi.VMStatus) (ok bool, err error) {
	ok = true
	switch vm.Phase {
	case Started:
		vm.MarkedStarted()
		r.NextPhase(vm)
	case PreHook, PostHook:
		// delegate to the common pipeline
		ok = false
		return
	case CreateSecrets:
		var secrets []core.Secret
		secrets, err = r.builder.Secrets(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building secrets", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		err = r.ensurer.SharedSecrets(vm, secrets)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error ensuring secrets", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case CreateConfigMaps:
		var configmaps []core.ConfigMap
		configmaps, err = r.builder.ConfigMaps(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building configmaps", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		err = r.ensurer.SharedConfigMaps(vm, configmaps)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error ensuring configmaps", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case EnsurePreference:
		var required bool
		required, err = r.RequiresLocalPreference(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error checking for Preference", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		if required {
			var preference *instancetype.VirtualMachinePreference
			preference, err = r.builder.LocalPreference(vm)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error building Preference", "vm", vm.Name)
					r.StepError(vm, err)
					err = nil
				}
				break
			}
			err = r.ensurer.EnsureLocalPreference(vm, preference)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error ensuring Preference", "vm", vm.Name)
					r.StepError(vm, err)
					err = nil
				}
				break
			}
		}
		r.NextPhase(vm)
	case EnsureInstanceType:
		var required bool
		required, err = r.RequiresLocalInstanceType(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error checking for InstanceType", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		if required {
			var instancetype *instancetype.VirtualMachineInstancetype
			instancetype, err = r.builder.LocalInstanceType(vm)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error building InstanceType", "vm", vm.Name)
					r.StepError(vm, err)
					err = nil
				}
				break
			}
			err = r.ensurer.EnsureLocalInstanceType(vm, instancetype)
			if err != nil {
				if !errors.As(err, &web.ProviderNotReadyError{}) {
					r.Log.Error(err, "error ensuring InstanceType", "vm", vm.Name)
					r.StepError(vm, err)
					err = nil
				}
				break
			}
		}
		r.NextPhase(vm)
	case EnsureDataVolumes:
		var dataVolumes []cdi.DataVolume
		dataVolumes, err = r.builder.DataVolumes(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building volumes", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		err = r.ensurer.DataVolumes(vm, dataVolumes)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case EnsurePersistentVolumeClaims:
		var pvcs []core.PersistentVolumeClaim
		pvcs, err = r.builder.PersistentVolumeClaims(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.Log.Error(err, "error building volumes", "vm", vm.Name)
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		err = r.ensurer.PersistentVolumeClaims(vm, pvcs)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case CreateTarget:
		var target *cnv.VirtualMachine
		target, err = r.builder.VirtualMachine(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		err = r.ensurer.VirtualMachine(vm, target)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case SetOwnerReferences:
		err = r.ensurer.EnsureOwnerReferences(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case CreateServiceExports:
		var kv *cnv.KubeVirt
		kv, err = r.GetTargetKubeVirt()
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		sync := r.builder.SyncServiceExport(vm, kv.Namespace)
		err = r.ensurer.EnsureServiceExport(vm, sync)
		if err != nil {
			r.StepError(vm, err)
			err = nil
			break
		}
		migration := r.builder.MigrationServiceExport(vm, kv.Namespace)
		err = r.ensurer.EnsureServiceExport(vm, migration)
		if err != nil {
			r.StepError(vm, err)
			err = nil
			break
		}
		r.NextPhase(vm)
	case WaitForTargetVMI:
		var ready bool
		ready, err = r.WaitForTargetVMI(vm)
		if err != nil {
			r.StepError(vm, err)
			err = nil
			break
		}
		if !ready {
			return
		}
		r.NextPhase(vm)
	case CreateVirtualMachineInstanceMigrations:
		var target *cnv.VirtualMachineInstanceMigration
		target, err = r.builder.TargetVMIM(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		target, err = r.ensurer.EnsureTargetVMIM(vm, target)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		address, ready := r.SynchronizationAddressReady(target)
		if !ready {
			return
		}
		var source *cnv.VirtualMachineInstanceMigration
		source, err = r.builder.SourceVMIM(vm, address)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		err = r.ensurer.EnsureSourceVMIM(vm, source)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		r.NextPhase(vm)
	case WaitForStateTransfer:
		var done bool
		done, err = r.WaitForStateTransfer(vm)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				r.StepError(vm, err)
				err = nil
			}
			break
		}
		if !done {
			return
		}
		r.NextPhase(vm)
	case Completed:
		r.Complete(vm)
		vm.MarkCompleted()
		r.Log.Info(
			"Migration [COMPLETED]",
			"vm",
			vm.String())
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

// SynchronizeCertificateBundles between the source and destination clusters.
// Copies contents of kubevirt-ca configmap on each cluster into the external-kubevirt-ca
// configmap on the opposite cluster.
func (r *LiveMigrator) SynchronizeCertificateBundles() (err error) {
	sourceKV, err := r.GetSourceKubeVirt()
	if err != nil {
		return
	}
	destKV, err := r.GetTargetKubeVirt()
	if err != nil {
		return
	}
	sourceCA, err := r.GetConfigMap(context.TODO(), r.sourceClient, sourceKV.Namespace, KubeVirtCA)
	if err != nil {
		err = liberr.New("Unable to get KubeVirt CA bundle from source cluster.", "err", err)
		return
	}
	destCA, err := r.GetConfigMap(context.TODO(), r.Destination.Client, destKV.Namespace, KubeVirtCA)
	if err != nil {
		err = liberr.New("Unable to get KubeVirt CA bundle from destination cluster.", "reason", err)
		return
	}
	sourceExternalCA, err := r.GetConfigMap(context.TODO(), r.sourceClient, sourceKV.Namespace, ExternalKubeVirtCA)
	if err != nil {
		err = liberr.New("Unable to get external KubeVirt CA bundle from source cluster.", "reason", err)
		return
	}
	destExternalCA, err := r.GetConfigMap(context.TODO(), r.Destination.Client, destKV.Namespace, ExternalKubeVirtCA)
	if err != nil {
		err = liberr.New("Unable to get external KubeVirt CA bundle from destination cluster.", "reason", err)
		return
	}
	sourceData := sourceCA.Data[KeyCABundle]
	sourceExternalData := sourceExternalCA.Data[KeyCABundle]
	destData := destCA.Data[KeyCABundle]
	destExternalData := destExternalCA.Data[KeyCABundle]
	if !strings.Contains(destExternalData, sourceData) {
		destExternalCA.Data[KeyCABundle] = destExternalData + fmt.Sprintf("\n%s", sourceData)
		err = r.Destination.Client.Update(context.TODO(), destExternalCA)
		if err != nil {
			err = liberr.New("Unable to update external KubeVirt CA bundle on destination cluster.", "reason", err) // TODO
			return
		}
	}
	if !strings.Contains(sourceExternalData, destData) {
		sourceExternalCA.Data[KeyCABundle] = sourceExternalData + fmt.Sprintf("\n%s", destData)
		err = r.sourceClient.Update(context.TODO(), sourceExternalCA)
		if err != nil {
			err = liberr.New("Unable to update external KubeVirt CA bundle on source cluster.", "reason", err) // TODO
			return
		}
	}
	return
}

// WaitForStateTransfer checks the status of the VMIM resources to determine if the
// migration has succeeded. Success is defined as both VMIMs reporting MigrationSucceeded,
// and a failure is reported if either VMIM reports MigrationFailed.
func (r *LiveMigrator) WaitForStateTransfer(vm *planapi.VMStatus) (done bool, err error) {
	target, err := r.GetTargetVMIM(vm)
	if err != nil {
		return
	}
	if target.Status.Phase == cnv.MigrationFailed {
		err = liberr.New("Migration failed, check VMIM status for details.", "vm", vm.String())
		return
	}
	if target.Status.Phase == cnv.MigrationSucceeded {
		done = true
	}
	return
}

// RequiresLocalPreference returns true if the source VM object has a relationship to a
// local (namespaced) VirtualMachinePreference.
func (r *LiveMigrator) RequiresLocalPreference(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Preference != nil &&
		!strings.EqualFold(virtualMachine.Object.Spec.Preference.Kind, kubevirtapi.ClusterSingularPreferenceResourceName)
	return
}

// RequiresClusterPreference returns true if the source VM object has a relationship to a
// VirtualMachineClusterPreference.
func (r *LiveMigrator) RequiresClusterPreference(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Preference != nil &&
		strings.EqualFold(virtualMachine.Object.Spec.Preference.Kind, kubevirtapi.ClusterSingularPreferenceResourceName)
	return
}

// RequiresLocalInstanceType returns true if the source VM object has a relationship to a local
// (namespaced) VirtualMachineInstancetype.
func (r *LiveMigrator) RequiresLocalInstanceType(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Instancetype != nil &&
		!strings.EqualFold(virtualMachine.Object.Spec.Instancetype.Kind, kubevirtapi.ClusterSingularResourceName)
	return
}

// RequiresClusterInstanceType returns true if the source VM object has a relationship to a
// VirtualMachineClusterInstancetype.
func (r *LiveMigrator) RequiresClusterInstanceType(vm *planapi.VMStatus) (required bool, err error) {
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	required = virtualMachine.Object.Spec.Instancetype != nil &&
		strings.EqualFold(virtualMachine.Object.Spec.Instancetype.Kind, kubevirtapi.ClusterSingularResourceName)
	return
}

// SynchronizationAddressReady reports when the synchronization address is available in the target VMIM status.
func (r *LiveMigrator) SynchronizationAddressReady(vmim *cnv.VirtualMachineInstanceMigration) (address string, ready bool) {
	if len(vmim.Status.SynchronizationAddresses) > 0 {
		ready = true
		address = vmim.Status.SynchronizationAddresses[0]
	}
	return
}

// GetTargetVM from the destination cluster or return an error if it is not found. If
// more than one VM matches the selection criteria, the first is returned.
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

// GetConfigMap by namespace and name with specified client.
func (r *LiveMigrator) GetConfigMap(ctx context.Context, c client.Client, namespace string, name string) (cm *core.ConfigMap, err error) {
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	cm = &core.ConfigMap{}
	err = c.Get(ctx, key, cm)
	if err != nil {
		err = liberr.Wrap(err, "configmap", key.String())
		return
	}
	return
}

// GetTargetVMIM object from the destination cluster.
func (r *LiveMigrator) GetTargetVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
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
		err = liberr.New("Target VMIM not found.", "vm", vm.String())
		return
	} else {
		vmim = &vmims.Items[0]
	}
	return
}

// GetSourceVMIM object from the source cluster.
func (r *LiveMigrator) GetSourceVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
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
		err = liberr.New("Source VMIM not found.", "vm", vm.String())
		return
	} else {
		vmim = &vmims.Items[0]
	}
	return
}

// WaitForTargetVMI waits for the target to indicate that it is ready to receive
// the source state. The target VMIM cannot be created until the VMI exists and
// is waiting for sync.
func (r *LiveMigrator) WaitForTargetVMI(vm *planapi.VMStatus) (ready bool, err error) {
	key := types.NamespacedName{Namespace: r.Plan.Spec.TargetNamespace, Name: vm.Name}
	vmi := &cnv.VirtualMachineInstance{}
	err = r.Destination.Client.Get(context.TODO(), key, vmi)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
			return
		}
		err = liberr.Wrap(err)
		return
	}
	ready = vmi.Status.Phase == cnv.WaitingForSync
	return
}

func (r *LiveMigrator) GetSourceKubeVirt() (*cnv.KubeVirt, error) {
	return r.findKubeVirt(r.Source.Inventory)
}

func (r *LiveMigrator) GetTargetKubeVirt() (*cnv.KubeVirt, error) {
	return r.findKubeVirt(r.Destination.Inventory)
}

func (r *LiveMigrator) findKubeVirt(inventory web.Client) (kv *cnv.KubeVirt, err error) {
	list := []model.KubeVirt{}
	err = inventory.List(&list, web.Param{
		Key:   web.DetailParam,
		Value: "all",
	})
	if err != nil {
		return
	}
	if len(list) == 0 {
		err = liberr.New("KubeVirt CR not found in any namespace.")
		return
	} else {
		kv = &list[0].Object
		return
	}
}

// DeleteServiceExports deletes the ServiceExports that were created to expose the sync endpooints on
// the destination cluster.
func (r *LiveMigrator) DeleteServiceExports(vm *planapi.VMStatus) (err error) {
	kv, err := r.GetTargetKubeVirt()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.Destination.Client.DeleteAllOf(
		context.Background(),
		&multicluster.ServiceExport{},
		&client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
				Namespace:     kv.Namespace,
			},
		})
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

// DeleteDataVolumes deletes the DataVolumes that were created for this VM.
func (r *LiveMigrator) DeleteDataVolumes(vm *planapi.VMStatus) (err error) {
	err = r.Destination.Client.DeleteAllOf(
		context.Background(),
		&cdi.DataVolume{},
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

// DeleteVirtualMachine deletes the target VirtualMachine.
func (r *LiveMigrator) DeleteVirtualMachine(vm *planapi.VMStatus) (err error) {
	err = r.Destination.Client.DeleteAllOf(
		context.Background(),
		&cnv.VirtualMachine{},
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

// DeleteJobs deletes any hook jobs created for the VM on the target cluster.
func (r *LiveMigrator) DeleteJobs(vm *planapi.VMStatus) (err error) {
	err = r.Destination.Client.DeleteAllOf(
		context.Background(),
		&batch.Job{},
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
	*ensurer.Ensurer
	SourceClient client.Client
}

// EnsureOwnerReferences are set on the target VM's DataVolumes.
func (r *Ensurer) EnsureOwnerReferences(vm *planapi.VMStatus) (err error) {
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
		err = liberr.New("unable to locate target VM for setting owner references")
		return
	}
	target := &vms.Items[0]
	dvs := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.Background(),
		dvs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range dvs.Items {
		dv := &dvs.Items[i]
		r.Labeler.SetOwnerReferences(target, dv)
		err = r.Destination.Client.Update(context.Background(), dv)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	pvcs := &core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.Background(),
		pvcs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		r.Labeler.SetOwnerReferences(target, pvc)
		err = r.Destination.Client.Update(context.Background(), pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	return
}

// EnsureTargetVMIM
func (r *Ensurer) EnsureTargetVMIM(vm *planapi.VMStatus, target *cnv.VirtualMachineInstanceMigration) (out *cnv.VirtualMachineInstanceMigration, err error) {
	list := &cnv.VirtualMachineInstanceMigrationList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(list.Items) == 0 {
		err = r.Destination.Client.Create(context.TODO(), target)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created target VirtualMachineInstanceMigration.",
			"vmim",
			path.Join(
				target.Namespace,
				target.Name),
			"vm",
			vm.String())
		out = target
	} else {
		out = &list.Items[0]
	}
	return
}

// EnsureSourceVMIM
func (r *Ensurer) EnsureSourceVMIM(vm *planapi.VMStatus, source *cnv.VirtualMachineInstanceMigration) (err error) {
	list := &cnv.VirtualMachineInstanceMigrationList{}
	err = r.SourceClient.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(list.Items) == 0 {
		err = r.SourceClient.Create(context.TODO(), source)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created source VirtualMachineInstanceMigration.",
			"vmim",
			path.Join(
				source.Namespace,
				source.Name),
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

// EnsureServiceExport exists on destination cluster.
func (r *Ensurer) EnsureServiceExport(vm *planapi.VMStatus, export *multicluster.ServiceExport) (err error) {
	err = r.Destination.Client.Create(context.TODO(), export)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			err = nil
			return
		}
		err = liberr.Wrap(err, "vm", vm.String())
		return
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
func (r *Builder) VirtualMachine(vm *planapi.VMStatus) (object *cnv.VirtualMachine, err error) {
	source := &model.VM{}
	err = r.Source.Inventory.Find(source, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	waitAsReceiver := cnv.RunStrategyWaitAsReceiver
	object = &cnv.VirtualMachine{
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
			RunStrategy:  &waitAsReceiver,
		},
	}
	key := types.NamespacedName{Namespace: vm.Namespace, Name: source.Name}
	object.Name = source.Name
	object.Namespace = r.Plan.Spec.TargetNamespace
	r.Labeler.SetLabels(object, source.Object.Labels)
	r.Labeler.SetLabels(object, r.Labeler.VMLabels(vm.Ref))
	r.Labeler.SetAnnotations(object, source.Object.Annotations)
	r.Labeler.SetAnnotations(object, r.Labeler.VMLabels(vm.Ref))
	r.Labeler.SetAnnotation(object, AnnSource, key.String())

	// preserve the original runstrategy so that it can be applied
	// once the migration is complete.
	runStrategy, _ := source.Object.RunStrategy()
	if source.Object.Spec.RunStrategy != nil {
		r.Labeler.SetAnnotation(object, AnnRestoreRunStrategy, string(runStrategy))
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
			err = r.Source.Inventory.Find(source, dvRef)
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
	for _, cred := range virtualMachine.Object.Spec.Template.Spec.AccessCredentials {
		switch {
		case cred.SSHPublicKey != nil:
			if cred.SSHPublicKey.Source.Secret != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: cred.SSHPublicKey.Source.Secret.SecretName}
				sources = append(sources, key)
			}
		case cred.UserPassword != nil:
			if cred.UserPassword.Source.Secret != nil {
				key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: cred.UserPassword.Source.Secret.SecretName}
				sources = append(sources, key)
			}
		}
	}
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

func (r *Builder) TargetVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
	inventoryVm := &model.VM{}
	err = r.Source.Inventory.Find(inventoryVm, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	vmim = &cnv.VirtualMachineInstanceMigration{}
	vmim.GenerateName = "forklift-"
	vmim.Namespace = r.Context.Plan.Spec.TargetNamespace
	vmim.Labels = r.Labeler.VMLabels(vm.Ref)
	vmim.Spec.VMIName = inventoryVm.Name
	vmim.Spec.Receive = &cnv.VirtualMachineInstanceMigrationTarget{
		MigrationID: r.kubevirtMigrationID(vm),
	}
	return
}

func (r *Builder) SourceVMIM(vm *planapi.VMStatus, syncAddress string) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
	inventoryVm := &model.VM{}
	err = r.Context.Source.Inventory.Find(inventoryVm, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	vmim = &cnv.VirtualMachineInstanceMigration{}
	vmim.GenerateName = "forklift-"
	vmim.Namespace = inventoryVm.Namespace
	vmim.Labels = r.Labeler.VMLabels(vm.Ref)
	vmim.Spec.VMIName = inventoryVm.Name
	vmim.Spec.SendTo = &cnv.VirtualMachineInstanceMigrationSource{
		MigrationID: r.kubevirtMigrationID(vm),
		ConnectURL:  syncAddress,
	}
	return
}

func (r *Builder) SyncServiceExport(vm *planapi.VMStatus, kvnamespace string) (export *multicluster.ServiceExport) {
	export = &multicluster.ServiceExport{}
	export.Namespace = kvnamespace
	export.Name = fmt.Sprintf("sync-%s-%s", vm.Name, r.Plan.Spec.TargetNamespace)
	return
}

func (r *Builder) MigrationServiceExport(vm *planapi.VMStatus, kvnamespace string) (export *multicluster.ServiceExport) {
	export = &multicluster.ServiceExport{}
	export.Namespace = kvnamespace
	export.Name = fmt.Sprintf("migration-%s-%s", vm.Name, r.Plan.Spec.TargetNamespace)
	return
}

func (r *Builder) kubevirtMigrationID(vm *planapi.VMStatus) string {
	return fmt.Sprintf("%s-%s", vm.ID, r.Migration.UID)
}

const (
	FlagPreHook      libitr.Flag = 0x01
	FlagPostHook     libitr.Flag = 0x02
	FlagSubmariner   libitr.Flag = 0x04
	FlagIntercluster libitr.Flag = 0x08
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
		allowed = r.Submariner()
	case FlagIntercluster:
		allowed = r.context.Source.Provider.UID != r.context.Destination.Provider.UID
	}
	return
}

func (r *Predicate) Count() int {
	return 0x04
}

func (r *Predicate) Submariner() (submariner bool) {
	submariner, _ = strconv.ParseBool(r.context.Source.Provider.Spec.Settings["submariner"])
	return
}
