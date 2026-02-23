package plan

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	refapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/validation"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/settings"
	"github.com/kubev2v/forklift/pkg/templateutil"
	batchv1 "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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
	WarmMigrationNotReady           = "WarmMigrationNotReady"
	MigrationTypeNotValid           = "MigrationTypeNotValid"
	NamespaceNotValid               = "NamespaceNotValid"
	TransferNetNotValid             = "TransferNetworkNotValid"
	TransferNetMissingDefaultRoute  = "TransferNetworkMissingDefaultRoute"
	NetRefNotValid                  = "NetworkMapRefNotValid"
	NetMapNotReady                  = "NetworkMapNotReady"
	NetMapPreservingIPsOnPodNetwork = "NetMapPreservingIPsOnPodNetwork"
	DsMapNotReady                   = "StorageMapNotReady"
	DsRefNotValid                   = "StorageRefNotValid"
	VMRefNotValid                   = "VMRefNotValid"
	VMNotFound                      = "VMNotFound"
	VMAlreadyExists                 = "VMAlreadyExists"
	VMNetworksNotMapped             = "VMNetworksNotMapped"
	VMStorageNotMapped              = "VMStorageNotMapped"
	VMStorageNotSupported           = "VMStorageNotSupported"
	VMMultiplePodNetworkMappings    = "VMMultiplePodNetworkMappings"
	VMDuplicateNADMappings          = "VMDuplicateNADMappings"
	VMMissingGuestIPs               = "VMMissingGuestIPs"
	VMIpNotMatchingUdnSubnet        = "VMIpNotMatchingUdnSubnet"
	VMMissingChangedBlockTracking   = "VMMissingChangedBlockTracking"
	VMHasSnapshots                  = "VMHasSnapshots"
	HostNotReady                    = "HostNotReady"
	DuplicateVM                     = "DuplicateVM"
	SharedDisks                     = "SharedDisks"
	SharedWarnDisks                 = "SharedWarnDisks"
	NameNotValid                    = "TargetNameNotValid"
	HookNotValid                    = "HookNotValid"
	HookNotReady                    = "HookNotReady"
	HookStepNotValid                = "HookStepNotValid"
	Executing                       = "Executing"
	Succeeded                       = "Succeeded"
	Failed                          = "Failed"
	Canceled                        = "Canceled"
	ConversionHasWarnings           = "ConversionHasWarnings"
	Deleted                         = "Deleted"
	Paused                          = "Paused"
	Archived                        = "Archived"
	InvalidDiskSizes                = "InvalidDiskSizes"
	MacConflicts                    = "MacConflicts"
	MissingPvcForOnlyConversion     = "MissingPvcForOnlyConversion"
	LuksAndClevisIncompatibility    = "LuksAndClevisIncompatibility"
	UnsupportedUdn                  = "UnsupportedUserDefinedNetwork"
	unsupportedVersion              = "UnsupportedVersion"
	VDDKInvalid                     = "VDDKInvalid"
	ValidatingVDDK                  = "ValidatingVDDK"
	VDDKInitImageNotReady           = "VDDKInitImageNotReady"
	VDDKInitImageUnavailable        = "VDDKInitImageUnavailable"
	UnsupportedOVFExportSource      = "UnsupportedOVFExportSource"
	VMPowerStateUnsupported         = "VMPowerStateUnsupported"
	VMMigrationTypeUnsupported      = "VMMigrationTypeUnsupported"
	GuestToolsIssue                 = "GuestToolsIssue"
	VDDKAndOffloadMixedUsage        = "VDDKAndOffloadMixedUsage"
	RestrictedPodSecurity           = "RestrictedPodSecurity"
	NetMapDestinationNADNotValid    = "NetMapDestinationNADNotValid"
)

// Categories
const (
	Required = libcnd.Required
	Advisory = libcnd.Advisory
	Critical = libcnd.Critical
	Error    = libcnd.Error
	Warn     = libcnd.Warn
)

