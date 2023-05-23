package plan

import (
	"context"
	"errors"
	"fmt"
	"path"

	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	refapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	"github.com/konveyor/forklift-controller/pkg/controller/validation"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libref "github.com/konveyor/forklift-controller/pkg/lib/ref"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	WarmMigrationNotReady        = "WarmMigrationNotReady"
	NamespaceNotValid            = "NamespaceNotValid"
	TransferNetNotValid          = "TransferNetworkNotValid"
	NetRefNotValid               = "NetworkMapRefNotValid"
	NetMapNotReady               = "NetworkMapNotReady"
	DsMapNotReady                = "StorageMapNotReady"
	DsRefNotValid                = "StorageRefNotValid"
	VMRefNotValid                = "VMRefNotValid"
	VMNotFound                   = "VMNotFound"
	VMAlreadyExists              = "VMAlreadyExists"
	VMNetworksNotMapped          = "VMNetworksNotMapped"
	VMStorageNotMapped           = "VMStorageNotMapped"
	VMMultiplePodNetworkMappings = "VMMultiplePodNetworkMappings"
	HostNotReady                 = "HostNotReady"
	DuplicateVM                  = "DuplicateVM"
	NameNotValid                 = "TargetNameNotValid"
	HookNotValid                 = "HookNotValid"
	HookNotReady                 = "HookNotReady"
	HookStepNotValid             = "HookStepNotValid"
	Executing                    = "Executing"
	Succeeded                    = "Succeeded"
	Failed                       = "Failed"
	Canceled                     = "Canceled"
	Deleted                      = "Deleted"
	Paused                       = "Paused"
	Pending                      = "Pending"
	Running                      = "Running"
	Blocked                      = "Blocked"
	Archived                     = "Archived"
	VDDKNotConfigured            = "VDDKNotConfigured"
)

// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

// Reasons
const (
	NotSet            = "NotSet"
	NotFound          = "NotFound"
	NotUnique         = "NotUnique"
	NotSupported      = "NotSupported"
	Ambiguous         = "Ambiguous"
	NotValid          = "NotValid"
	Modified          = "Modified"
	UserRequested     = "UserRequested"
	InMaintenanceMode = "InMaintenanceMode"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

// Validate the plan resource.
func (r *Reconciler) validate(plan *api.Plan) error {
	// Provider.
	pv := validation.ProviderPair{Client: r}
	conditions, err := pv.Validate(plan.Spec.Provider)
	if err != nil {
		return liberr.Wrap(err)
	}
	plan.Status.SetCondition(conditions.List...)
	if plan.Status.HasCondition(validation.SourceProviderNotReady) {
		return nil
	}
	plan.Referenced.Provider.Source = pv.Referenced.Source
	plan.Referenced.Provider.Destination = pv.Referenced.Destination
	//
	// Target namespace
	err = r.validateTargetNamespace(plan)
	if err != nil {
		return err
	}
	//
	// Mapping
	err = r.validateNetworkMap(plan)
	if err != nil {
		return err
	}
	err = r.validateStorageMap(plan)
	if err != nil {
		return err
	}
	//
	// Warm migration
	err = r.validateWarmMigration(plan)
	if err != nil {
		return err
	}
	//
	// VM list.
	err = r.validateVM(plan)
	if err != nil {
		return err
	}
	//
	// Transfer network
	err = r.validateTransferNetwork(plan)
	if err != nil {
		return err
	}
	// VM Hooks.
	err = r.validateHooks(plan)
	if err != nil {
		return err
	}
	// VDDK image
	err = r.validateVddkImage(plan)
	if err != nil {
		return err
	}

	return nil
}

// Validate that warm migration is supported from the source provider.
func (r *Reconciler) validateWarmMigration(plan *api.Plan) (err error) {
	if !plan.Spec.Warm {
		return
	}
	provider := plan.Referenced.Provider.Source
	if provider == nil {
		return nil
	}
	pAdapter, err := adapter.New(provider)
	if err != nil {
		return err
	}
	validator, err := pAdapter.Validator(plan)
	if err != nil {
		return err
	}
	if !validator.WarmMigration() {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     WarmMigrationNotReady,
			Status:   True,
			Category: Critical,
			Reason:   NotSupported,
			Message:  "Warm migration from the source provider is not supported.",
		})
	}
	return
}

