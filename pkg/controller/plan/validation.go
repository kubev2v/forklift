package plan

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"path"
	"strconv"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/controller/validation"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/templateutil"
	batchv1 "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Types
const (
	WarmMigrationNotReady         = "WarmMigrationNotReady"
	NamespaceNotValid             = "NamespaceNotValid"
	TransferNetNotValid           = "TransferNetworkNotValid"
	NetRefNotValid                = "NetworkMapRefNotValid"
	NetMapNotReady                = "NetworkMapNotReady"
	DsMapNotReady                 = "StorageMapNotReady"
	DsRefNotValid                 = "StorageRefNotValid"
	VMRefNotValid                 = "VMRefNotValid"
	VMNotFound                    = "VMNotFound"
	VMAlreadyExists               = "VMAlreadyExists"
	VMNetworksNotMapped           = "VMNetworksNotMapped"
	VMStorageNotMapped            = "VMStorageNotMapped"
	VMStorageNotSupported         = "VMStorageNotSupported"
	VMMultiplePodNetworkMappings  = "VMMultiplePodNetworkMappings"
	VMMissingGuestIPs             = "VMMissingGuestIPs"
	VMMissingChangedBlockTracking = "VMMissingChangedBlockTracking"
	HostNotReady                  = "HostNotReady"
	DuplicateVM                   = "DuplicateVM"
	SharedDisks                   = "SharedDisks"
	SharedWarnDisks               = "SharedWarnDisks"
	NameNotValid                  = "TargetNameNotValid"
	HookNotValid                  = "HookNotValid"
	HookNotReady                  = "HookNotReady"
	HookStepNotValid              = "HookStepNotValid"
	Executing                     = "Executing"
	Succeeded                     = "Succeeded"
	Failed                        = "Failed"
	Canceled                      = "Canceled"
	Deleted                       = "Deleted"
	Paused                        = "Paused"
	Archived                      = "Archived"
	unsupportedVersion            = "UnsupportedVersion"
	VDDKInvalid                   = "VDDKInvalid"
	ValidatingVDDK                = "ValidatingVDDK"
	VDDKInitImageNotReady         = "VDDKInitImageNotReady"
	VDDKInitImageUnavailable      = "VDDKInitImageUnavailable"
	UnsupportedOvaSource          = "UnsupportedOvaSource"
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
	Started                     = "Started"
	NotSet                      = "NotSet"
	NotFound                    = "NotFound"
	NotUnique                   = "NotUnique"
	NotSupported                = "NotSupported"
	Ambiguous                   = "Ambiguous"
	NotValid                    = "NotValid"
	Modified                    = "Modified"
	UserRequested               = "UserRequested"
	InMaintenanceMode           = "InMaintenanceMode"
	MissingGuestInfo            = "MissingGuestInformation"
	MissingChangedBlockTracking = "MissingChangedBlockTracking"
)

// Statuses
const (
	True  = libcnd.True
	False = libcnd.False
)

const (
	Shareable = "shareable"
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

	if err := r.ensureSecretForProvider(plan); err != nil {
		return err
	}

	if err := r.validateTargetNamespace(plan); err != nil {
		return err
	}

	if err := r.validateNetworkMap(plan); err != nil {
		return err
	}

	if err := r.validateStorageMap(plan); err != nil {
		return err
	}

	if err := r.validateWarmMigration(plan); err != nil {
		return err
	}

	if err := r.validateVM(plan); err != nil {
		return err
	}

	if err := r.validateTransferNetwork(plan); err != nil {
		return err
	}

	if err := r.validateHooks(plan); err != nil {
		return err
	}

	if err := r.validateVddkImage(plan); err != nil {
		return err
	}

	// Validate version only if migration is OCP to OCP
	if err := r.validateOpenShiftVersion(plan); err != nil {
		return err
	}

	// Validate PVC name template
	if err := r.validatePVCNameTemplate(plan); err != nil {
		return err
	}

	// Validate volume name template
	if err := r.validateVolumeNameTemplate(plan); err != nil {
		return err
	}

	// Validate network name template
	if err := r.validateNetworkNameTemplate(plan); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) validatePVCNameTemplate(plan *api.Plan) error {
	if err := r.IsValidPVCNameTemplate(plan.Spec.PVCNameTemplate); err != nil {
		invalidPVCNameTemplate := libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryCritical,
			Message:  "PVC name template is invalid.",
			Items:    []string{},
		}

		plan.Status.SetCondition(invalidPVCNameTemplate)

		r.Log.Info("PVC name template is invalid", "error", err.Error(), "plan", plan.Name, "namespace", plan.Namespace)
	}

	return nil
}