// Network types
const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
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

	if err = r.ensureSecretForProvider(plan); err != nil {
		return err
	}

	if err = r.validateTargetNamespace(plan); err != nil {
		return err
	}

	if err = r.validateNetworkMap(plan); err != nil {
		return err
	}

	if err = r.validateStorageMap(plan); err != nil {
		return err
	}

	// If critical conditions were found (e.g. missing network/storage maps),
	// context may not be available, skip the validations that require context.
	// The blocker conditions have already been set, reconciler will not try to execute the plan.
	if plan.Status.HasBlockerCondition() {
		return nil
	}

	var ctx *plancontext.Context
	ctx, err = plancontext.New(r, plan, r.Log)
	if err != nil {
		return err
	}

	if err = r.validateUserDefinedNetwork(ctx); err != nil {
		return err
	}

	if err = r.validateWarmMigration(ctx); err != nil {
		return err
	}

	if err = r.validateMigrationType(ctx); err != nil {
		return err
	}

	if err = r.validateVM(plan); err != nil {
		return err
	}

	if err = r.validateTransferNetwork(plan); err != nil {
		return err
	}

	if err = r.validateHooks(plan); err != nil {
		return err
	}

	if err = r.validateVddkImage(plan); err != nil {
		return err
	}

	// Validate version only if migration is OCP to OCP
	if err = r.validateOpenShiftVersion(plan); err != nil {
		return err
	}

	// Validate volume name template
	if err = r.validateVolumeNameTemplate(plan); err != nil {
		return err
	}

	// Validate network name template
	if err = r.validateNetworkNameTemplate(plan); err != nil {
		return err
	}

	// Validate SSH readiness for plans using xcopy with SSH-enabled providers
	if err = r.validateSSHReadiness(plan); err != nil {
		return err
	}

	// Validate conversion temp storage configuration
	if err = r.validateConversionTempStorage(plan); err != nil {
		return err
	}

	// Validate pod security policies (non-blocking warning)
	if err = r.validatePodSecurity(plan); err != nil {
		// Log error but don't block validation
		r.Log.V(1).Info("Failed to validate pod security policies", "error", err)
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
func (r *Reconciler) validateWarmMigration(ctx *plancontext.Context) (err error) {
	if !ctx.Plan.IsWarm() {
		return
	}
	provider := ctx.Plan.Referenced.Provider.Source
	if provider == nil {
		return nil
	}
	pAdapter, err := adapter.New(provider)
	if err != nil {
		return err
	}
	validator, err := pAdapter.Validator(ctx)
	if err != nil {
		return err
	}
	if !validator.WarmMigration() {
		ctx.Plan.Status.SetCondition(libcnd.Condition{
			Type:     WarmMigrationNotReady,
			Status:   True,
			Category: api.CategoryCritical,
			Reason:   NotSupported,
			Message:  "Warm migration from the source provider is not supported.",
		})
	}
	return
}

func (r *Reconciler) validateMigrationType(ctx *plancontext.Context) (err error) {
	provider := ctx.Plan.Referenced.Provider.Source
	if provider == nil {
		return nil
	}
	pAdapter, err := adapter.New(provider)
	if err != nil {
		return err
	}
	validator, err := pAdapter.Validator(ctx)
	if err != nil {
		return err
	}
	if !validator.MigrationType() {
		ctx.Plan.Status.SetCondition(libcnd.Condition{
			Type:     MigrationTypeNotValid,
			Status:   True,
			Category: Critical,
			Reason:   NotSupported,
			Message:  fmt.Sprintf("`%s` migration from the source provider is not supported.", ctx.Plan.Spec.Type),
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

// Validate unsupported User Defined Network configurations in the destination namespace.
func (r *Reconciler) validateUserDefinedNetwork(ctx *plancontext.Context) (err error) {
	nads, err := r.getDestinationNamespaceNads(ctx)
	if err != nil {
		return err
	}

	for _, nad := range nads.Items {
		var networkConfig model.NetworkConfig
		err = json.Unmarshal([]byte(nad.Spec.Config), &networkConfig)
		if err != nil {
			r.Log.Info("Skipping NAD: failed to parse network config", "namespace", nad.Namespace, "name", nad.Name, "error", err.Error())
			continue
		}
		if networkConfig.Type != model.OvnOverlayType {
			continue
		}
		if networkConfig.Topology == model.TopologyLayer3 {
			// CNV does not support l3
			ctx.Plan.Status.SetCondition(libcnd.Condition{
				Type:     UnsupportedUdn,
				Status:   True,
				Reason:   NotSupported,
				Category: api.CategoryCritical,
				Message:  "UserDefinedNetwork Layer3 is not supported, please use Layer2",
			})
		}
	}
	return
}

func (r *Reconciler) getDestinationNamespaceNads(ctx *plancontext.Context) (*k8snet.NetworkAttachmentDefinitionList, error) {
	nadList := &k8snet.NetworkAttachmentDefinitionList{}
	listOpts := []client.ListOption{
		client.InNamespace(ctx.Plan.Spec.TargetNamespace),
		client.MatchingLabels{"k8s.ovn.org/user-defined-network": ""},
	}

	err := ctx.Destination.Client.List(context.TODO(), nadList, listOpts...)
	if err != nil {
		return nil, err
	}
	return nadList, nil
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
	// Check if we are preserving static IPs and give warning if we are mapping to Pod Network.
	// The Pod network has different subnet than the source provider so the VMs might not be accessible.
	if plan.Referenced.Provider.Source.SupportsPreserveStaticIps() && plan.Spec.PreserveStaticIPs {
		var hasMappingToPodNetwork bool
		for _, networkMap := range mp.Spec.Map {
			if networkMap.Destination.Type == Pod {
				hasMappingToPodNetwork = true
			}
		}
		// The UDNs can be valid network for which there are additional validations to check the subnet ranges per VM
		if hasMappingToPodNetwork && !plan.DestinationHasUdnNetwork(r.Client) {
			plan.Status.SetCondition(libcnd.Condition{
				Type:     NetMapPreservingIPsOnPodNetwork,
				Status:   True,
				Category: api.CategoryWarn,
				Message:  "Your migration plan preserves the static IPs of VMs and uses Pod Networking target network mapping. This combination isn't supported, because VM IPs aren't preserved in Pod Networking migrations.",
			})
		}
	}

	for _, pair := range mp.Spec.Map {
		if pair.Destination.Type != Multus {
			continue
		}

		if pair.Destination.Namespace != plan.Spec.TargetNamespace &&
			pair.Destination.Namespace != core.NamespaceDefault {
			plan.Status.SetCondition(libcnd.Condition{
				Type:     NetMapDestinationNADNotValid,
				Status:   True,
				Category: api.CategoryCritical,
				Reason:   NotValid,
				Message: fmt.Sprintf(
					"Destination NAD %s/%s must be in either the target namespace (%s) or the default namespace. "+
						"Pods cannot reference network attachment definitions from other namespaces.",
					pair.Destination.Namespace,
					pair.Destination.Name,
					plan.Spec.TargetNamespace),
			})
			return
		}
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
	// Ignore mapping for only conversion mode
	if plan.Spec.Type == api.MigrationOnlyConversion {
		return
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
	duplicateNADMappings := libcnd.Condition{
		Type:     VMDuplicateNADMappings,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "Multiple VM NICs use the same Multus NAD name (OVN-Kubernetes uses NAD names as map keys).",
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
	vmIpDoesNotMatchUdnSubnet := libcnd.Condition{
		Type:     VMIpNotMatchingUdnSubnet,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryWarn,
		Message:  "VM IP does not match with the primary UDN subnet",
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
	vmHasSnapshotsForWarm := libcnd.Condition{
		Type:     VMHasSnapshots,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VM has pre-existing snapshots which are incompatible with warm migration.",
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
	unsupportedOVFExportSource := libcnd.Condition{
		Type:     UnsupportedOVFExportSource,
		Status:   True,
		Category: api.CategoryWarn,
		Message:  "VM appears to have been exported from an unsupported OVF source, and may have issues during import.",
		Items:    []string{},
	}
	powerStateUnsupported := libcnd.Condition{
		Type:     VMPowerStateUnsupported,
		Status:   True,
		Reason:   NotSupported,
		Category: api.CategoryCritical,
		Message:  "VM power state is incompatible with the selected migration type.",
		Items:    []string{},
	}
	vmMigrationTypeUnsupported := libcnd.Condition{
		Type:     VMMigrationTypeUnsupported,
		Status:   True,
		Reason:   NotSupported,
		Category: api.CategoryCritical,
		Message:  "VM is incompatible with the selected migration type.",
		Items:    []string{},
	}
	guestToolsIssue := libcnd.Condition{
		Type:     GuestToolsIssue,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VMware Tools issues detected. This may impact migration performance, guest OS detection, and network configuration. Ensure VMware Tools are properly installed and running before migration. If this is an encrypted VM, please turn the VM off manually before migration.",
		Items:    []string{},
	}
	invalidDiskSizes := libcnd.Condition{
		Type:     InvalidDiskSizes,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "VM has disks with invalid sizes.",
		Items:    []string{},
	}
	macConflicts := libcnd.Condition{
		Type:     MacConflicts,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "",
	}
	missingPvcForOnlyConversion := libcnd.Condition{
		Type:     MissingPvcForOnlyConversion,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryCritical,
		Message:  "Missing required PVCs for conversion-only mode. Ensure vendor-provided PVCs exist in the target namespace and are labeled with vmID and vmUUID.",
		Items:    []string{},
	}
	luksAndClevisIncompatibility := libcnd.Condition{
		Type:     LuksAndClevisIncompatibility,
		Status:   True,
		Reason:   NotValid,
		Category: api.CategoryWarn,
		Message:  "LUKS keys and Clevis cannot be configured together; Clevis will be used.",
		Items:    []string{},
	}
	vddkAndOffloadMixedUsage := libcnd.Condition{
		Type:     VDDKAndOffloadMixedUsage,
		Status:   True,
		Reason:   NotSupported,
		Category: api.CategoryCritical,
		Message:  "Copy offload is enabled. MTV does not support mixed copy methods. Each migration plan can use one migration strategy, either VDDK or copy offload. Check your storage map and VMs to ensure they are using the same migration strategy.",
		Items:    []string{},
	}

	var sharedDisksConditions []libcnd.Condition
	setOf := map[string]bool{}
	setOfTargetName := map[string]bool{}

	// Check if plan uses storage offload (vSphere only)
	source := plan.Referenced.Provider.Source
	checkMixedUsage := source != nil && source.Type() == api.VSphere && settings.Settings.Features.CopyOffload
	planUsesOffload := checkMixedUsage && plan.IsUsingOffloadPlugin()

	//
	// Referenced VMs.
	for i := range plan.Spec.VMs {
		vm := &plan.Spec.VMs[i]
		ref := &vm.Ref

		// Skip VMs that have already succeeded - no validation needed
		if status, found := plan.Status.Migration.FindVM(*ref); found {
			if status.HasCondition(api.ConditionSucceeded) {
				continue
			}
		}

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
					unsupportedOVFExportSource.Items = append(unsupportedOVFExportSource.Items, ref.String())
				}
			}
		}
		if plan.Spec.Type == api.MigrationOnlyConversion {
			if vm, ok := v.(*vsphere.VM); ok {
				pvcs, err := r.getVmPVCs(plan, vm)
				if err != nil {
					return err
				}
				if len(pvcs) != len(vm.Disks) {
					missingPvcForOnlyConversion.Items = append(missingPvcForOnlyConversion.Items, ref.String())
				}
			}
		}

		// Check for mixed VDDK/Offload usage (vSphere only)
		// If plan uses offload, add VMs with VDDK disks to the condition
		if planUsesOffload {
			if vsphereVM, ok := v.(*vsphere.VM); ok {
				storageMap := plan.Referenced.Map.Storage
				if storageMap != nil {
					curVMHasVddk, err := r.vmUsesVddk(storageMap, vsphereVM, vm.Name)
					if err != nil {
						return err
					}
					if curVMHasVddk {
						vddkAndOffloadMixedUsage.Items = append(vddkAndOffloadMixedUsage.Items, ref.String())
					}
				}
			}
		}
		pAdapter, err := adapter.New(provider)
		if err != nil {
			return err
		}
		var ctx *plancontext.Context
		ctx, err = plancontext.New(r, plan, r.Log)
		if err != nil {
			return err
		}
		validator, err := pAdapter.Validator(ctx)
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
			nicRefs, nErr := validator.NICNetworkRefs(*ref)
			if nErr != nil {
				return nErr
			}
			foundNadDup, foundPodDup := planbase.ValidateNetworkDuplicates(nicRefs, plan.Referenced.Map.Network)
			if foundPodDup {
				multiplePodNetworkMappings.Items = append(multiplePodNetworkMappings.Items, ref.String())
			}
			if foundNadDup {
				duplicateNADMappings.Items = append(duplicateNADMappings.Items, ref.String())
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
		ok, err = validator.PowerState(*ref)
		if err != nil {
			return err
		}
		if !ok {
			powerStateUnsupported.Items = append(powerStateUnsupported.Items, ref.String())
		}
		ok, err = validator.VMMigrationType(*ref)
		if err != nil {
			return err
		}
		if !ok {
			vmMigrationTypeUnsupported.Items = append(vmMigrationTypeUnsupported.Items, ref.String())
		}
		if vm.LUKS.Name != "" && vm.NbdeClevis {
			luksAndClevisIncompatibility.Items = append(luksAndClevisIncompatibility.Items, ref.String())
		}
		// Guest tools validation (provider-specific)
		ok, err = validator.GuestToolsInstalled(*ref)
		if err != nil {
			return err
		}
		if !ok {
			guestToolsIssue.Items = append(guestToolsIssue.Items, ref.String())
		}
		invalidSizes, err := validator.InvalidDiskSizes(*ref)
		if err != nil {
			return err
		}
		if len(invalidSizes) > 0 {
			invalidDiskSizes.Items = append(invalidDiskSizes.Items, ref.String())
		}

		conflicts, err := validator.MacConflicts(*ref)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			macConflicts.Items = append(macConflicts.Items, ref.String())
			// Group conflicts by destination VM for this specific source VM
			vmConflictsByVM := make(map[string][]string)
			for _, conflict := range conflicts {
				vmConflictsByVM[conflict.DestinationVM] = append(vmConflictsByVM[conflict.DestinationVM], conflict.MAC)
			}

			// Build detailed message with grouped conflicts
			var conflictDetails []string
			for destinationVM, macs := range vmConflictsByVM {
				conflictDetails = append(conflictDetails, fmt.Sprintf("MACs %s conflict with destination VM %s", strings.Join(macs, ", "), destinationVM))
			}
			if macConflicts.Message != "" {
				macConflicts.Message += "; "
			}
			macConflicts.Message += fmt.Sprintf("VM %s has MAC address conflicts: %s", ref.String(), strings.Join(conflictDetails, "; "))
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
		if settings.Settings.StaticUdnIpAddresses && plan.Spec.PreserveStaticIPs && plan.DestinationHasUdnNetwork(r.Client) {
			ok, err = validator.UdnStaticIPs(*ref, ctx.Destination.Client)
			if err != nil {
				return err
			}
			if !ok {
				vmIpDoesNotMatchUdnSubnet.Items = append(vmIpDoesNotMatchUdnSubnet.Items, ref.String())
			}
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
		vmName := ref.Name
		if vm.TargetName != "" {
			// if target name is provided, use it to look for existing VMs
			vmName = vm.TargetName
		}
		vmRef := &refapi.Ref{
			Name:      vmName,
			Namespace: plan.Spec.TargetNamespace,
		}
		_, pErr = inventory.VM(vmRef)
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
		if plan.IsWarm() {
			enabled, err := validator.ChangeTrackingEnabled(*ref)
			if err != nil {
				return err
			}
			if !enabled {
				missingCbtForWarm.Items = append(missingCbtForWarm.Items, ref.String())
			}

			// Check for pre-existing snapshots
			ok, msg, _, err := validator.HasSnapshot(*ref)
			if err != nil {
				return err
			}
			if !ok {
				vmHasSnapshotsForWarm.Items = append(vmHasSnapshotsForWarm.Items, ref.String())
				if msg != "" {
					vmHasSnapshotsForWarm.Message = msg
				}
			}
		}
		// is valid vm pvc name template
		if plan.Spec.PVCNameTemplate != "" || vm.PVCNameTemplate != "" {
			// if vm level pvc name template is set, use it, otherwise use plan level pvc name template
			pvcNameTemplate := plan.Spec.PVCNameTemplate
			if vm.PVCNameTemplate != "" {
				pvcNameTemplate = vm.PVCNameTemplate
			}

			// validate pvc name template for the vm
			if _, err := validator.PVCNameTemplate(vm.Ref, pvcNameTemplate); err != nil {
				r.Log.Info("PVC name template is invalid", "error", err.Error(), "template", pvcNameTemplate, "plan", plan.Name, "namespace", plan.Namespace)

				conditionItem := fmt.Sprintf("%s template:%s error:%s", ref.String(), pvcNameTemplate, err.Error())
				pvcNameInvalid.Items = append(pvcNameInvalid.Items, conditionItem)
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
	if len(duplicateNADMappings.Items) > 0 {
		plan.Status.SetCondition(duplicateNADMappings)
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
	if len(vmIpDoesNotMatchUdnSubnet.Items) > 0 {
		plan.Status.SetCondition(vmIpDoesNotMatchUdnSubnet)
	}
	if len(vmHasSnapshotsForWarm.Items) > 0 {
		plan.Status.SetCondition(vmHasSnapshotsForWarm)
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
	if len(unsupportedOVFExportSource.Items) > 0 {
		plan.Status.SetCondition(unsupportedOVFExportSource)
	}
	if len(powerStateUnsupported.Items) > 0 {
		plan.Status.SetCondition(powerStateUnsupported)
	}
	if len(vmMigrationTypeUnsupported.Items) > 0 {
		plan.Status.SetCondition(vmMigrationTypeUnsupported)
	}
	if len(guestToolsIssue.Items) > 0 {
		plan.Status.SetCondition(guestToolsIssue)
	}
	if len(invalidDiskSizes.Items) > 0 {
		plan.Status.SetCondition(invalidDiskSizes)
	}
	if len(macConflicts.Items) > 0 {
		plan.Status.SetCondition(macConflicts)
	}
	if len(missingPvcForOnlyConversion.Items) > 0 {
		plan.Status.SetCondition(missingPvcForOnlyConversion)
	}
	if len(luksAndClevisIncompatibility.Items) > 0 {
		plan.Status.SetCondition(luksAndClevisIncompatibility)
	}

	// Set the condition if any VMs with VDDK disks were found when plan uses offload
	if len(vddkAndOffloadMixedUsage.Items) > 0 {
		plan.Status.SetCondition(vddkAndOffloadMixedUsage)
	}

	return nil
}

// Return PersistentVolumeClaims associated with a VM.
func (r *Reconciler) getVmPVCs(plan *api.Plan, vm *vsphere.VM) (pvcs []*core.PersistentVolumeClaim, err error) {
	// Add VM uuid
	labelSelector := map[string]string{
		kVM:     vm.ID,
		kVmUuid: vm.UUID,
	}
	var ctx *plancontext.Context
	ctx, err = plancontext.New(r, plan, r.Log)
	if err != nil {
		return
	}
	pvcsList := &core.PersistentVolumeClaimList{}
	err = ctx.Destination.Client.List(
		context.TODO(),
		pvcsList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(labelSelector),
			Namespace:     plan.Spec.TargetNamespace,
		},
	)

	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	pvcs = make([]*core.PersistentVolumeClaim, len(pvcsList.Items))
	for i, pvc := range pvcsList.Items {
		// loopvar
		pvc := pvc
		pvcs[i] = &pvc
	}

	return
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
	missingDefaultRoute := libcnd.Condition{
		Type:     TransferNetMissingDefaultRoute,
		Status:   True,
		Category: api.CategoryWarn,
		Reason:   NotValid,
		Message:  "Transfer network missing default route annotation.",
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

	if plan.Spec.TransferNetwork.Namespace != plan.Spec.TargetNamespace &&
		plan.Spec.TransferNetwork.Namespace != core.NamespaceDefault {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     TransferNetNotValid,
			Status:   True,
			Category: api.CategoryCritical,
			Reason:   NotValid,
			Message: fmt.Sprintf(
				"Transfer network %s/%s is in a different namespace than the target namespace %s. "+
					"Pods cannot reference network attachment definitions across namespaces.",
				plan.Spec.TransferNetwork.Namespace,
				plan.Spec.TransferNetwork.Name,
				plan.Spec.TargetNamespace),
		})
		return
	}
	route, found := netAttachDef.Annotations[AnnForkliftNetworkRoute]
	if !found {
		plan.Status.SetCondition(missingDefaultRoute)
		return
	}
	// Handle case where user explicitly requested not to have a default route
	if route != AnnForkliftRouteValueNone {
		ip := net.ParseIP(route)
		if ip == nil {
			plan.Status.SetCondition(notValid)
		}
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
	for _, cnd := range []libcnd.Condition{notSet, notFound, notReady, stepNotValid} {
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
	if plan.IsWarm() && vddkImage == "" {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     VDDKInitImageUnavailable,
			Status:   True,
			Reason:   NotSet,
			Category: api.CategoryCritical,
			Message:  "VDDK image not set on the provider, this is required for the warm migration",
		})
	}
	if plan.Spec.SkipGuestConversion && vddkImage == "" && !plan.IsUsingOffloadPlugin() {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     VDDKInitImageUnavailable,
			Status:   True,
			Reason:   NotSet,
			Category: api.CategoryCritical,
			Message:  "VDDK image not set on the provider, this is required for the raw copy mode migration",
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
			// Pod exists but init container statuses haven't been populated yet.
			// This is normal when the pod was just created. Log it and
			// let the next reconcile check again.
			r.Log.Info("Validation pod init container statuses not yet available, will requeue",
				"pod", pod.Name, "phase", pod.Status.Phase)
			plan.Status.SetCondition(vddkValidationInProgress)
			return
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

	secret := core.Secret{}
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
		return "", liberr.Wrap(err, "template", templateStr)
	}

	// Empty output is not valid
	if result == "" {
		return "", liberr.New("Template output is empty", "template", templateStr)
	}

	return result, nil
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
		return liberr.New(errMsg, "template", volumeNameTemplate, "errors", errs)
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
		return liberr.New(errMsg, "template", networkNameTemplate, "errors", errs)
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

func (r *Reconciler) validateConversionTempStorage(plan *api.Plan) error {
	storageClass := plan.Spec.ConversionTempStorageClass
	storageSize := plan.Spec.ConversionTempStorageSize

	// If neither is set, that's fine
	if storageClass == "" && storageSize == "" {
		return nil
	}

	// If only one is set, that's an error
	if storageClass == "" || storageSize == "" {
		conversionTempStorageIncomplete := libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryCritical,
			Message:  "Both ConversionTempStorageClass and ConversionTempStorageSize must be specified together.",
			Items:    []string{},
		}
		plan.Status.SetCondition(conversionTempStorageIncomplete)
		return nil
	}

	// Validate that storageSize is a valid Kubernetes resource quantity
	requestedQty, err := resource.ParseQuantity(storageSize)
	if err != nil {
		conversionTempStorageSizeInvalid := libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryCritical,
			Message:  fmt.Sprintf("ConversionTempStorageSize '%s' is not a valid Kubernetes resource quantity: %v", storageSize, err),
			Items:    []string{},
		}
		plan.Status.SetCondition(conversionTempStorageSizeInvalid)
		r.Log.Info("Conversion temp storage size is invalid", "error", err.Error(), "size", storageSize, "plan", plan.Name, "namespace", plan.Namespace)
		return nil
	}

	// Check CSIStorageCapacity when available: block migration if storage class
	// reports insufficient capacity for the requested conversion temp volume size.
	if err := r.validateConversionTempStorageCapacity(plan, storageClass, requestedQty); err != nil {
		return err
	}

	return nil
}

// validateConversionTempStorageCapacity checks CSIStorageCapacity for the given
// storage class. If any entry reports capacity sufficient for the requested size
// (MaximumVolumeSize or Capacity >= requested), the check passes. If entries exist
// but none have sufficient capacity, a blocking condition is set. If no entries
// exist for the storage class, an advisory warning is set.
func (r *Reconciler) validateConversionTempStorageCapacity(plan *api.Plan, storageClassName string, requested resource.Quantity) error {
	ctx := context.Background()
	list := &storagev1.CSIStorageCapacityList{}
	if err := r.Client.List(ctx, list, client.InNamespace(core.NamespaceAll)); err != nil {
		r.Log.Info("Could not list CSIStorageCapacity (capacity check skipped)", "error", err.Error(), "storageClass", storageClassName)
		plan.Status.SetCondition(libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryAdvisory,
			Message:  fmt.Sprintf("Storage capacity for ConversionTempStorageClass %q could not be verified. Ensure sufficient space is available.", storageClassName),
			Items:    []string{},
		})
		return nil
	}

	var matching []storagev1.CSIStorageCapacity
	for i := range list.Items {
		if list.Items[i].StorageClassName == storageClassName {
			matching = append(matching, list.Items[i])
		}
	}

	if len(matching) == 0 {
		plan.Status.SetCondition(libcnd.Condition{
			Type:     NotValid,
			Status:   True,
			Category: api.CategoryAdvisory,
			Message:  fmt.Sprintf("No capacity information found for ConversionTempStorageClass %q. Ensure sufficient space is available.", storageClassName),
			Items:    []string{},
		})
		return nil
	}

	for _, cap := range matching {
		// Prefer MaximumVolumeSize (largest single volume); fall back to Capacity (available space).
		if cap.MaximumVolumeSize != nil && cap.MaximumVolumeSize.Cmp(requested) >= 0 {
			return nil
		}
		if cap.MaximumVolumeSize == nil && cap.Capacity != nil && cap.Capacity.Cmp(requested) >= 0 {
			return nil
		}
	}

	// Have capacity info but no entry can satisfy the requested size
	plan.Status.SetCondition(libcnd.Condition{
		Type:     NotValid,
		Status:   True,
		Category: api.CategoryCritical,
		Message:  fmt.Sprintf("Insufficient space in storage class %q for conversion temp storage (requested %s). Migration may fail.", storageClassName, requested.String()),
		Items:    []string{},
	})
	r.Log.Info("Conversion temp storage capacity insufficient", "storageClass", storageClassName, "requested", requested.String(), "plan", plan.Name, "namespace", plan.Namespace)
	return nil
}

// validatePodSecurity checks if the controller namespace has restrictive pod security policies
// that may cause migration failures. This is a non-blocking warning.
func (r *Reconciler) validatePodSecurity(plan *api.Plan) error {
	// Get controller namespace from environment variable
	// POD_NAMESPACE should be set via fieldRef in the deployment template
	controllerNamespace := os.Getenv("POD_NAMESPACE")
	if controllerNamespace == "" {
		// Fallback to settings if available (loaded from POD_NAMESPACE env var)
		controllerNamespace = settings.Settings.Inventory.Namespace
	}
	if controllerNamespace == "" {
		// Can't check if we don't know the controller namespace
		// POD_NAMESPACE should be set via fieldRef.fieldPath: metadata.namespace in the deployment
		r.Log.Info("Skipping pod security check: POD_NAMESPACE not set in controller pod",
			"plan", plan.Name,
			"planNamespace", plan.GetNamespace())
		return nil
	}

	// Log which namespace we're checking (for debugging)
	r.Log.Info("Checking pod security policy for controller namespace",
		"controllerNamespace", controllerNamespace,
		"plan", plan.Name,
		"planNamespace", plan.GetNamespace())

	// Read the namespace object
	ns := &core.Namespace{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: controllerNamespace}, ns)
	if err != nil {
		if k8serr.IsNotFound(err) {
			// Namespace not found, skip check
			r.Log.Info("Skipping pod security check: namespace not found",
				"namespace", controllerNamespace)
			return nil
		}
		return liberr.Wrap(err, "failed to get controller namespace", "namespace", controllerNamespace)
	}

	// Check pod-security.kubernetes.io/enforce label
	enforceLabel := ns.Labels["pod-security.kubernetes.io/enforce"]
	isRestricted := false

	r.Log.Info("Checking pod security policy",
		"namespace", controllerNamespace,
		"enforceLabel", enforceLabel,
		"plan", plan.Name)

	if enforceLabel == "restricted" {
		isRestricted = true
		r.Log.Info("Detected restricted pod security policy from namespace label",
			"namespace", controllerNamespace,
			"label", "pod-security.kubernetes.io/enforce",
			"value", enforceLabel,
			"plan", plan.Name)
	}

	// If restricted policies detected, add a warning condition (non-blocking)
	if isRestricted {
		restrictedPodSecurity := libcnd.Condition{
			Type:     RestrictedPodSecurity,
			Status:   True,
			Category: Warn, // Non-blocking warning
			Message:  fmt.Sprintf("Namespace '%s' where MTV is installed may be restricted, causing migration to fail. Disable SCC label synchronization and apply the privileged label.", controllerNamespace),
			Items:    []string{},
		}
		plan.Status.SetCondition(restrictedPodSecurity)
		r.Log.Info("Restricted pod security policy detected - warning condition added",
			"namespace", controllerNamespace,
			"plan", plan.Name)
	} else {
		r.Log.Info("No restricted pod security policy detected",
			"namespace", controllerNamespace,
			"enforceLabel", enforceLabel,
			"plan", plan.Name)
	}

	return nil
}

// vmUsesVddk checks if the VM requires VDDK for migration (i.e., if any disk doesn't use storage offload)
func (r *Reconciler) vmUsesVddk(storageMap *api.StorageMap, vsphereVM *vsphere.VM, vmName string) (bool, error) {
	for _, disk := range vsphereVM.Disks {
		mapping, found := storageMap.FindStorage(disk.Datastore.ID)
		if !found {
			continue // Another validation will handle this
		}
		if mapping.OffloadPlugin == nil || mapping.OffloadPlugin.VSphereXcopyPluginConfig == nil {
			return true, nil
		}
	}

	return false, nil
}