// Validate the target namespace.
func (r *Reconciler) validateTargetNamespace(plan *api.Plan) (err error) {
	newCnd := libcnd.Condition{
		Type:     NamespaceNotValid,
		Status:   True,
		Category: Critical,
		Message:  "Target namespace is not valid.",
	}
	if plan.Spec.TargetNamespace == "" {
		newCnd.Reason = NotSet
		plan.Status.SetCondition(newCnd)
		return
	}
	if len(k8svalidation.IsDNS1123Label(plan.Spec.TargetNamespace)) > 0 {
		newCnd.Reason = NotValid
		plan.Status.SetCondition(newCnd)
	}
	return
}

// Validate network mapping ref.
func (r *Reconciler) validateNetworkMap(plan *api.Plan) (err error) {
	ref := plan.Spec.Map.Network
	newCnd := libcnd.Condition{
		Type:     NetRefNotValid,
		Status:   True,
		Category: Critical,
		Message:  "Map.Network is not valid.",
	}
	if !libref.RefSet(&ref) {
		newCnd.Reason = NotSet
		plan.Status.SetCondition(newCnd)
		return
	}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	mp := &api.NetworkMap{}
	err = r.Get(context.TODO(), key, mp)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		plan.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !mp.Status.HasCondition(libcnd.Ready) {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     NetMapNotReady,
			Status:   True,
			Category: Critical,
			Message:  "Map.Network does not have Ready condition.",
		})
	}

	plan.Referenced.Map.Network = mp

	return
}

// Validate storage mapping ref.
func (r *Reconciler) validateStorageMap(plan *api.Plan) (err error) {
	ref := plan.Spec.Map.Storage
	newCnd := libcnd.Condition{
		Type:     DsRefNotValid,
		Status:   True,
		Category: Critical,
		Message:  "Map.Storage is not valid.",
	}
	if !libref.RefSet(&ref) {
		newCnd.Reason = NotSet
		plan.Status.SetCondition(newCnd)
		return
	}
	key := client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
	mp := &api.StorageMap{}
	err = r.Get(context.TODO(), key, mp)
	if k8serr.IsNotFound(err) {
		err = nil
		newCnd.Reason = NotFound
		plan.Status.SetCondition(newCnd)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if !mp.Status.HasCondition(libcnd.Ready) {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     DsMapNotReady,
			Status:   True,
			Category: Critical,
			Message:  "Map.Storage does not have Ready condition.",
		})
	}

	plan.Referenced.Map.Storage = mp

	return
}

