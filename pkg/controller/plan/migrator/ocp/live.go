package ocp

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/migrator/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	multicluster "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

const (
	Live = "live"
)

const (
	KubeVirtNamespace        = "kubevirtNamespace"
	AnnDiskSource            = "forklift.konveyor.io/disk-source"
	AnnVolumeName            = "forklift.konveyor.io/volume"
	AnnSource                = "forklift.konveyor.io/source"
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
	CreateTarget                           = "CreateTarget"
	CreateVirtualMachineInstanceMigrations = "CreateVirtualMachineInstanceMigrations"
	WaitForStateTransfer                   = "WaitForStateTransfer"
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

// Package logger.
var log = logging.WithName("migrator|ocp")

func New(context *plancontext.Context) (migrator base.Migrator, err error) {
	switch context.Plan.Spec.Type {
	case Live:
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
	log.Info("Built OCP migrator.", "plan", path.Join(context.Plan.Namespace, context.Plan.Name), "type", context.Plan.Spec.Type)
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
			{Name: PreHook},
			{Name: SynchronizeCertificates},
			{Name: CreateSecrets},
			{Name: CreateConfigMaps},
			{Name: CreateTarget},
			{Name: CreateServiceExports},
			{Name: CreateVirtualMachineInstanceMigrations},
			{Name: WaitForStateTransfer},
			{Name: PostHook},
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
			log.Error(err, "Next phase failed.")
		}
	} else {
		next = step.Name
	}
	log.Info("Itinerary transition", "current phase", status.Phase, "next phase", next)
	return
}

func (r *LiveMigrator) Step(status *planapi.VMStatus) (step string) {
	switch status.Phase {
	case Started:
		step = base.Initialize
	case PreHook, PostHook:
		step = status.Phase
	case CreateSecrets, CreateConfigMaps, CreateTarget, SynchronizeCertificates, CreateServiceExports:
		step = PrepareTarget
	case CreateVirtualMachineInstanceMigrations, WaitForStateTransfer:
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

	log.V(2).Info(
		"Pipeline built.",
		"vm",
		vm.String())
	return
}

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
				log.Error(err, "error building secrets", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		err = r.ensurer.EnsureSecrets(vm, secrets)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				log.Error(err, "error ensuring secrets", "vm", vm.Name)
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
				log.Error(err, "error building configmaps", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
		}
		err = r.ensurer.EnsureConfigMaps(vm, configmaps)
		if err != nil {
			if !errors.As(err, &web.ProviderNotReadyError{}) {
				log.Error(err, "error ensuring configmaps", "vm", vm.Name)
				step.AddError(err.Error())
				err = nil
			}
			break
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
				log.Error(err, "error building volumes", "vm", vm.Name)
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
		time.Sleep(2 * time.Second)
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
		step.MarkCompleted()
		step.Phase = Completed
		vm.Phase = r.Next(vm)
	default:
		ok = false
		log.Info(
			"Phase unknown, defer to base migrator.",
			"vm",
			vm,
			"phase",
			vm.Phase)
	}
	return
}

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
	inventoryVm := &model.VM{}
	err = r.Context.Source.Inventory.Find(inventoryVm, vm.Ref)
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
			Namespace:     inventoryVm.Namespace,
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
		log.Info(
			"Created target VirtualMachineInstanceMigration.",
			"vmim",
			path.Join(vmim.Namespace, vmim.Name),
			"vm",
			vm.String())
	}

	return
}

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
		log.Info(
			"Created source VirtualMachineInstanceMigration.",
			"vmim",
			path.Join(vmim.Namespace, vmim.Name),
			"vm",
			vm.String())
	}

	return
}

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
	ready = vmim.Status.SyncEndpoint != nil && *vmim.Status.SyncEndpoint != ""
	return
}

func (r *LiveMigrator) targetVMIM(vm *planapi.VMStatus) (vmim *cnv.VirtualMachineInstanceMigration, err error) {
	target, err := r.GetTargetVM(vm)
	if err != nil {
		return
	}

	vmim = &cnv.VirtualMachineInstanceMigration{}
	vmim.GenerateName = fmt.Sprintf("forklift-")
	vmim.Namespace = r.Context.Plan.Spec.TargetNamespace
	vmim.Labels = r.Labeler.VMLabels(vm.Ref)
	operation := cnv.MigrationTarget
	vmim.Spec.Operation = &operation
	vmim.Spec.VMIName = target.Name
	return
}

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
	operation := cnv.MigrationSource
	vmim.Spec.Operation = &operation
	vmim.Spec.VMIName = inventoryVm.Name
	if target.Status.SyncEndpoint != nil {
		connectUrl := strings.ReplaceAll(*target.Status.SyncEndpoint, ".svc:", ".svc.clusterset.local:")
		vmim.Spec.ConnectURL = &connectUrl
	}
	return
}

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

type Ensurer struct {
	*plancontext.Context
}

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
// TODO: raise a VM concern if a configmap with the desired name already exists but does not
// have the annotation indicating that Forklift created it.
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
// TODO: raise a VM concern if a secret with the desired name already exists but does not
// have the annotation indicating that Forklift created it.
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
			"Created Kubevirt VM.",
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

func (r *Ensurer) EnsureDataVolumeOwnership(vm *planapi.VMStatus) (err error) {
	dvs := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		return liberr.Wrap(err)
	}
	return
}

type Builder struct {
	*plancontext.Context
	sourceClient client.Client
}

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
			Template:    source.Object.Spec.Template.DeepCopy(),
			Running:     nil,
			RunStrategy: &halted,
		},
	}
	r.mapVolumes(object, dvs)
	r.mapNetworks(object)
	return
}