func (r *Reconciler) validateVolumeNameTemplate(plan *api.Plan) error {
	if err := r.IsValidVolumeNameTemplate(plan.Spec.VolumeNameTemplate); err != nil {
		invalidPVCNameTemplate := libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryCritical,
			Message:  "Volume name template is invalid.",
			Items:    []string{},
		}

		plan.Status.SetCondition(invalidPVCNameTemplate)

		r.Log.Info("Volume name template is invalid", "error", err.Error(), "plan", plan.Name, "namespace", plan.Namespace)
	}

	return nil
}

func (r *Reconciler) validateNetworkNameTemplate(plan *api.Plan) error {
	if err := r.IsValidNetworkNameTemplate(plan.Spec.NetworkNameTemplate); err != nil {
		invalidPVCNameTemplate := libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryCritical,
			Message:  "Network name template is invalid.",
			Items:    []string{},
		}

		plan.Status.SetCondition(invalidPVCNameTemplate)

		r.Log.Info("Network name template is invalid", "error", err.Error(), "plan", plan.Name, "namespace", plan.Namespace)
	}

	return nil
}

func (r *Reconciler) validateOpenShiftVersion(plan *api.Plan) error {
	source := plan.Referenced.Provider.Source
	if source == nil {
		return nil
	}

	destination := plan.Referenced.Provider.Destination
	if destination == nil {
		return nil
	}

	if source.Type() == api.OpenShift && destination.Type() == api.OpenShift {
		unsupportedVersion := libcnd.Condition{
			Type:     unsupportedVersion,
			Status:   True,
			Reason:   NotSupported,
			Category: api.CategoryCritical,
			Message:  "Source version is not supported.",
			Items:    []string{},
		}

		restCfg := ocp.RestCfg(source, plan.Referenced.Secret)
		clientset, err := kubernetes.NewForConfig(restCfg)
		if err != nil {
			return liberr.Wrap(err)
		}

		err = r.checkOCPVersion(clientset)
		if err != nil {
			r.Log.Error(err, "check ocp version failed", "plan", plan)
			plan.Status.SetCondition(unsupportedVersion)
		}
	}

	return nil
}