// Validate listed VMs.
func (r *Reconciler) validateVM(plan *api.Plan) error {
	if plan.Status.HasCondition(Executing) {
		return nil
	}
	notFound := libcnd.Condition{
		Type:     VMNotFound,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "VM not found.",
		Items:    []string{},
	}
	notUnique := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   NotUnique,
		Category: Critical,
		Message:  "Duplicate (source) VM.",
		Items:    []string{},
	}
	ambiguous := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   Ambiguous,
		Category: Critical,
		Message:  "VM reference is ambiguous.",
		Items:    []string{},
	}
	nameNotValid := libcnd.Condition{
		Type:     NameNotValid,
		Status:   True,
		Reason:   NotValid,
		Category: Warn,
		Message:  "Target VM name does not comply with DNS1123 RFC, will be automatically changed.",
		Items:    []string{},
	}
	alreadyExists := libcnd.Condition{
		Type:     VMAlreadyExists,
		Status:   True,
		Reason:   NotUnique,
		Category: Critical,
		Message:  "Target VM already exists.",
		Items:    []string{},
	}
	unmappedNetwork := libcnd.Condition{
		Type:     VMNetworksNotMapped,
		Status:   True,
		Reason:   NotValid,
		Category: Critical,
		Message:  "VM has unmapped networks.",
		Items:    []string{},
	}
	unmappedStorage := libcnd.Condition{
		Type:     VMStorageNotMapped,
		Status:   True,
		Reason:   NotValid,
		Category: Critical,
		Message:  "VM has unmapped storage.",
		Items:    []string{},
	}
	maintenanceMode := libcnd.Condition{
		Type:     HostNotReady,
		Status:   True,
		Reason:   InMaintenanceMode,
		Category: Warn,
		Message:  "VM host is in maintenance mode.",
		Items:    []string{},
	}
	multiplePodNetworkMappings := libcnd.Condition{
		Type:     VMMultiplePodNetworkMappings,
		Status:   True,
		Reason:   NotValid,
		Category: Critical,
		Message:  "VM has more than one interface mapped to the pod network.",
		Items:    []string{},
	}

	setOf := map[string]bool{}
	//
	// Referenced VMs.
	for i := range plan.Spec.VMs {
		ref := &plan.Spec.VMs[i].Ref
		if ref.NotSet() {
			plan.Status.SetCondition(libcnd.Condition{
				Type:     VMRefNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: Critical,
				Message:  "Either `ID` or `Name` required.",
			})
			continue
		}
		// Source.
		provider := plan.Referenced.Provider.Source
		if provider == nil {
			return nil
		}
		inventory, pErr := web.NewClient(provider)
		if pErr != nil {
			return liberr.Wrap(pErr)
		}
		_, pErr = inventory.VM(ref)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				notFound.Items = append(notFound.Items, ref.String())
				continue
			}
			if errors.As(pErr, &web.RefNotUniqueError{}) {
				ambiguous.Items = append(ambiguous.Items, ref.String())
				continue
			}
			return liberr.Wrap(pErr)
		}
		if len(k8svalidation.IsDNS1123Label(ref.Name)) > 0 {
			nameNotValid.Items = append(nameNotValid.Items, ref.String())
		}
		if _, found := setOf[ref.ID]; found {
			notUnique.Items = append(notUnique.Items, ref.String())
		} else {
			setOf[ref.ID] = true
		}
		pAdapter, err := adapter.New(provider)
		if err != nil {
			return err
		}
		validator, err := pAdapter.Validator(plan)
		if err != nil {
			return err
		}
		if plan.Referenced.Map.Network != nil {
			ok, err := validator.NetworksMapped(*ref)
			if err != nil {
				return err
			}
			if !ok {
				unmappedNetwork.Items = append(unmappedNetwork.Items, ref.String())
			}
			ok, err = validator.PodNetwork(*ref)
			if err != nil {
				return err
			}
			if !ok {
				multiplePodNetworkMappings.Items = append(multiplePodNetworkMappings.Items, ref.String())
			}
		}
		if plan.Referenced.Map.Storage != nil {
			ok, err := validator.StorageMapped(*ref)
			if err != nil {
				return err
			}
			if !ok {
				unmappedStorage.Items = append(unmappedStorage.Items, ref.String())
			}
		}
		ok, err := validator.MaintenanceMode(*ref)
		if err != nil {
			return err
		}
		if !ok {
			maintenanceMode.Items = append(maintenanceMode.Items, ref.String())
		}
		// Destination.
		provider = plan.Referenced.Provider.Destination
		if provider == nil {
			return nil
		}
		inventory, pErr = web.NewClient(provider)
		if pErr != nil {
			return liberr.Wrap(pErr)
		}
		id := path.Join(
			plan.Spec.TargetNamespace,
			ref.Name)
		_, pErr = inventory.VM(&refapi.Ref{Name: id})
		if pErr == nil {
			if _, found := plan.Status.Migration.FindVM(*ref); !found {
				// This VM is preexisting or is being managed by a
				// different migration plan.
				alreadyExists.Items = append(
					alreadyExists.Items,
					ref.String())
			}
		} else {
			if !errors.As(pErr, &web.NotFoundError{}) {
				return liberr.Wrap(pErr)
			}
		}
	}
	if len(notFound.Items) > 0 {
		plan.Status.SetCondition(notFound)
	}
	if len(notUnique.Items) > 0 {
		plan.Status.SetCondition(notUnique)
	}
	if len(alreadyExists.Items) > 0 {
		plan.Status.SetCondition(alreadyExists)
	}
	if len(nameNotValid.Items) > 0 {
		plan.Status.SetCondition(nameNotValid)
	}
	if len(ambiguous.Items) > 0 {
		plan.Status.SetCondition(ambiguous)
	}
	if len(unmappedNetwork.Items) > 0 {
		plan.Status.SetCondition(unmappedNetwork)
	}
	if len(unmappedStorage.Items) > 0 {
		plan.Status.SetCondition(unmappedStorage)
	}
	if len(maintenanceMode.Items) > 0 {
		plan.Status.SetCondition(maintenanceMode)
	}
	if len(multiplePodNetworkMappings.Items) > 0 {
		plan.Status.SetCondition(multiplePodNetworkMappings)
	}

	return nil
}