func (r *Builder) mapVolumes(target *cnv.VirtualMachine, dvs []cdi.DataVolume) {
	volMap := make(map[string]*cdi.DataVolume)
	for i := range dvs {
		dv := &dvs[i]
		volMap[dv.Annotations[AnnVolumeName]] = dv
	}
	for i := range target.Spec.Template.Spec.Volumes {
		vol := &target.Spec.Template.Spec.Volumes[i]
		switch {
		case vol.DataVolume != nil:
			vol.DataVolume.Name = volMap[vol.Name].Name
		case vol.PersistentVolumeClaim != nil:
			vol.DataVolume = &cnv.DataVolumeSource{
				Name: volMap[vol.Name].Name,
			}
			vol.PersistentVolumeClaim = nil
		}
	}
	return
}

func (r *Builder) mapNetworks(target *cnv.VirtualMachine) {
	networkMap := make(map[string]v1beta1.DestinationNetwork)
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

func (r *Builder) DataVolumes(vm *planapi.VMStatus) (dvs []cdi.DataVolume, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	storageMap := make(map[string]v1beta1.DestinationStorage)
	for _, storage := range r.Map.Storage.Spec.Map {
		storageMap[storage.Source.Name] = storage.Destination
	}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		pvc := &model.PersistentVolumeClaim{}
		switch {
		case vol.DataVolume != nil:
			pvc, err = r.pvc(vm, vol)
			if err != nil {
				err = liberr.Wrap(err, "vm", vm.Ref.String(), "volume", vol.Name)
				return
			}
		case vol.PersistentVolumeClaim != nil:
			pvcRef := ref.Ref{Name: vol.PersistentVolumeClaim.ClaimName, Namespace: vm.Namespace}
			err = r.Source.Inventory.Find(pvc, pvcRef)
			if err != nil {
				err = liberr.Wrap(err, "vm", vm.Ref.String(), "volume", vol.Name)
				return
			}
		default:
			continue
		}
		if pvc.Object.Spec.StorageClassName == nil {
			err = liberr.New("Couldn't find destination storage class for volume.", "")
			return
		}
		storage := storageMap[*pvc.Object.Spec.StorageClassName]
		dv := r.dataVolume(vm, pvc, storage)
		r.Labeler.SetAnnotation(&dv, AnnDiskSource, path.Join(pvc.Namespace, pvc.Name))
		r.Labeler.SetAnnotation(&dv, AnnVolumeName, vol.Name)
		dvs = append(dvs, dv)
	}
	return
}

func (r *Builder) dataVolume(vm *planapi.VMStatus, pvc *model.PersistentVolumeClaim, storage v1beta1.DestinationStorage) (dv cdi.DataVolume) {
	size := pvc.Object.Spec.Resources.Requests["storage"]
	dv = cdi.DataVolume{}
	dv.Namespace = r.Plan.Spec.TargetNamespace
	dv.GenerateName = strings.Join(
		[]string{
			r.Plan.Name,
			vm.ID},
		"-") + "-"
	dv.Annotations = r.Labeler.VMLabels(vm.Ref)
	dv.Annotations[AnnBindImmediate] = "true"
	dv.Annotations[AnnDeleteAfterCompletion] = "false"
	dv.Labels = r.Labeler.VMLabels(vm.Ref)
	dv.Spec = cdi.DataVolumeSpec{
		Source: &cdi.DataVolumeSource{
			Blank: &cdi.DataVolumeBlankImage{},
		},
		Storage: &cdi.StorageSpec{
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: size,
				},
			},
			StorageClassName: &storage.StorageClass,
		},
	}
	if storage.AccessMode != "" {
		dv.Spec.Storage.AccessModes = []core.PersistentVolumeAccessMode{storage.AccessMode}
	}
	if storage.VolumeMode != "" {
		dv.Spec.Storage.VolumeMode = &storage.VolumeMode
	}
	return
}

func (r *Builder) pvc(vm *planapi.VMStatus, vol cnv.Volume) (pvc *model.PersistentVolumeClaim, err error) {
	source := model.DataVolume{}
	dvRef := ref.Ref{Name: vol.DataVolume.Name, Namespace: vm.Namespace}
	err = r.Source.Inventory.Find(&source, dvRef)
	if err != nil {
		return
	}
	pvc = &model.PersistentVolumeClaim{}
	pvcRef := ref.Ref{Name: source.Object.Status.ClaimName, Namespace: vm.Namespace}
	err = r.Source.Inventory.Find(pvc, pvcRef)
	if err != nil {
		return
	}
	return
}

func (r *Builder) ConfigMaps(vm *planapi.VMStatus) (list []core.ConfigMap, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		switch {
		case vol.ConfigMap != nil:
			source := &core.ConfigMap{}
			key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.ConfigMap.Name}
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
			target.SetAnnotations(source.GetAnnotations())
			r.Labeler.SetAnnotation(&target, AnnSource, key.String())
			list = append(list, target)
		}
	}
	return
}

func (r *Builder) Secrets(vm *planapi.VMStatus) (list []core.Secret, err error) {
	virtualMachine := &model.VM{}
	err = r.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}
	for _, vol := range virtualMachine.Object.Spec.Template.Spec.Volumes {
		switch {
		case vol.Secret != nil:
			source := &core.Secret{}
			key := types.NamespacedName{Namespace: virtualMachine.Namespace, Name: vol.Secret.SecretName}
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
	}
	return
}