func (r *Reconciler) ensureSecretForProvider(plan *api.Plan) error {
	if plan.Referenced.Provider.Source != nil &&
		plan.Referenced.Secret == nil &&
		!plan.Referenced.Provider.Source.IsHost() {
		err := r.setupSecret(plan)
		if err != nil {
			return err
		}
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
			Category: api.CategoryCritical,
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
		Category: api.CategoryCritical,
		Message:  "Target namespace is not valid.",
	}
	if plan.Spec.TargetNamespace == "" {
		newCnd.Reason = NotSet
		plan.Status.SetCondition(newCnd)
		return
	}
	if len(k8svalidation.IsDNS1123Subdomain(plan.Spec.TargetNamespace)) > 0 {
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
		Category: api.CategoryCritical,
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
			Category: api.CategoryCritical,
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
		Category: api.CategoryCritical,
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
			Category: api.CategoryCritical,
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
		Category: api.CategoryCritical,
		Message:  "VM not found.",
		Items:    []string{},
	}
	notUnique := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   NotUnique,
		Category: api.CategoryCritical,
		Message:  "Duplicate (source) VM.",
		Items:    []string{},
	}
	ambiguous := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   Ambiguous,
		Category: api.CategoryCritical,
		Message:  "VM reference is ambiguous.",
		Items:    []string{},
	}
	nameNotValid := libcnd.Condition{
		Type:     NameNotValid,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryWarn,
		Message:  "Target VM name does not comply with DNS1123 RFC, will be automatically changed.",
		Items:    []string{},
	}
	alreadyExists := libcnd.Condition{
		Type:     VMAlreadyExists,
		Status:   True,
		Reason:   NotUnique,
		Category: api.CategoryCritical,
		Message:  "Target VM already exists.",
		Items:    []string{},
	}
	unmappedNetwork := libcnd.Condition{
		Type:     VMNetworksNotMapped,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VM has unmapped networks.",
		Items:    []string{},
	}
	unmappedStorage := libcnd.Condition{
		Type:     VMStorageNotMapped,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VM has unmapped storage.",
		Items:    []string{},
	}
	unsupportedStorage := libcnd.Condition{
		Type:     VMStorageNotSupported,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VM has unsupported storage. Migration of Direct LUN/FC from oVirt is supported as from version 4.5.2.1",
		Items:    []string{},
	}
	maintenanceMode := libcnd.Condition{
		Type:     HostNotReady,
		Status:   True,
		Reason:   InMaintenanceMode,
		Category: api.CategoryWarn,
		Message:  "VM host is in maintenance mode.",
		Items:    []string{},
	}
	multiplePodNetworkMappings := libcnd.Condition{
		Type:     VMMultiplePodNetworkMappings,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VM has more than one interface mapped to the pod network.",
		Items:    []string{},
	}
	missingStaticIPs := libcnd.Condition{
		Type:     VMMissingGuestIPs,
		Status:   True,
		Reason:   MissingGuestInfo,
		Category: api.CategoryWarn,
		Message:  "Guest information on vNICs is missing, cannot preserve static IPs. If this machine has static IP, make sure VMware tools are installed and the VM is running.",
		Items:    []string{},
	}
	missingCbtForWarm := libcnd.Condition{
		Type:     VMMissingChangedBlockTracking,
		Status:   True,
		Reason:   MissingChangedBlockTracking,
		Category: api.CategoryCritical,
		Message:  "Changed Block Tracking (CBT) has not been enabled on some VM. This feature is a prerequisite for VM warm migration.",
		Items:    []string{},
	}
	pvcNameInvalid := libcnd.Condition{
		Type:     NotValid,
		Status:   True,
		Category: api.CategoryCritical,
		Message:  "VM PVC name template is invalid.",
		Items:    []string{},
	}
	volumeNameInvalid := libcnd.Condition{
		Type:     NotValid,
		Status:   True,
		Category: api.CategoryCritical,
		Message:  "VM volume name template is invalid.",
		Items:    []string{},
	}
	networkNameInvalid := libcnd.Condition{
		Type:     NotValid,
		Status:   True,
		Category: api.CategoryCritical,
		Message:  "VM network name template is invalid.",
		Items:    []string{},
	}
	targetNameInvalid := libcnd.Condition{
		Type:     NotValid,
		Status:   True,
		Category: api.CategoryCritical,
		Message:  "TargetName is invalid.",
		Items:    []string{},
	}
	targetNameNotUnique := libcnd.Condition{
		Type:     DuplicateVM,
		Status:   True,
		Reason:   NotUnique,
		Category: api.CategoryCritical,
		Message:  "Duplicate targetName.",
		Items:    []string{},
	}
	unsupportedOvaSource := libcnd.Condition{
		Type:     UnsupportedOvaSource,
		Status:   True,
		Category: api.CategoryWarn,
		Message:  "OVA appears to have been exported from an unsupported source, and may have issues during import.",
		Items:    []string{},
	}
	var sharedDisksConditions []libcnd.Condition
	setOf := map[string]bool{}
	setOfTargetName := map[string]bool{}
	//
	// Referenced VMs.
	for i := range plan.Spec.VMs {
		vm := &plan.Spec.VMs[i]
		ref := &vm.Ref
		if ref.NotSet() {
			plan.Status.SetCondition(libcnd.Condition{
				Type:     VMRefNotValid,
				Status:   True,
				Reason:   NotSet,
				Category: api.CategoryCritical,
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
		v, pErr := inventory.VM(ref)
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
		if vm.TargetName == "" {
			if len(k8svalidation.IsDNS1123Subdomain(ref.Name)) > 0 {
				// if source VM name is not valid
				nameNotValid.Items = append(nameNotValid.Items, ref.String())
			}
		} else {
			if len(k8svalidation.IsDNS1123Subdomain(vm.TargetName)) > 0 {
				// if a manually assigned target name is not valid
				targetNameInvalid.Items = append(targetNameInvalid.Items, ref.String())
			}
		}
		// check if vm ID is unique
		if _, found := setOf[ref.ID]; found {
			notUnique.Items = append(notUnique.Items, ref.String())
		} else {
			setOf[ref.ID] = true
		}
		// check if targetName is unique
		if vm.TargetName != "" {
			if _, found := setOfTargetName[vm.TargetName]; found {
				targetNameNotUnique.Items = append(targetNameNotUnique.Items, ref.String())
			} else {
				setOfTargetName[vm.TargetName] = true
			}
		}
		// check for supported OVA source
		if ova, ok := v.(*ova.VM); ok {
			for _, concern := range ova.Concerns {
				// match label from ova/export_source.rego
				if concern.Id == "ova.source.unsupported" {
					unsupportedOvaSource.Items = append(unsupportedOvaSource.Items, ref.String())
				}
			}
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
			ok, err = validator.DirectStorage(*ref)
			if err != nil {
				return err
			}
			if !ok {
				unsupportedStorage.Items = append(unsupportedStorage.Items, ref.String())
			}
		}
		ok, err := validator.MaintenanceMode(*ref)
		if err != nil {
			return err
		}
		if !ok {
			maintenanceMode.Items = append(maintenanceMode.Items, ref.String())
		}
		ok, err = validator.StaticIPs(*ref)
		if err != nil {
			return err
		}
		if !ok {
			missingStaticIPs.Items = append(missingStaticIPs.Items, ref.String())
		}

		var ctx *plancontext.Context
		ctx, err = plancontext.New(r, plan, r.Log)
		if err != nil {
			return err
		}
		ok, msg, category, err := validator.SharedDisks(*ref, ctx.Destination.Client)
		if err != nil {
			return err
		}
		if !ok {
			sharedDisks := libcnd.Condition{
				Type:     SharedWarnDisks,
				Status:   True,
				Category: category,
				Message:  "VMs with shared disk can not be migrated.", // This should be set by the provider validator
				Items:    []string{ref.String()},
			}
			if msg != "" {
				sharedDisks.Message = msg
			}
			if category == validation.Warn {
				sharedDisks.Type = SharedWarnDisks
			} else {
				sharedDisks.Type = SharedDisks
			}
			sharedDisks.Type = fmt.Sprintf("%s-%s", sharedDisks.Type, ref.ID)
			sharedDisksConditions = append(sharedDisksConditions, sharedDisks)
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
		if vm.TargetName != "" {
			// if target name is provided, use it to look for existing VMs
			id = path.Join(
				plan.Spec.TargetNamespace,
				vm.TargetName)
		}
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
		// Warm migration.
		if plan.Spec.Warm {
			enabled, err := validator.ChangeTrackingEnabled(*ref)
			if err != nil {
				return err
			}
			if !enabled {
				missingCbtForWarm.Items = append(missingCbtForWarm.Items, ref.String())
			}
		}
		// is valid vm pvc name template
		if vm.PVCNameTemplate != "" {
			if err := r.IsValidPVCNameTemplate(vm.PVCNameTemplate); err != nil {
				pvcNameInvalid.Items = append(pvcNameInvalid.Items, ref.String())
			}
		}
		// is valid vm pvc name template
		if vm.VolumeNameTemplate != "" {
			if err := r.IsValidVolumeNameTemplate(vm.VolumeNameTemplate); err != nil {
				volumeNameInvalid.Items = append(volumeNameInvalid.Items, ref.String())
			}
		}
		// is valid vm pvc name template
		if vm.NetworkNameTemplate != "" {
			if err := r.IsValidNetworkNameTemplate(vm.NetworkNameTemplate); err != nil {
				networkNameInvalid.Items = append(networkNameInvalid.Items, ref.String())
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
	if len(unsupportedStorage.Items) > 0 {
		plan.Status.SetCondition(unsupportedStorage)
	}
	if len(maintenanceMode.Items) > 0 {
		plan.Status.SetCondition(maintenanceMode)
	}
	if len(multiplePodNetworkMappings.Items) > 0 {
		plan.Status.SetCondition(multiplePodNetworkMappings)
	}
	if len(missingStaticIPs.Items) > 0 {
		plan.Status.SetCondition(missingStaticIPs)
	}
	if len(sharedDisksConditions) > 0 {
		plan.Status.SetCondition(sharedDisksConditions...)
	}
	if len(missingCbtForWarm.Items) > 0 {
		plan.Status.SetCondition(missingCbtForWarm)
	}
	if len(pvcNameInvalid.Items) > 0 {
		plan.Status.SetCondition(pvcNameInvalid)
	}
	if len(volumeNameInvalid.Items) > 0 {
		plan.Status.SetCondition(volumeNameInvalid)
	}
	if len(networkNameInvalid.Items) > 0 {
		plan.Status.SetCondition(networkNameInvalid)
	}
	if len(targetNameInvalid.Items) > 0 {
		plan.Status.SetCondition(targetNameInvalid)
	}
	if len(targetNameNotUnique.Items) > 0 {
		plan.Status.SetCondition(targetNameNotUnique)
	}
	if len(unsupportedOvaSource.Items) > 0 {
		plan.Status.SetCondition(unsupportedOvaSource)
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
		Category: api.CategoryCritical,
		Reason:   NotFound,
		Message:  "Transfer network is not valid.",
	}
	notValid := libcnd.Condition{
		Type:     TransferNetNotValid,
		Status:   True,
		Category: api.CategoryCritical,
		Reason:   NotValid,
		Message:  "Transfer network default route annotation is not a valid IP address.",
	}
	key := client.ObjectKey{
		Namespace: plan.Spec.TransferNetwork.Namespace,
		Name:      plan.Spec.TransferNetwork.Name,
	}
	netAttachDef := &k8snet.NetworkAttachmentDefinition{}
	err = r.Get(context.TODO(), key, netAttachDef)
	if k8serr.IsNotFound(err) {
		err = nil
		plan.Status.SetCondition(notFound)
		return
	}
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	route, found := netAttachDef.Annotations[AnnForkliftNetworkRoute]
	if !found {
		return
	}
	ip := net.ParseIP(route)
	if ip == nil {
		plan.Status.SetCondition(notValid)
	}

	return
}

// Validate referenced hooks.
func (r *Reconciler) validateHooks(plan *api.Plan) (err error) {
	notSet := libcnd.Condition{
		Type:     HookNotValid,
		Status:   True,
		Reason:   NotSet,
		Category: api.CategoryCritical,
		Message:  "Hook specified by: `namespace` and `name`.",
		Items:    []string{},
	}
	notFound := libcnd.Condition{
		Type:     HookNotValid,
		Status:   True,
		Reason:   NotFound,
		Category: api.CategoryCritical,
		Message:  "Hook not found.",
		Items:    []string{},
	}
	notReady := libcnd.Condition{
		Type:     HookNotReady,
		Status:   True,
		Reason:   NotFound,
		Category: api.CategoryCritical,
		Message:  "Hook does not have `Ready` condition.",
		Items:    []string{},
	}
	stepNotValid := libcnd.Condition{
		Type:     HookStepNotValid,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "Hook step not valid.",
		Items:    []string{},
	}
	for _, vm := range plan.Spec.VMs {
		for _, ref := range vm.Hooks {
			// Step not valid.
			if _, found := map[string]int{api.PhasePreHook: 1, api.PhasePostHook: 1}[ref.Step]; !found {
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
	source := plan.Referenced.Provider.Source
	if source == nil {
		return liberr.New("source provider is not set")
	}
	destination := plan.Referenced.Provider.Destination
	if destination == nil {
		return liberr.New("destination provider is not set")
	}

	if source.Type() != api.VSphere {
		// VDDK is not used for other provider types
		return
	}

	vddkImage := settings.GetVDDKImage(source.Spec.Settings)
	if vddkImage != "" {
		var job *batchv1.Job
		if job, err = r.ensureVddkImageValidationJob(plan); err != nil {
			return
		}
		err = r.validateVddkImageJob(job, plan)
	}
	if plan.Spec.Warm && vddkImage == "" {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     VDDKInitImageUnavailable,
			Status:   True,
			Reason:   NotSet,
			Category: api.CategoryCritical,
			Message:  "VDDK image not set on the provider, this is required for the warm migration",
		})
	}

	return
}

func jobExceedsDeadline(job *batchv1.Job) bool {
	ActiveDeadlineSeconds := settings.Settings.Migration.VddkJobActiveDeadline

	if job.Status.StartTime == nil {
		return false
	}
	return meta.Now().Sub(job.Status.StartTime.Time).Seconds() > float64(ActiveDeadlineSeconds)
}

func (r *Reconciler) validateVddkImageJob(job *batchv1.Job, plan *api.Plan) (err error) {
	image := settings.GetVDDKImage(plan.Referenced.Provider.Source.Spec.Settings)
	vddkInvalid := libcnd.Condition{
		Type:     VDDKInvalid,
		Status:   True,
		Reason:   NotSet,
		Category: api.CategoryCritical,
		Message:  "VDDK init image is invalid",
	}
	vddkValidationInProgress := libcnd.Condition{
		Type:     ValidatingVDDK,
		Status:   True,
		Reason:   Started,
		Category: api.CategoryAdvisory,
		Message:  "Validating VDDK init image",
	}

	if len(job.Status.Conditions) == 0 {
		r.Log.Info("validation of VDDK job is in progress", "image", image)
		plan.Status.SetCondition(vddkValidationInProgress)
	}
	var ctx *plancontext.Context
	ctx, err = plancontext.New(r, plan, r.Log)
	if err != nil {
		return
	}
	// check if a pod exists for the job
	pods := &core.PodList{}
	if err = ctx.Destination.Client.List(context.TODO(), pods, &client.ListOptions{
		Namespace:     plan.Spec.TargetNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{"job-name": job.Name}),
	}); err != nil {
		return
	}
	if len(pods.Items) > 0 {
		pod := pods.Items[0]
		if len(pod.Status.InitContainerStatuses) == 0 {
			return liberr.New("Validation pod doesn't contain expected init container", "pod", pod)
		}
		waiting := pod.Status.InitContainerStatuses[0].State.Waiting
		if waiting != nil {
			if jobExceedsDeadline(job) {
				// If we've exceeded the deadline, set a `warning` condition to increase
				// severity. Don't set it as `critical` because the job will continue retrying
				// indefinitely until the pull succeeds or the provider's vddk init image URL is
				// updated.
				plan.Status.SetCondition(libcnd.Condition{
					Type:     VDDKInitImageUnavailable,
					Status:   True,
					Reason:   waiting.Reason,
					Category: api.CategoryCritical,
					Message:  "Unable to Pull VDDK init image. Check that the image URL is correct.",
				})
			} else {
				plan.Status.SetCondition(libcnd.Condition{
					Type:     VDDKInitImageNotReady,
					Status:   True,
					Reason:   waiting.Reason,
					Category: api.CategoryAdvisory,
					Message:  waiting.Message,
				})
			}
		} else {
			plan.Status.DeleteCondition(VDDKInitImageNotReady)
		}
	}
	for _, condition := range job.Status.Conditions {
		switch condition.Type {
		case batchv1.JobComplete:
			r.Log.Info("validate VDDK job completed", "image", image)
			err = nil
			return
		case batchv1.JobFailed:
			plan.Status.SetCondition(vddkInvalid)
			err = nil
			return
		default:
			err = liberr.New("validation of VDDK job has an unexpected condition", "type", condition.Type)
		}
	}

	return
}

// Cancel all other vddk validation jobs that are currently running for the
// plan. This is necessary because validation jobs do not have a deadline,
// so they will keep trying indefinitely if they can't pull the image. If the
// VDDK URL is later changed, we will launch a new validation job, and the old
// validation job is no longer relevant, so we can just kill it.
func (r *Reconciler) cancelOtherActiveVddkCheckJobs(plan *api.Plan) (err error) {
	ctx, err := plancontext.New(r, plan, r.Log)
	if err != nil {
		return
	}
	jobLabels := getVddkImageValidationJobLabels(ctx.Plan)

	queryLabels := make(map[string]string, 1)
	queryLabels["plan"] = jobLabels["plan"]
	delete(queryLabels, "vddk")

	jobs := &batchv1.JobList{}
	if err = ctx.Destination.Client.List(
		context.TODO(),
		jobs,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(queryLabels),
			Namespace:     plan.Spec.TargetNamespace,
		},
	); err != nil {
		return
	}

	for _, job := range jobs.Items {
		if _, found := job.Labels["vddk"]; !found {
			continue
		}
		if job.Status.Active > 0 && job.Labels["vddk"] != jobLabels["vddk"] {
			r.Log.Info("Another validation job is active for this plan. Stopping...", "job", job)
			// make sure to delete the pod associated with this job so that it doesn't
			// become orphaned while trying to pull its image indefinitely
			fg := meta.DeletePropagationForeground
			opts := &client.DeleteOptions{PropagationPolicy: &fg}
			if err = ctx.Destination.Client.Delete(context.TODO(), &job, opts); err != nil {
				return
			}
		}
	}

	return nil
}

func (r *Reconciler) ensureVddkImageValidationJob(plan *api.Plan) (*batchv1.Job, error) {
	ctx, err := plancontext.New(r, plan, r.Log)
	if err != nil {
		return nil, err
	}

	if err = r.ensureNamespace(ctx); err != nil {
		return nil, liberr.Wrap(err)
	}

	if err := r.cancelOtherActiveVddkCheckJobs(ctx.Plan); err != nil {
		return nil, liberr.Wrap(err)
	}

	jobLabels := getVddkImageValidationJobLabels(ctx.Plan)
	jobs := &batchv1.JobList{}
	err = ctx.Destination.Client.List(
		context.TODO(),
		jobs,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(jobLabels),
			Namespace:     plan.Spec.TargetNamespace,
		},
	)
	switch {
	case err != nil:
		return nil, err
	case len(jobs.Items) == 0:
		job := createVddkCheckJob(ctx.Plan)
		err = ctx.Destination.Client.Create(context.Background(), job)
		if err != nil {
			return nil, err
		}
		return job, nil
	default:
		return &jobs.Items[0], nil
	}
}

func (r *Reconciler) ensureNamespace(ctx *plancontext.Context) error {
	err := ensureNamespace(ctx.Plan, ctx.Destination.Client)
	if err == nil {
		r.Log.Info(
			"Created namespace.",
			"import",
			ctx.Plan.Spec.TargetNamespace)
	}
	return err
}

func getVddkImageValidationJobLabels(plan *api.Plan) map[string]string {
	image := settings.GetVDDKImage(plan.Referenced.Provider.Source.Spec.Settings)
	sum := md5.Sum([]byte(image))
	return map[string]string{
		"plan": string(plan.ObjectMeta.UID),
		"vddk": hex.EncodeToString(sum[:]),
	}
}

func createVddkCheckJob(plan *api.Plan) *batchv1.Job {
	image := settings.GetVDDKImage(plan.Referenced.Provider.Source.Spec.Settings)

	mount := core.VolumeMount{
		Name:      VddkVolumeName,
		MountPath: "/opt",
	}
	initContainers := []core.Container{
		{
			Name:            "vddk-side-car",
			Image:           image,
			ImagePullPolicy: core.PullIfNotPresent,
			VolumeMounts:    []core.VolumeMount{mount},
			SecurityContext: &core.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities: &core.Capabilities{
					Drop: []core.Capability{"ALL"},
				},
			},
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceCPU:    resource.MustParse("100m"),
					core.ResourceMemory: resource.MustParse("150Mi"),
				},
				Limits: core.ResourceList{
					core.ResourceCPU:    resource.MustParse("1000m"),
					core.ResourceMemory: resource.MustParse("500Mi"),
				},
			},
		},
	}

	volumes := []core.Volume{
		{
			Name: VddkVolumeName,
			VolumeSource: core.VolumeSource{
				EmptyDir: &core.EmptyDirVolumeSource{},
			},
		},
	}
	psc := &core.PodSecurityContext{
		SeccompProfile: &core.SeccompProfile{
			Type: core.SeccompProfileTypeRuntimeDefault,
		},
	}
	if !Settings.OpenShift {
		psc.RunAsNonRoot = ptr.To(true)
		psc.RunAsUser = ptr.To(qemuUser)
	}
	return &batchv1.Job{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("vddk-validator-%s", plan.Name),
			Namespace:    plan.Spec.TargetNamespace,
			Labels:       getVddkImageValidationJobLabels(plan),
			Annotations: map[string]string{
				"provider": plan.Referenced.Provider.Source.Name,
				"vddk":     image,
				"plan":     plan.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To[int32](2),
			Completions:  ptr.To[int32](1),
			Template: core.PodTemplateSpec{
				Spec: core.PodSpec{
					SecurityContext: psc,
					RestartPolicy:   core.RestartPolicyNever,
					InitContainers:  initContainers,
					Containers: []core.Container{
						{
							Name: "validator",
							Resources: core.ResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceCPU:    resource.MustParse("100m"),
									core.ResourceMemory: resource.MustParse("150Mi"),
								},
								Limits: core.ResourceList{
									core.ResourceCPU:    resource.MustParse("1000m"),
									core.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
							Image: Settings.Migration.VirtV2vImage,
							SecurityContext: &core.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &core.Capabilities{
									Drop: []core.Capability{"ALL"},
								},
							},
							VolumeMounts: []core.VolumeMount{mount},
							Command:      []string{"file", "-E", "/opt/vmware-vix-disklib-distrib/lib64/libvixDiskLib.so"},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func (r *Reconciler) setupSecret(plan *api.Plan) (err error) {
	key := client.ObjectKey{
		Namespace: plan.Referenced.Provider.Source.Spec.Secret.Namespace,
		Name:      plan.Referenced.Provider.Source.Spec.Secret.Name,
	}

	secret := v1.Secret{}
	err = r.Get(context.TODO(), key, &secret)
	if err != nil {
		return
	}

	plan.Referenced.Secret = &secret
	return
}

func (r *Reconciler) checkOCPVersion(clientset kubernetes.Interface) error {
	discoveryClient := clientset.Discovery()
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return liberr.Wrap(err)
	}

	major, err := strconv.Atoi(version.Major)
	if err != nil {
		return liberr.Wrap(err)
	}

	minor, err := strconv.Atoi(version.Minor)
	if err != nil {
		return liberr.Wrap(err)
	}

	if major < 1 || (major == 1 && minor < 26) {
		return liberr.New("source provider version is not supported")
	}

	return nil
}

func (r *Reconciler) IsValidTemplate(templateStr string, testData interface{}) (string, error) {
	// Execute the template with test data
	result, err := templateutil.ExecuteTemplate(templateStr, testData)
	if err != nil {
		return "", liberr.Wrap(err, "Template execution failed")
	}

	// Empty output is not valid
	if result == "" {
		return "", liberr.New("Template output is empty")
	}

	return result, nil
}

func (r *Reconciler) IsValidPVCNameTemplate(pvcNameTemplate string) error {
	if pvcNameTemplate == "" {
		return nil
	}

	// Test template with sample data
	testData := api.PVCNameTemplateData{
		VmName:         "test-vm",
		PlanName:       "test-plan",
		DiskIndex:      0,
		RootDiskIndex:  0,
		Shared:         false,
		FileName:       "[test07_ds1] test_sp/test-000001.vmdk",
		WinDriveLetter: "c",
	}

	result, err := r.IsValidTemplate(pvcNameTemplate, testData)
	if err != nil {
		return err
	}

	// Validate that template output is a valid k8s label
	errs := k8svalidation.IsDNS1123Label(result)
	if len(errs) > 0 {
		errMsg := fmt.Sprintf("Template output is not a valid k8s label [%s]", result)
		return liberr.New(errMsg, errs)
	}

	return nil
}

func (r *Reconciler) IsValidVolumeNameTemplate(volumeNameTemplate string) error {
	if volumeNameTemplate == "" {
		return nil
	}

	testData := api.VolumeNameTemplateData{
		PVCName:     "test-pvc",
		VolumeIndex: 0,
	}

	result, err := r.IsValidTemplate(volumeNameTemplate, testData)
	if err != nil {
		return err
	}

	// Validate that template output is a valid k8s label
	errs := k8svalidation.IsDNS1123Label(result)
	if len(errs) > 0 {
		errMsg := fmt.Sprintf("Template output is not a valid k8s label [%s]", result)
		return liberr.New(errMsg, errs)
	}

	return nil
}

func (r *Reconciler) IsValidNetworkNameTemplate(networkNameTemplate string) error {
	if networkNameTemplate == "" {
		return nil
	}

	testData := api.NetworkNameTemplateData{
		NetworkName:      "test-network",
		NetworkNamespace: "test-namespace",
		NetworkType:      "Multus",
		NetworkIndex:     0,
	}

	result, err := r.IsValidTemplate(networkNameTemplate, testData)
	if err != nil {
		return err
	}

	// Validate that template output is a valid k8s label
	errs := k8svalidation.IsDNS1123Label(result)
	if len(errs) > 0 {
		errMsg := fmt.Sprintf("Template output is not a valid k8s label [%s]", result)
		return liberr.New(errMsg, errs)
	}

	return nil
}

func (r *Reconciler) IsValidTargetName(targetName string) error {
	if targetName == "" {
		return nil
	}

	// Validate that the target name is a valid k8s name ( e.g. label with dots )
	errs := k8svalidation.IsDNS1123Subdomain(targetName)
	if len(errs) > 0 {
		return liberr.New("Target name is not a valid k8s subdomain", "errors", errs)
	}

	return nil
}