// Validate transfer network selection.
func (r *Reconciler) validateTransferNetwork(plan *api.Plan) (err error) {
	if plan.Spec.TransferNetwork == nil {
		return
	}
	notFound := libcnd.Condition{
		Type:     TransferNetNotValid,
		Status:   True,
		Category: Critical,
		Reason:   NotFound,
		Message:  "Transfer network is not valid.",
	}
	key := client.ObjectKey{
		Namespace: plan.Spec.TransferNetwork.Namespace,
		Name:      plan.Spec.TransferNetwork.Name,
	}
	netAttachDef := &net.NetworkAttachmentDefinition{}
	err = r.Get(context.TODO(), key, netAttachDef)
	if k8serr.IsNotFound(err) {
		err = nil
		plan.Status.SetCondition(notFound)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

// Validate referenced hooks.
func (r *Reconciler) validateHooks(plan *api.Plan) (err error) {
	notSet := libcnd.Condition{
		Type:     HookNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "Hook specified by: `namespace` and `name`.",
		Items:    []string{},
	}
	notFound := libcnd.Condition{
		Type:     HookNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "Hook not found.",
		Items:    []string{},
	}
	notReady := libcnd.Condition{
		Type:     HookNotReady,
		Status:   True,
		Reason:   NotFound,
		Category: Critical,
		Message:  "Hook does not have `Ready` condition.",
		Items:    []string{},
	}
	stepNotValid := libcnd.Condition{
		Type:     HookStepNotValid,
		Status:   True,
		Reason:   NotValid,
		Category: Critical,
		Message:  "Hook step not valid.",
		Items:    []string{},
	}
	for _, vm := range plan.Spec.VMs {
		for _, ref := range vm.Hooks {
			// Step not valid.
			if _, found := map[string]int{PreHook: 1, PostHook: 1}[ref.Step]; !found {
				description := fmt.Sprintf(
					"VM: %s step: %s",
					vm.String(),
					ref.Step)
				stepNotValid.Items = append(
					stepNotValid.Items,
					description)
			}
			// Not Set.
			if !libref.RefSet(&ref.Hook) {
				description := fmt.Sprintf("VM: %s", vm.String())
				notSet.Items = append(
					notSet.Items,
					description)
				continue
			}
			// Not Found.
			hook := &api.Hook{}
			err = r.Get(
				context.TODO(),
				client.ObjectKey{
					Namespace: ref.Hook.Namespace,
					Name:      ref.Hook.Name,
				},
				hook)
			if err != nil {
				if k8serr.IsNotFound(err) {
					description := fmt.Sprintf(
						"VM: %s hook: %s",
						vm.String(),
						ref.Hook.String())
					notFound.Items = append(
						notFound.Items,
						description)
					continue
				} else {
					return
				}
			} else {
				plan.Referenced.Hooks = append(
					plan.Referenced.Hooks,
					hook)
			}
			// Not Ready.
			if !hook.Status.HasCondition(libcnd.Ready) {
				description := fmt.Sprintf(
					"VM: %s hook: %s",
					vm.String(),
					ref.Hook.String())
				notReady.Items = append(
					notReady.Items,
					description)
			}
		}
	}
	for _, cnd := range []libcnd.Condition{} {
		if len(cnd.Items) > 0 {
			plan.Status.SetCondition(cnd)
		}
	}

	return
}

func (r *Reconciler) validateVddkImage(plan *api.Plan) (err error) {
	vddkNotConfigured := libcnd.Condition{
		Type:     VDDKNotConfigured,
		Status:   True,
		Reason:   NotSet,
		Category: Critical,
		Message:  "VDDK image is necessary for this type of migration",
	}

	source := plan.Referenced.Provider.Source
	if source == nil {
		return nil
	}
	destination := plan.Referenced.Provider.Destination
	if destination == nil {
		return nil
	}

	if source.Type() != api.VSphere {
		// VDDK is used for other provider types
		return nil
	}

	el9, el9Err := plan.VSphereUsesEl9VirtV2v()
	if el9Err != nil {
		return el9Err
	}
	if el9 {
		// VDDK image is optional when EL9 virt-v2v image is in use
		return nil
	}

	if _, found := source.Spec.Settings["vddkInitImage"]; !found {
		plan.Status.SetCondition(vddkNotConfigured)
	}
	return nil
}
