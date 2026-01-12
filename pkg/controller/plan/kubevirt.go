package plan

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	convbuilder "github.com/kubev2v/forklift/pkg/controller/conversion"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	hvutil "github.com/kubev2v/forklift/pkg/controller/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	inspectionparser "github.com/kubev2v/forklift/pkg/controller/plan/adapter/vsphere"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	migbase "github.com/kubev2v/forklift/pkg/controller/plan/migrator/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	ctrlutil "github.com/kubev2v/forklift/pkg/controller/util"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
	"github.com/kubev2v/forklift/pkg/settings"
	template "github.com/openshift/api/template/v1"
	"github.com/openshift/library-go/pkg/template/generator"
	"github.com/openshift/library-go/pkg/template/templateprocessing"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CSI Drivers
const (
	// SMBCSIDriver is the CSI driver name for SMB shares (used by HyperV provider)
	SMBCSIDriver = "smb.csi.k8s.io"
)

// Annotations
const (
	// Legacy transfer network annotation (value=network-attachment-definition name)
	// FIXME: this should be phased out and replaced with the
	// k8s.v1.cni.cncf.io/networks annotation.
	AnnLegacyTransferNetwork = "v1.multus-cni.io/default-network"
	// Transfer network annotation (value=network-attachment-definition name)
	AnnTransferNetwork = "k8s.v1.cni.cncf.io/networks"
	// Annotation to specify the default route for the transfer network.
	// To be set on the transfer network NAD by the end user.
	AnnForkliftNetworkRoute = "forklift.konveyor.io/route"
	// Special value for AnnForkliftNetworkRoute to explicitly request no gateway.
	// Use this to enable modern k8s.v1.cni.cncf.io/networks annotation without default-route.
	AnnForkliftRouteValueNone = "none"
	// Contains validations for a Kubevirt VM. Needs to be removed when
	// creating a VM from a template.
	AnnKubevirtValidations = "vm.kubevirt.io/validations"
	// PVC annotation containing the name of the importer pod.
	AnnImporterPodName = "cdi.kubevirt.io/storage.import.importPodName"
	// Openshift display name annotation (value=vmName)
	AnnDisplayName = "openshift.io/display-name"
	//  Original VM name on source (value=vmOriginalID)
	AnnOriginalID = "original-ID"
	// DV deletion on completion
	AnnDeleteAfterCompletion = "cdi.kubevirt.io/storage.deleteAfterCompletion"
	// VddkVolumeName is the volume name used for the VDDK library scratch space.
	VddkVolumeName = "vddk-vol-mount"
	// DynamicScriptsVolumeName is the volume name used to mount first-boot scripts.
	DynamicScriptsVolumeName = "scripts-volume-mount"
	// DynamicScriptsMountPath is the mount path for first-boot scripts.
	DynamicScriptsMountPath = "/mnt/dynamic_scripts"
	// Annotation to specify current number of retries for getting parent backing
	ParentBackingRetriesAnnotation = "parentBackingRetries"
	// AnnPopulatorServiceAccount is set on populator PVCs to propagate the
	// migration ServiceAccount to the populator pod.
	AnnPopulatorServiceAccount = "forklift.konveyor.io/serviceAccount"
	// AnnCDIPodServiceAccount is set on DataVolume annotations so CDI uses
	// the specified ServiceAccount for its data transfer pods.
	AnnCDIPodServiceAccount = "cdi.kubevirt.io/storage.pod.serviceAccountName"
)

// Labels
const (
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// plan name label (value=Plan.Name)
	kPlanName = "plan-name"
	// plan namespace label (value=Plan.Namespace)
	kPlanNamespace = "plan-namespace"
	// VM label (value=vmID)
	kVM = "vmID"
	// VM UUID label
	kVmUuid = "vmUUID"
	// App label
	kApp = "forklift.app"
	// LUKS
	kLUKS = "isLUKS"
	// Connection
	kConnection = "isConnection"
	// Use
	kUse = "use"
	// DV secret
	kDV = "isDV"
	// Populator secret
	kPopulator = "isPopulator"
	// V2V conversion secret
	kV2V = "isV2V"
	// Resource label
	kResource = "resource"
)

// Resource labels
const (
	ResourceVMConfig      = "vm-config"
	ResourceVDDKConfig    = "vddk-config"
	ResourceWaitForReboot = "wait-for-reboot"
)

// User
const (
	// Qemu user
	qemuUser = int64(107)
)

// Labels
const (
	OvaPVCLabel    = "nfs-pvc"
	OvaPVLabel     = "nfs-pv"
	HyperVPVCLabel = "smb-pvc"
	HyperVPVLabel  = "smb-pv"
)

// Vddk v2v conf
const (
	ExtraV2vConf         = "extra-v2v-conf"
	VddkConf             = "vddk-conf"
	CustomizationScripts = "customization-scripts"

	VddkAioBufSizeDefault  = "16"
	VddkAioBufCountDefault = "4"
)

// VirtV2V pod types (aliases for the canonical constants in the conversion/context package).
const (
	VirtV2vConversionPod = convctx.VirtV2vConversionPod
	VirtV2vInspectionPod = convctx.VirtV2vInspectionPod
)

// PV size
const (
	PVSize = "1Gi"
)

// Map of VirtualMachines keyed by vmID.
type VirtualMachineMap map[string]VirtualMachine

// Represents kubevirt.
type KubeVirt struct {
	*plancontext.Context
	// Builder
	Builder adapter.Builder
	// Ensurer
	Ensurer adapter.Ensurer
}

// conversionResources holds all resolved resources needed by both the
// Conversion CR and the direct pod creation
type conversionResources struct {
	podConfig      convctx.PodConfig
	pvcs           []*core.PersistentVolumeClaim
	volumes        []core.Volume
	mounts         []core.VolumeMount
	devices        []core.VolumeDevice
	extraVolumes   []core.Volume
	extraMounts    []core.VolumeMount
	secret         *core.Secret
	inPlace        bool
	vddkImage      string
	localMigration bool
	udn            bool
	// ready is false when inspection environment data is not yet available (e.g. waiting for a snapshot)
	ready bool
}

// resolveConversionResources resolves VM volumes, PVCs, pod volume mounts,
// the v2v secret, provider settings and a PodConfig. For inspection pods
// it also builds the inspection specific environment. When that data is
// not yet available res.ready will be false.
func (r *KubeVirt) resolveConversionResources(vm *plan.VMStatus, podType convctx.V2vPodType, step *plan.Step) (res conversionResources, err error) {
	res.podConfig = convctx.PodConfigFromPlan(r.Plan)
	res.podConfig.Affinity = r.getConvertorAffinity()

	res.podConfig.RequestKVM = shouldRequestKVM(r.Plan.Provider.Source)

	var vmVolumes []cnv.Volume
	if podType == convctx.VirtV2vConversionPod {
		vmVolumes, err = r.getVMVolumes(vm)
		if err != nil {
			return
		}

		res.pvcs, err = r.getPVCs(vm.Ref)
		if err != nil {
			return
		}

		useV2v, v2vErr := r.Context.Plan.ShouldUseV2vForTransfer(vm.Ref, r.Destination.Client)
		if v2vErr != nil {
			err = v2vErr
			return
		}
		res.inPlace = !useV2v || r.IsCopyOffload(res.pvcs)
	}

	var vddkConfigMap *core.ConfigMap
	if r.Source.Provider.UseVddkAioOptimization() {
		vddkConfigMap, err = r.ensureVddkConfigMap()
		if err != nil {
			return
		}
	}

	res.volumes, res.mounts, res.devices, res.extraVolumes, res.extraMounts, err = r.podVolumeMounts(vmVolumes, vddkConfigMap, res.pvcs, vm)
	if err != nil {
		return
	}

	res.secret, err = r.ensureV2vSecret(vm.Ref)
	if err != nil {
		return
	}

	res.vddkImage = settings.GetVDDKImage(r.Source.Provider.Spec.Settings)
	res.localMigration = r.Destination.Provider.IsHost()
	res.udn = r.Plan.DestinationHasUdnNetwork(r.Destination)

	res.podConfig.VDDKImage = res.vddkImage
	res.podConfig.LocalMigration = res.localMigration
	res.podConfig.GenerateName = r.getGeneratedName(vm)

	if res.podConfig.TransferNetwork != nil {
		anns := map[string]string{}
		if err = r.setTransferNetwork(anns); err != nil {
			return
		}
		res.podConfig.TransferNetworkAnnotations = anns
	}

	switch podType {
	case convctx.VirtV2vConversionPod:
		res.podConfig.PodLabels = r.conversionLabels(vm.Ref, false)
	case convctx.VirtV2vInspectionPod:
		res.podConfig.PodLabels = r.inspectionLabels(vm.Ref)
	}

	providerCfg, err := r.Builder.ConversionPodConfig(vm.Ref)
	if err != nil {
		return
	}
	maps.Copy(res.podConfig.PodLabels, providerCfg.Labels)
	maps.Copy(res.podConfig.PodLabels, res.podConfig.ConvertorLabels)
	res.podConfig.PodAnnotations = providerCfg.Annotations
	if res.udn {
		udnAnnotation, udnErr := buildUDNAnnotation()
		if udnErr != nil {
			err = udnErr
			return
		}
		if res.podConfig.PodAnnotations == nil {
			res.podConfig.PodAnnotations = make(map[string]string)
		}
		res.podConfig.PodAnnotations[planbase.AnnOpenDefaultPorts] = udnAnnotation
	}
	if providerCfg.NodeSelector != nil || res.podConfig.ConvertorNodeSelector != nil {
		res.podConfig.PodNodeSelector = make(map[string]string)
		maps.Copy(res.podConfig.PodNodeSelector, providerCfg.NodeSelector)
		maps.Copy(res.podConfig.PodNodeSelector, res.podConfig.ConvertorNodeSelector)
	}

	providerEnv, err := r.Builder.PodEnvironment(vm.Ref, r.Source.Secret)
	if err != nil {
		return
	}
	res.podConfig.Environment = providerEnv
	if vm.RootDisk != "" {
		res.podConfig.Environment = append(res.podConfig.Environment, core.EnvVar{Name: "V2V_RootDisk", Value: vm.RootDisk})
	}
	if vm.NewName != "" {
		res.podConfig.Environment = append(res.podConfig.Environment, core.EnvVar{Name: "V2V_NewName", Value: r.getNewVMName(vm)})
	}

	res.ready = true
	if podType == convctx.VirtV2vInspectionPod && step != nil {
		var inspEnv []core.EnvVar
		var success bool
		inspEnv, success, err = r.buildInspectionPodEnvironment(res.podConfig.Environment, vm, step)
		if err != nil {
			return
		}
		if !success {
			res.ready = false
			return
		}
		res.podConfig.Environment = inspEnv
	}

	return
}

// checkProviderReady returns whether the provider storage is ready after pod/CR creation.
func (r *KubeVirt) checkProviderReady(vmID string) (ready bool, err error) {
	switch r.Source.Provider.Type() {
	case api.Ova, api.HyperV:
		return r.EnsureProviderVirtV2VPVCStatus(vmID)
	case api.EC2, api.VSphere:
		return true, nil
	default:
		return true, nil
	}
}

// ResolveConversionType determines whether the migration for the given VM
// should use InPlace or Remote conversion based on the plan transfer mode
// and PVC copy-offload annotations.
func (r *KubeVirt) ResolveConversionType(vm *plan.VMStatus) (api.ConversionType, error) {
	useV2v, err := r.Context.Plan.ShouldUseV2vForTransfer(vm.Ref, r.Destination.Client)
	if err != nil {
		return "", err
	}
	pvcs, err := r.getPVCs(vm.Ref)
	if err != nil {
		return "", err
	}
	if !useV2v || r.IsCopyOffload(pvcs) {
		return api.InPlace, nil
	}
	return api.Remote, nil
}

// EnsureConversion resolves all plan data, checks whether a matching
// Conversion CR already exists and creates one if needed.
func (r *KubeVirt) EnsureConversion(vm *plan.VMStatus, conversionType api.ConversionType, planName, planNamespace, planID string, migration *api.Migration, step *plan.Step) (ready bool, err error) {
	var podType convctx.V2vPodType
	var labels map[string]string

	switch conversionType {
	case api.Remote, api.InPlace:
		podType = convctx.VirtV2vConversionPod
	case api.Inspection, api.DeepInspection:
		podType = convctx.VirtV2vInspectionPod
	default:
		return
	}

	resources, err := r.resolveConversionResources(vm, podType, step)
	if err != nil {
		return
	}
	if !resources.ready {
		return false, nil
	}

	labels = map[string]string{
		convctx.LabelPlan:           planID,
		convctx.LabelVM:             vm.ID,
		convctx.LabelPlanName:       planName,
		convctx.LabelPlanNamespace:  planNamespace,
		convctx.LabelConversionType: string(conversionType),
	}
	if migration != nil {
		labels[convctx.LabelMigration] = string(migration.UID)
	}
	// Pod labels are stored on the CR itself so they can be copied
	// directly to the pod during creation without routing through PodSettings.
	maps.Copy(labels, resources.podConfig.PodLabels)

	list := &api.ConversionList{}
	err = r.Client.List(context.TODO(), list,
		client.InNamespace(r.Plan.Namespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		r.Log.Info(
			"Conversion CR already exists.",
			"conversion", path.Join(list.Items[0].Namespace, list.Items[0].Name),
			"vm", vm.String())
		return r.checkProviderReady(vm.ID)
	}

	extraVolNames := make(map[string]bool, len(resources.extraVolumes))
	for _, v := range resources.extraVolumes {
		extraVolNames[v.Name] = true
	}
	var diskVolumes []core.Volume
	for _, v := range resources.volumes {
		if !extraVolNames[v.Name] {
			diskVolumes = append(diskVolumes, v)
		}
	}
	diskRefs := convbuilder.DiskRefsFromVolumes(diskVolumes, resources.mounts, resources.devices, resources.pvcs)

	envSettings := make(map[string]string, len(resources.podConfig.Environment))
	for _, ev := range resources.podConfig.Environment {
		envSettings[ev.Name] = ev.Value
	}

	conversion := &api.Conversion{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Namespace,
			GenerateName: planName + "-" + vm.ID + "-",
			Labels:       labels,
			Annotations:  resources.podConfig.PodAnnotations,
		},
		Spec: api.ConversionSpec{
			Type:            conversionType,
			TargetNamespace: r.Plan.Spec.TargetNamespace,
			Destination: core.ObjectReference{
				Namespace: r.Destination.Provider.Namespace,
				Name:      r.Destination.Provider.Name,
			},
			VM:    vm.Ref,
			Disks: diskRefs,
			Connection: api.Connection{
				Secret: core.ObjectReference{
					Namespace: resources.podConfig.TargetNamespace,
					Name:      resources.secret.Name,
				},
			},
			Image:            convctx.GetVirtV2vImage(&resources.podConfig),
			XfsCompatibility: resources.podConfig.XfsCompatibility,
			Settings:         envSettings,
			VDDKImage:        resources.vddkImage,
			LocalMigration:   resources.localMigration,
			PodSettings: api.PodSettings{
				ServiceAccount:             resolveServiceAccount(r.Plan),
				Affinity:                   resources.podConfig.Affinity,
				GenerateName:               resources.podConfig.GenerateName,
				TransferNetworkAnnotations: resources.podConfig.TransferNetworkAnnotations,
				NodeSelector:               resources.podConfig.PodNodeSelector,
				RequestKVM:                 resources.podConfig.RequestKVM,
			},
			ExtraVolumes: resources.extraVolumes,
			ExtraMounts:  resources.extraMounts,
		},
	}

	if vm.NbdeClevis {
		conversion.Spec.DiskEncryption = &api.DiskEncryption{
			Type: api.DiskEncryptionTypeClevis,
		}
	} else if vm.LUKS.Name != "" {
		// The pod runs in TargetNamespace on the destination cluster, so the
		// LUKS secret must exist there. Copy it from the management cluster
		// (where vm.LUKS lives) to target cluster.
		sourceNS := vm.LUKS.Namespace
		if sourceNS == "" {
			sourceNS = r.Plan.Namespace
		}
		source := &core.Secret{}
		if err = r.Client.Get(context.TODO(),
			types.NamespacedName{Namespace: sourceNS, Name: vm.LUKS.Name}, source,
		); err != nil {
			err = liberr.Wrap(err)
			return
		}
		luksLabels := r.getConversionLabels(conversionType, vm.ID, planID,
			map[string]string{kLUKS: "true"})
		luksSecretSpec := r.buildConversionSecret(
			r.Plan.Spec.TargetNamespace,
			planName+"-"+vm.ID+"-luks-",
			luksLabels,
			source.Data,
		)
		var luksSecret *core.Secret
		luksSecret, err = r.ensureConversionSecret(r.Destination.Client, luksSecretSpec)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		conversion.Spec.DiskEncryption = &api.DiskEncryption{
			Type: api.DiskEncryptionTypeLUKS,
			Secret: core.ObjectReference{
				Namespace: luksSecret.Namespace,
				Name:      luksSecret.Name,
			},
		}
	}

	err = r.Client.Create(context.TODO(), conversion)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	r.Log.Info(
		"Conversion CR created.",
		"conversion", path.Join(conversion.Namespace, conversion.Name),
		"type", string(conversionType),
		"vm", vm.String())
	return
}

// getConversionLabels returns the base label set for a Conversion CR or its
// associated secrets. convType and vmID are always included; planID is added
// when non-empty. Entries in extra override / extend the base set.
func (r *KubeVirt) getConversionLabels(convType api.ConversionType, vmID, planID string, extra map[string]string) map[string]string {
	labels := map[string]string{
		convctx.LabelVM:             vmID,
		convctx.LabelConversionType: string(convType),
	}
	if planID != "" {
		labels[convctx.LabelPlan] = planID
	}
	for k, v := range extra {
		labels[k] = v
	}
	return labels
}

// getConversion looks up a single Conversion CR in Plan.Namespace that matches
// all supplied labels. Returns nil (no error) when no match is found.
func (r *KubeVirt) getConversion(labels map[string]string) (*api.Conversion, error) {
	list := &api.ConversionList{}
	if err := r.Client.List(context.TODO(), list,
		client.InNamespace(r.Plan.Namespace),
		client.MatchingLabels(labels),
	); err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) == 0 {
		return nil, nil //nolint:nilnil
	}
	return &list.Items[0], nil
}

// buildConversion constructs a Conversion CR in memory from a ready-made Spec.
func (r *KubeVirt) buildConversion(planName, vmID string, labels map[string]string, spec api.ConversionSpec) *api.Conversion {
	return &api.Conversion{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Namespace,
			GenerateName: planName + "-" + vmID + "-",
			Labels:       labels,
		},
		Spec: spec,
	}
}

// ensureConversion creates or updates the Conversion CR on the cluster.
// An existing CR is found by matching all labels (including LabelPlan). When
// found its Spec is refreshed; otherwise the provided CR is created verbatim.
func (r *KubeVirt) ensureConversion(cr *api.Conversion) (*api.Conversion, error) {
	list := &api.ConversionList{}
	if err := r.Client.List(context.TODO(), list,
		client.InNamespace(cr.Namespace),
		client.MatchingLabels(cr.Labels),
	); err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) > 0 {
		existing := &list.Items[0]
		existing.Spec = cr.Spec
		if err := r.Client.Update(context.TODO(), existing); err != nil {
			return nil, liberr.Wrap(err)
		}
		r.Log.Info("Conversion CR updated.",
			"conversion", path.Join(existing.Namespace, existing.Name),
			"type", string(existing.Spec.Type))
		return existing, nil
	}
	if err := r.Client.Create(context.TODO(), cr); err != nil {
		return nil, liberr.Wrap(err)
	}
	r.Log.Info("Conversion CR created.",
		"conversion", path.Join(cr.Namespace, cr.Name),
		"type", string(cr.Spec.Type))
	return cr, nil
}

// buildConversionSecret constructs a Secret spec in memory for a Conversion pod.
func (r *KubeVirt) buildConversionSecret(namespace, generateName string, labels map[string]string, data map[string][]byte) *core.Secret {
	return &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    namespace,
			GenerateName: generateName,
			Labels:       labels,
		},
		Data: data,
	}
}

// ensureConversionSecret creates or updates a Secret using the supplied client.
// Existing secrets are found by label-match, excluding LabelPlan (it changes).
func (r *KubeVirt) ensureConversionSecret(cl client.Client, secret *core.Secret) (*core.Secret, error) {
	lookupLabels := make(map[string]string, len(secret.Labels))
	for k, v := range secret.Labels {
		if k != convctx.LabelPlan {
			lookupLabels[k] = v
		}
	}
	list := &core.SecretList{}
	if err := cl.List(context.TODO(), list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(lookupLabels),
			Namespace:     secret.Namespace,
		},
	); err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) > 0 {
		existing := &list.Items[0]
		existing.Data = secret.Data
		if err := cl.Update(context.TODO(), existing); err != nil {
			return nil, liberr.Wrap(err)
		}
		r.Log.V(1).Info("Conversion secret updated.",
			"secret", path.Join(existing.Namespace, existing.Name))
		return existing, nil
	}
	if err := cl.Create(context.TODO(), secret); err != nil {
		return nil, liberr.Wrap(err)
	}
	r.Log.V(1).Info("Conversion secret created.",
		"secret", path.Join(secret.Namespace, secret.Name))
	return secret, nil
}

// GetGuestConversion returns the guest-conversion (Remote or InPlace) Conversion CR
// for the given VM on this plan, or nil when none exists.
func (r *KubeVirt) GetGuestConversion(vm *plan.VMStatus) (*api.Conversion, error) {
	typeReq, err := k8slabels.NewRequirement(
		convctx.LabelConversionType,
		selection.In,
		[]string{string(api.Remote), string(api.InPlace)},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	selector := k8slabels.SelectorFromSet(map[string]string{
		convctx.LabelPlan: string(r.Plan.UID),
		convctx.LabelVM:   vm.ID,
	}).Add(*typeReq)

	list := &api.ConversionList{}
	if err := r.List(context.TODO(), list,
		client.InNamespace(r.Plan.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil //nolint:nilnil
}

// GetDeepInspectionConversion returns the DeepInspection Conversion CR for the
// given VM on this plan, or nil when none exists.
func (r *KubeVirt) GetDeepInspectionConversion(vm *plan.VMStatus) (*api.Conversion, error) {
	labels := r.getConversionLabels(api.DeepInspection, vm.ID, string(r.Plan.UID), nil)
	return r.getConversion(labels)
}

// CreateDeepInspectionConversion creates a new DeepInspection Conversion CR and
// returns it.
func (r *KubeVirt) CreateDeepInspectionConversion(
	vm *plan.VMStatus, snapshotMoref, planName, planID string,
) (*api.Conversion, error) {
	// Connection secret goes to Plan.Namespace on the management cluster
	// (DeepInspection pods run there, not on the destination cluster).
	connSecretData := r.buildDeepInspectionConnectionSecretData()
	connLabels := r.getConversionLabels(api.DeepInspection, vm.Ref.ID, planID,
		map[string]string{kConnection: "true"})
	connSecretSpec := r.buildConversionSecret(
		r.Plan.Namespace,
		planName+"-"+vm.Ref.ID+"-di-",
		connLabels,
		connSecretData,
	)
	connSecret, err := r.ensureConversionSecret(r.Client, connSecretSpec)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	// Build and ensure the disk-encryption secret(s) when needed.
	var diskEncryption *api.DiskEncryption
	switch {
	case vm.NbdeClevis:
		diskEncryption = &api.DiskEncryption{Type: api.DiskEncryptionTypeClevis}
	case vm.LUKS.Name != "":
		sourceNS := vm.LUKS.Namespace
		if sourceNS == "" {
			sourceNS = r.Plan.Namespace
		}
		source := &core.Secret{}
		if err = r.Client.Get(context.TODO(),
			types.NamespacedName{Namespace: sourceNS, Name: vm.LUKS.Name}, source,
		); err != nil {
			return nil, liberr.Wrap(err)
		}
		luksLabels := r.getConversionLabels(api.DeepInspection, vm.Ref.ID, planID,
			map[string]string{kLUKS: "true"})
		luksSecretSpec := r.buildConversionSecret(
			r.Plan.Namespace,
			planName+"-"+vm.Ref.ID+"-di-luks-",
			luksLabels,
			source.Data,
		)
		luksSecret, luksErr := r.ensureConversionSecret(r.Client, luksSecretSpec)
		if luksErr != nil {
			return nil, liberr.Wrap(luksErr)
		}
		diskEncryption = &api.DiskEncryption{
			Type: api.DiskEncryptionTypeLUKS,
			Secret: core.ObjectReference{
				Namespace: luksSecret.Namespace,
				Name:      luksSecret.Name,
			},
		}
	}

	// Empty Destination → resolveDestinationClient returns localClient →
	// pod is created on the management cluster in Plan.Namespace.
	crLabels := r.getConversionLabels(api.DeepInspection, vm.ID, planID, nil)
	spec := api.ConversionSpec{
		Type:            api.DeepInspection,
		TargetNamespace: r.Plan.Namespace,
		VM:              vm.Ref,
		Connection: api.Connection{
			Secret: core.ObjectReference{
				Namespace: connSecret.Namespace,
				Name:      connSecret.Name,
			},
		},
		Settings: map[string]string{
			api.SpecSettingsSnapshotMorefKey: snapshotMoref,
		},
		XfsCompatibility: r.Plan.Spec.XfsCompatibility,
		VDDKImage:        settings.GetVDDKImage(r.Source.Provider.Spec.Settings),
		DiskEncryption:   diskEncryption,
		PodSettings: api.PodSettings{
			ServiceAccount: resolveServiceAccount(r.Plan),
		},
	}
	cr := r.buildConversion(planName, vm.ID, crLabels, spec)
	return r.ensureConversion(cr)
}

// DeleteConversion tears down a Conversion CR: pod → snapshot → secrets → CR.
func (r *KubeVirt) DeleteConversion(cr *api.Conversion) error {
	if err := r.deleteConversionPod(cr); err != nil {
		return err
	}
	if err := r.removeOwnedSnapshotForCR(cr); err != nil {
		r.Log.Error(err, "Failed to remove owned snapshot during Conversion deletion; continuing.",
			"conversion", path.Join(cr.Namespace, cr.Name))
	}
	if cr.Spec.TargetNamespace != "" {
		if err := r.deleteConversionSecrets(cr); err != nil {
			return err
		}
	}
	if err := r.Client.Delete(context.TODO(), cr); err != nil && !k8serr.IsNotFound(err) {
		return liberr.Wrap(err)
	}
	r.Log.Info("Conversion CR deleted.",
		"conversion", path.Join(cr.Namespace, cr.Name),
		"type", string(cr.Spec.Type))
	return nil
}

// DeleteAllConversions deletes every Conversion CR that was created for the
// given VM on this plan, together with their pods and secrets.
func (r *KubeVirt) DeleteAllConversions(vm *plan.VMStatus) error {
	labels := map[string]string{
		convctx.LabelPlan: string(r.Plan.UID),
		convctx.LabelVM:   vm.Ref.ID,
	}
	list := &api.ConversionList{}
	if err := r.Client.List(context.TODO(), list,
		client.InNamespace(r.Plan.Namespace),
		client.MatchingLabels(labels),
	); err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		if err := r.DeleteConversion(&list.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// deleteConversionPod finds the pod that was created for the given Conversion
// CR and deletes it. For DeepInspection the pod lives on the management cluster
// for all other types it lives on the destination cluster.
func (r *KubeVirt) deleteConversionPod(cr *api.Conversion) error {
	cl := r.Destination.Client
	if cr.Spec.Type == api.DeepInspection {
		cl = r.Client
	}
	matchLabels := map[string]string{
		convctx.LabelVM:             cr.Labels[convctx.LabelVM],
		convctx.LabelConversionType: string(cr.Spec.Type),
	}
	if matchLabels[convctx.LabelVM] == "" {
		return nil
	}
	list := &core.PodList{}
	if err := cl.List(context.TODO(), list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(matchLabels),
			Namespace:     cr.Spec.TargetNamespace,
		},
	); err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		pod := &list.Items[i]
		if err := cl.Delete(context.TODO(), pod); err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		r.Log.V(1).Info("Conversion pod deleted.",
			"pod", path.Join(pod.Namespace, pod.Name))
	}
	return nil
}

// removeOwnedSnapshotForCR triggers vSphere snapshot removal (fire-and-forget).
// only done for ddep inspection type.
func (r *KubeVirt) removeOwnedSnapshotForCR(cr *api.Conversion) error {
	ensurer, err := convbuilder.NewEnsurer(r.Client, r.Log, cr.Spec)
	if err != nil {
		return err
	}
	_, err = ensurer.RemoveOwnedSnapshot(context.TODO(), cr)
	return err
}

// deleteConversionSecrets deletes all secrets owned by a Conversion CR.
func (r *KubeVirt) deleteConversionSecrets(cr *api.Conversion) error {
	vmID, ok := cr.Labels[convctx.LabelVM]
	if !ok || vmID == "" {
		return nil
	}

	// DeepInspection secrets live on the management cluster.
	cl := r.Destination.Client
	if cr.Spec.Type == api.DeepInspection {
		cl = r.Client
	}
	matchLabels := map[string]string{
		convctx.LabelVM:             vmID,
		convctx.LabelConversionType: string(cr.Spec.Type),
	}
	list := &core.SecretList{}
	if err := cl.List(context.TODO(), list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(matchLabels),
			Namespace:     cr.Spec.TargetNamespace,
		},
	); err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		s := &list.Items[i]
		if err := cl.Delete(context.TODO(), s); err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		r.Log.V(1).Info("Conversion secret deleted.",
			"secret", path.Join(s.Namespace, s.Name))
	}

	// v2v credentials secret (not labelled with conversion-type).
	if cr.Spec.Type == api.DeepInspection {
		return nil
	}
	v2vList := &core.SecretList{}
	if err := r.Destination.Client.List(context.TODO(), v2vList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(map[string]string{
				convctx.LabelVM: vmID,
				kV2V:            "true",
			}),
			Namespace: cr.Spec.TargetNamespace,
		},
	); err != nil {
		return liberr.Wrap(err)
	}
	for i := range v2vList.Items {
		s := &v2vList.Items[i]
		if err := r.Destination.Client.Delete(context.TODO(), s); err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		r.Log.V(1).Info("Conversion v2v secret deleted.",
			"secret", path.Join(s.Namespace, s.Name))
	}
	return nil
}

// buildDeepInspectionConnectionSecretData assembles the secret payload for the
// deep-inspection pod's connection secret. It copies the source provider
// credentials and injects the provider URL and fingerprint so the pod can read
// them from /etc/secret/ without needing the Provider CR.
func (r *KubeVirt) buildDeepInspectionConnectionSecretData() map[string][]byte {
	data := make(map[string][]byte, len(r.Source.Secret.Data)+2)
	for k, v := range r.Source.Secret.Data {
		data[k] = v
	}
	if r.Source.Provider.Spec.URL != "" {
		data["url"] = []byte(r.Source.Provider.Spec.URL)
	}
	if r.Source.Provider.Status.Fingerprint != "" {
		data["fingerprint"] = []byte(r.Source.Provider.Status.Fingerprint)
	}
	return data
}

// CancelConversion finds the Conversion CR for the given VM and marks it as Canceled.
func (r *KubeVirt) CancelConversion(vm *plan.VMStatus) error {
	labels := map[string]string{
		convctx.LabelPlan: string(r.Plan.UID),
		convctx.LabelVM:   vm.Ref.ID,
	}
	list := &api.ConversionList{}
	err := r.Client.List(context.TODO(), list,
		client.InNamespace(r.Plan.Namespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		conv := &list.Items[i]
		if conv.Status.Phase == api.PhaseSucceeded || conv.Status.Phase == api.PhaseFailed || conv.Status.Phase == api.PhaseCanceled {
			continue
		}
		conv.Status.Phase = api.PhaseCanceled
		conv.Status.Stage = api.StageFinished
		if err = r.Client.Status().Update(context.TODO(), conv); err != nil {
			return liberr.Wrap(err)
		}
		r.Log.Info("Conversion CR canceled.", "conversion", path.Join(conv.Namespace, conv.Name), "vm", vm.String())
	}
	return nil
}

// EnsureGuestConversionPod resolves all data and creates the conversion pod
// via conversion.EnsureVirtV2vPod.
func (r *KubeVirt) EnsureGuestConversionPod(vm *plan.VMStatus, step *plan.Step) (ready bool, err error) {
	res, err := r.resolveConversionResources(vm, convctx.VirtV2vConversionPod, nil)
	if err != nil {
		return
	}
	if !res.ready {
		return false, nil
	}

	if util.AnyNetAppShiftPersistentVolumeClaim(res.pvcs) {
		res.podConfig.ExtraInitContainers = append(res.podConfig.ExtraInitContainers,
			netAppShiftDiskPermsInitContainer(res.mounts, getVirtV2vImage(r.Plan)))
	}

	err = convbuilder.EnsureVirtV2vPod(r.Destination.Client, r.Log, vm, res.volumes, res.mounts, res.devices, res.secret, convctx.VirtV2vConversionPod, res.inPlace, res.podConfig)
	if err != nil {
		return
	}

	return r.checkProviderReady(vm.ID)
}

// EnsureWaitForRebootPod creates a pod that runs the forklift-wait-for-reboot binary to monitor the target VMI serial console (idempotent).
func (r *KubeVirt) EnsureWaitForRebootPod(vm *plan.VMStatus) (err error) {
	nonRoot := true
	allowPrivilegeEscalation := false
	user := qemuUser

	img := getVirtV2vImage(r.Plan)
	if img == "" {
		err = liberr.New("virt-v2v image is not set; cannot create Windows wait-for-reboot pod")
		return
	}

	existing, gErr := r.GetWaitForRebootPod(vm)
	if gErr != nil {
		err = liberr.Wrap(gErr)
		return
	}
	if existing != nil {
		return nil
	}

	if err = r.ensureWaitForRebootRBAC(r.Plan.Spec.TargetNamespace); err != nil {
		return
	}

	activeDeadline := int64(settings.Settings.WindowsRebootTimeout + 600)
	automount := true

	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: "forklift-wait-reboot-",
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       r.waitForRebootLabels(vm.Ref),
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				RunAsNonRoot:   &nonRoot,
				SeccompProfile: &core.SeccompProfile{Type: core.SeccompProfileTypeRuntimeDefault},
			},
			RestartPolicy:                core.RestartPolicyNever,
			ServiceAccountName:           waitForRebootSAName,
			AutomountServiceAccountToken: &automount,
			ActiveDeadlineSeconds:        &activeDeadline,
			Containers: []core.Container{
				{
					Name:    "forklift-wait-for-reboot",
					Image:   img,
					Command: []string{"/usr/local/bin/forklift-wait-for-reboot"},
					Env: []core.EnvVar{
						{Name: "VMI_NAME", Value: r.getNewVMName(vm)},
						{Name: "VMI_NAMESPACE", Value: r.Plan.Spec.TargetNamespace},
						{Name: "SIGNAL", Value: "CONVERSION_DONE"},
						{Name: "TIMEOUT", Value: strconv.Itoa(settings.Settings.WindowsRebootTimeout)},
					},
					SecurityContext: &core.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilegeEscalation,
						RunAsUser:                &user,
						Capabilities:             &core.Capabilities{Drop: []core.Capability{"ALL"}},
					},
				},
			},
		},
	}
	err = r.Destination.Create(context.TODO(), pod)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info("Created Windows wait-for-reboot pod.", "podNamespace", pod.Namespace, "podName", pod.Name, "vm", vm.String())
	return nil
}

const waitForRebootSAName = "forklift-wait-reboot"

// ensureWaitForRebootRBAC creates a dedicated ServiceAccount, Role, and RoleBinding
// in the target namespace granting only VMI serial console access.
func (r *KubeVirt) ensureWaitForRebootRBAC(namespace string) error {
	sa := &core.ServiceAccount{
		ObjectMeta: meta.ObjectMeta{
			Name:      waitForRebootSAName,
			Namespace: namespace,
		},
	}
	if err := r.Destination.Create(context.TODO(), sa); err != nil && !k8serr.IsAlreadyExists(err) {
		return liberr.Wrap(err)
	}

	// rules for the wait-for-reboot pod
	desiredRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"subresources.kubevirt.io"},
			Resources: []string{"virtualmachineinstances/console"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{"kubevirt.io"},
			Resources: []string{"virtualmachineinstances"},
			Verbs:     []string{"get"},
		},
	}

	role := &rbacv1.Role{
		ObjectMeta: meta.ObjectMeta{
			Name:      waitForRebootSAName,
			Namespace: namespace,
		},
		Rules: desiredRules,
	}
	if err := r.Destination.Create(context.TODO(), role); err != nil {
		if !k8serr.IsAlreadyExists(err) {
			return liberr.Wrap(err)
		}
		existing := &rbacv1.Role{}
		if err = r.Destination.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: waitForRebootSAName}, existing); err != nil {
			return liberr.Wrap(err)
		}
		if !reflect.DeepEqual(existing.Rules, desiredRules) {
			existing.Rules = desiredRules
			if err = r.Destination.Update(context.TODO(), existing); err != nil {
				return liberr.Wrap(err)
			}
		}
	}

	desiredSubjects := []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      waitForRebootSAName,
			Namespace: namespace,
		},
	}
	binding := &rbacv1.RoleBinding{
		ObjectMeta: meta.ObjectMeta{
			Name:      waitForRebootSAName + "-binding",
			Namespace: namespace,
		},
		Subjects: desiredSubjects,
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     waitForRebootSAName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	if err := r.Destination.Create(context.TODO(), binding); err != nil {
		if !k8serr.IsAlreadyExists(err) {
			return liberr.Wrap(err)
		}
		existing := &rbacv1.RoleBinding{}
		if err = r.Destination.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: waitForRebootSAName + "-binding"}, existing); err != nil {
			return liberr.Wrap(err)
		}
		// roleRef is immutable, the binding must be replaced entirely if it's different
		// subjects diff can be fixed with a plain update
		if !reflect.DeepEqual(existing.RoleRef, binding.RoleRef) {
			if err = r.Destination.Delete(context.TODO(), existing); err != nil {
				return liberr.Wrap(err)
			}
			if err = r.Destination.Create(context.TODO(), binding); err != nil {
				return liberr.Wrap(err)
			}
		} else if !reflect.DeepEqual(existing.Subjects, desiredSubjects) {
			existing.Subjects = desiredSubjects
			if err = r.Destination.Update(context.TODO(), existing); err != nil {
				return liberr.Wrap(err)
			}
		}
	}
	return nil
}

// GetWaitForRebootPod finds the GetWaitForRebootPod-wait-for-reboot pod for the VM migration, if present.
func (r *KubeVirt) GetWaitForRebootPod(vm *plan.VMStatus) (*core.Pod, error) {
	list, err := r.GetPodsWithLabels(r.waitForRebootLabels(vm.Ref))
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) == 0 {
		return nil, nil //nolint:nilnil
	}
	return &list.Items[0], nil
}

// DeleteWaitForRebootPod removes the forklift-wait-for-reboot pod for the VM migration.
func (r *KubeVirt) DeleteWaitForRebootPod(vm *plan.VMStatus) error {
	pod, err := r.GetWaitForRebootPod(vm)
	if err != nil || pod == nil {
		return err
	}
	err = r.Destination.Delete(context.TODO(), pod)
	if err != nil && !k8serr.IsNotFound(err) {
		return liberr.Wrap(err)
	}
	return nil
}

// EnsureGuestInspectionPod resolves all data and creates the inspection pod
// via conversion.EnsureVirtV2vPod.
func (r *KubeVirt) EnsureGuestInspectionPod(vm *plan.VMStatus, step *plan.Step) (ready bool, err error) {
	res, err := r.resolveConversionResources(vm, convctx.VirtV2vInspectionPod, step)
	if err != nil {
		return
	}
	if !res.ready {
		return false, nil
	}

	err = convbuilder.EnsureVirtV2vPod(
		r.Destination.Client, r.Log, vm,
		res.volumes, res.mounts, res.devices,
		res.secret, convctx.VirtV2vInspectionPod, false, res.podConfig)
	if err != nil {
		return
	}

	return r.checkProviderReady(vm.ID)
}

// GetConversionPod returns the managed pod for the given VM ref and pod type.
func (r *KubeVirt) GetConversionPod(vmRef ref.Ref, podType convctx.V2vPodType) (*core.Pod, error) {
	var labels map[string]string
	switch podType {
	case convctx.VirtV2vConversionPod:
		labels = r.conversionLabels(vmRef, true)
	case convctx.VirtV2vInspectionPod:
		labels = r.inspectionLabels(vmRef)
	}
	list, err := r.GetPodsWithLabels(labels)
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil //nolint:nilnil
}

// ensureV2vSecret ensures the v2v secret exists for the given VM.
func (r *KubeVirt) ensureV2vSecret(vmRef ref.Ref) (*core.Secret, error) {
	labels := r.vmLabels(vmRef)
	labels[kV2V] = "true"
	return r.ensureSecret(vmRef, r.secretDataSetterForCDI(vmRef), labels)
}

// getVMVolumes returns the volumes from the KubeVirt VM spec.
func (r *KubeVirt) getVMVolumes(vm *plan.VMStatus) ([]cnv.Volume, error) {
	vmCr, err := r.virtualMachine(vm, true)
	if err != nil {
		return nil, err
	}
	return vmCr.Spec.Template.Spec.Volumes, nil
}

// resolveServiceAccount resolves the ServiceAccount for migration pods.
// Priority: Plan.Spec.ServiceAccount > Settings.Migration.ServiceAccount > "" (namespace default).
func resolveServiceAccount(plan *api.Plan) string {
	return cmp.Or(plan.Spec.ServiceAccount, Settings.Migration.ServiceAccount)
}

// CNINetworkConfig represents a CNI network configuration parsed from a NetworkAttachmentDefinition.
// This includes the IPAM configuration with routes for determining default gateway.
type CNINetworkConfig struct {
	IPAM CNIIPAMConfig `json:"ipam"`
}

// CNIIPAMConfig represents the IPAM section of a CNI network configuration.
type CNIIPAMConfig struct {
	Routes []CNIRoute `json:"routes"`
}

// CNIRoute represents a single route entry in the CNI IPAM configuration.
type CNIRoute struct {
	Dst string `json:"dst"` // Destination network in CIDR notation (e.g., "0.0.0.0/0" for default route)
	GW  string `json:"gw"`  // Gateway IP address
}

// Build a VirtualMachineMap.
func (r *KubeVirt) VirtualMachineMap() (mp VirtualMachineMap, err error) {
	list, err := r.ListVMs()
	if err != nil {
		return
	}
	mp = VirtualMachineMap{}
	for _, object := range list {
		mp[object.Labels[kVM]] = object
	}

	return
}

// List VirtualMachine CRs.
// Each VirtualMachine represents an imported kubevirt VM with associated DataVolumes.
func (r *KubeVirt) ListVMs() ([]VirtualMachine, error) {
	planLabels := r.planLabels()
	delete(planLabels, kMigration)
	vList := &cnv.VirtualMachineList{}
	err := r.Destination.Client.List(
		context.TODO(),
		vList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(planLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	list := []VirtualMachine{}
	for i := range vList.Items {
		vm := &vList.Items[i]
		list = append(
			list,
			VirtualMachine{
				VirtualMachine: vm,
			})
	}
	dvList := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvList,
		r.getListOptionsNamespaced(),
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for i := range list {
		vm := &list[i]
		for i := range dvList.Items {
			dv := &dvList.Items[i]
			if vm.Owner(dv) {
				pvc := &core.PersistentVolumeClaim{}
				err = r.Destination.Client.Get(
					context.TODO(),
					types.NamespacedName{Namespace: r.Plan.Spec.TargetNamespace, Name: dv.Name},
					pvc,
				)
				if err != nil && !k8serr.IsNotFound(err) {
					return nil, liberr.Wrap(err)
				}
				vm.DataVolumes = append(
					vm.DataVolumes,
					ExtendedDataVolume{
						DataVolume: dv,
						PVC:        pvc,
					})
			}
		}
	}

	return list, nil
}

// Ensure the namespace exists on the destination.
func (r *KubeVirt) EnsureNamespace() error {
	err := ensureNamespace(r.Plan, r.Destination.Client)
	if err == nil {
		r.Log.Info(
			"Created namespace.",
			"import",
			r.Plan.Spec.TargetNamespace)
	}
	return err
}

// Ensure the config map that contains extra configuration for virt-v2v exists on the destination.
func (r *KubeVirt) EnsureExtraV2vConfConfigMap() error {
	if len(Settings.Migration.VirtV2vExtraConfConfigMap) == 0 {
		return nil
	}
	configMap := &core.ConfigMap{}
	err := r.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Name:      Settings.Migration.VirtV2vExtraConfConfigMap,
			Namespace: r.Plan.Namespace,
		},
		configMap,
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	err = ensureConfigMap(configMap, genExtraV2vConfConfigMapName, r.Plan, r.Destination.Client)
	if err == nil {
		r.Log.Info(
			"Created config map for extra configuration for virt-v2v.",
			"target namespace",
			r.Plan.Spec.TargetNamespace)
	}
	return err
}

func genExtraV2vConfConfigMapName(plan *api.Plan) string {
	return fmt.Sprintf("%s-%s", plan.Name, ExtraV2vConf)
}

func genVddkConfConfigMapName(plan *api.Plan) string {
	return fmt.Sprintf("%s-%s-", plan.Name, VddkConf)
}

func genCustomizationScriptsConfigMapName(plan *api.Plan) string {
	return fmt.Sprintf("%s-%s", plan.Name, CustomizationScripts)
}

// Ensure the ConfigMap referenced by CustomizationScripts exists in TargetNamespace.
// If the ConfigMap lives in a different namespace, it is copied to the target.
func (r *KubeVirt) EnsureCustomizationScriptsConfigMap() error {
	if r.Plan.Spec.CustomizationScripts == nil {
		return nil
	}

	configMapNamespace := r.Plan.Spec.CustomizationScripts.Namespace
	if configMapNamespace == "" {
		configMapNamespace = r.Plan.Namespace
	}
	if configMapNamespace == r.Plan.Spec.TargetNamespace {
		return nil
	}

	configMap := &core.ConfigMap{}
	err := r.Get(
		context.TODO(),
		client.ObjectKey{
			Name:      r.Plan.Spec.CustomizationScripts.Name,
			Namespace: configMapNamespace,
		},
		configMap,
	)
	if err != nil {
		return liberr.Wrap(err)
	}

	err = ensureConfigMap(configMap, genCustomizationScriptsConfigMapName, r.Plan, r.Destination.Client)
	if err == nil {
		r.Log.V(4).Info(
			"Ensured ConfigMap for customization scripts in target namespace.",
			"configMap namespace", configMapNamespace,
			"target namespace", r.Plan.Spec.TargetNamespace)
	}
	return err
}

// CleanupCopiedConfigMaps deletes the extra-v2v-conf, customization-scripts,
// and vddk-conf ConfigMaps that were copied to the plan's TargetNamespace at
// migration start. Safe to call regardless of migration outcome.
func (r *KubeVirt) CleanupCopiedConfigMaps() {
	ns := r.Plan.Spec.TargetNamespace
	cl := r.Destination.Client

	// extra-v2v-conf and customization-scripts use deterministic names (delete by name).
	for _, name := range []string{
		genExtraV2vConfConfigMapName(r.Plan),
		genCustomizationScriptsConfigMapName(r.Plan),
	} {
		cm := &core.ConfigMap{}
		cm.Name = name
		cm.Namespace = ns
		if err := cl.Delete(context.TODO(), cm); err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete copied ConfigMap.", "name", name, "namespace", ns)
		} else if err == nil {
			r.Log.Info("Deleted copied ConfigMap.", "name", name, "namespace", ns)
		}
	}

	// vddk-conf uses GenerateName so must be found by label selector.
	list := &core.ConfigMapList{}
	err := cl.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vddkLabels()),
			Namespace:     ns,
		},
	)
	if err != nil {
		r.Log.Error(err, "Failed to list vddk-conf ConfigMaps for cleanup.", "namespace", ns)
		return
	}
	for i := range list.Items {
		if err := cl.Delete(context.TODO(), &list.Items[i]); err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete vddk-conf ConfigMap.", "name", list.Items[i].Name, "namespace", ns)
		} else if err == nil {
			r.Log.Info("Deleted vddk-conf ConfigMap.", "name", list.Items[i].Name, "namespace", ns)
		}
	}
}

// Get the importer pod for a PersistentVolumeClaim.
func (r *KubeVirt) GetImporterPod(pvc core.PersistentVolumeClaim) (pod *core.Pod, found bool, err error) {
	pod = &core.Pod{}
	if pvc.Annotations[AnnImporterPodName] == "" {
		return
	}

	err = r.Destination.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      pvc.Annotations[AnnImporterPodName],
			Namespace: r.Plan.Spec.TargetNamespace,
		},
		pod,
	)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
			return
		}
		err = liberr.Wrap(err)
		return
	}

	found = true
	return
}

// Get the importer pods for a PersistentVolumeClaim.
func (r *KubeVirt) getImporterPods(pvc *core.PersistentVolumeClaim) (pods []core.Pod, err error) {
	if _, ok := pvc.Annotations[AnnImporterPodName]; !ok {
		return
	}

	podList := &core.PodList{}
	err = r.Destination.Client.List(
		context.TODO(),
		podList,
		&client.ListOptions{
			Namespace:     r.Plan.Spec.TargetNamespace,
			LabelSelector: k8slabels.SelectorFromSet(map[string]string{"app": "containerized-data-importer"}),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, pod := range podList.Items {
		if strings.Contains(pod.Name, fmt.Sprintf("importer-%s", pvc.Name)) {
			pods = append(pods, pod)
		}
	}

	return
}

// Delete the DataVolumes associated with the VM.
func (r *KubeVirt) DeleteDataVolumes(vm *plan.VMStatus) (err error) {
	dvs, err := r.getDVs(vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, dv := range dvs {
		r.Log.Info(
			"Deleting DataVolume.",
			"dv",
			path.Join(dv.Namespace, dv.Name),
			"vm",
			vm.String())
		err = r.Destination.Client.Delete(context.TODO(), dv.DataVolume)
		if err != nil {
			return
		}
	}
	return
}

// Delete the importer pods for a PersistentVolumeClaim.
func (r *KubeVirt) DeleteImporterPods(pvc *core.PersistentVolumeClaim) (err error) {
	pods, err := r.getImporterPods(pvc)
	if err != nil {
		return
	}
	for _, pod := range pods {
		err = r.Destination.Client.Delete(context.TODO(), &pod)
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(
				err,
				"Deleting importer pod failed.",
				"pod",
				path.Join(
					pod.Namespace,
					pod.Name),
				"pvc",
				pvc.Name)
			continue
		}
		r.Log.Info(
			"Deleted importer pod.",
			"pod",
			path.Join(
				pod.Namespace,
				pod.Name),
			"pvc",
			pvc.Name)
	}
	return
}

func (r *KubeVirt) DeleteJobs(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmAllButMigrationLabels(vm.Ref)
	list := &batch.JobList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(vmLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info("Found jobs to delete.", "count", len(list.Items), "vm", vm.String())

	jobNames := []string{}
	for _, job := range list.Items {
		err = r.DeleteObject(&job, vm, "Deleted job.", "job")
		if err != nil {
			err = liberr.Wrap(err)
			r.Log.Error(
				err,
				"Deleting job failed.",
				"job",
				path.Join(
					job.Namespace,
					job.Name))
			continue
		}

		jobNames = append(jobNames, job.Name)
	}

	// One day we'll figure out why client.PropagationPolicy(meta.DeletePropagationBackground) doesn't remove the pods
	for _, job := range jobNames {
		podList := &core.PodList{}
		err = r.Destination.Client.List(
			context.TODO(),
			podList,
			&client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(map[string]string{"job-name": job}),
				Namespace:     r.Plan.Spec.TargetNamespace,
			},
		)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		for _, pod := range podList.Items {
			err = r.DeleteObject(&pod, vm, "Deleted job pod.", "pod")
			if err != nil {
				err = liberr.Wrap(err)
				r.Log.Error(
					err,
					"Deleting pod failed.",
					"pod",
					path.Join(
						pod.Namespace,
						pod.Name))
				continue
			}
		}
	}

	return
}

// Ensure the kubevirt VirtualMachine exists on the destination.
func (r *KubeVirt) EnsureVM(vm *plan.VMStatus) error {
	vms := &cnv.VirtualMachineList{}
	err := r.Destination.Client.List(
		context.TODO(),
		vms,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}

	var virtualMachine *cnv.VirtualMachine
	if len(vms.Items) == 0 {
		if virtualMachine, err = r.virtualMachine(vm, false); err != nil {
			return liberr.Wrap(err)
		}
		if err = r.Destination.Client.Create(context.TODO(), virtualMachine); err != nil {
			return liberr.Wrap(err)
		}
		r.Log.Info(
			"Created Kubevirt VM.",
			"vm",
			path.Join(
				virtualMachine.Namespace,
				virtualMachine.Name),
			"source",
			vm.String())
	} else {
		virtualMachine = &vms.Items[0]
	}

	// set DataVolume owner references so that they'll be cleaned up
	// when the VirtualMachine is removed.
	dvs := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		return liberr.Wrap(err)
	}
	pvcs, err := r.getPVCs(vm.Ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, pvc := range pvcs {
		ownerRefs := []meta.OwnerReference{vmOwnerReference(virtualMachine)}
		pvcCopy := pvc.DeepCopy()
		pvc.OwnerReferences = ownerRefs
		patch := client.MergeFrom(pvcCopy)
		err = r.Destination.Client.Patch(context.TODO(), pvc, patch)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	return nil
}

// Delete the Secret that was created for this VM.
func (r *KubeVirt) DeleteSecret(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmAllButMigrationLabels(vm.Ref)
	list := &core.SecretList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(vmLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info("Found secrets to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		err = r.DeleteObject(&object, vm, "Deleted secret.", "secret")
		if err != nil {
			return err
		}
	}
	return
}

// Delete the ConfigMap that was created for this VM.
func (r *KubeVirt) DeleteConfigMap(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmAllButMigrationLabels(vm.Ref)
	list := &core.ConfigMapList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(vmLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info("Found config maps to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		err = r.DeleteObject(&object, vm, "Deleted configMap.", "configMap")
		if err != nil {
			return err
		}
	}
	return
}

// Delete the VirtualMachine CR on the destination cluster.
func (r *KubeVirt) DeleteVM(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmAllButMigrationLabels(vm.Ref)
	list := &cnv.VirtualMachineList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(vmLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info("Found VMs to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		foreground := meta.DeletePropagationForeground
		opts := &client.DeleteOptions{PropagationPolicy: &foreground}
		err = r.Destination.Client.Delete(context.TODO(), &object, opts)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
			} else {
				return liberr.Wrap(err)
			}
		} else {
			r.Log.Info(
				"Deleted Kubevirt VM.",
				"vm",
				path.Join(
					object.Namespace,
					object.Name),
				"source",
				vm.String())
		}
	}
	return
}

func (r *KubeVirt) DataVolumes(vm *plan.VMStatus) (dataVolumes []cdi.DataVolume, err error) {
	labels := r.vmLabels(vm.Ref)
	labels[kDV] = "true"
	secret, err := r.ensureSecret(vm.Ref, r.secretDataSetterForCDI(vm.Ref), labels)
	if err != nil {
		return
	}
	configMap, err := r.ensureConfigMap(vm.Ref)
	if err != nil {
		return
	}
	var vddkConfigMap *core.ConfigMap
	if r.Source.Provider.UseVddkAioOptimization() {
		vddkConfigMap, err = r.ensureVddkConfigMap()
		if err != nil {
			return nil, err
		}
	}

	dataVolumes, err = r.dataVolumes(vm, secret, configMap, vddkConfigMap)
	if err != nil {
		return
	}
	return
}

func (r *KubeVirt) PopulatorVolumes(vmRef ref.Ref) (pvcs []*core.PersistentVolumeClaim, err error) {
	labels := r.vmLabels(vmRef)
	labels[kPopulator] = "true"
	secret, err := r.ensureSecret(vmRef, r.copyDataFromProviderSecret, labels)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	annotations := make(map[string]string)
	if sa := resolveServiceAccount(r.Plan); sa != "" {
		annotations[AnnPopulatorServiceAccount] = sa
	}
	err = r.createLunDisks(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return r.Builder.PopulatorVolumes(vmRef, annotations, secret.Name)
}

// Ensure the DataVolumes exist on the destination.
func (r *KubeVirt) EnsureDataVolumes(vm *plan.VMStatus, dataVolumes []cdi.DataVolume) (err error) {
	dataVolumeList := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dataVolumeList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	for _, dv := range dataVolumes {
		if !r.isDataVolumeExistsInList(&dv, dataVolumeList) {
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

// NetAppShiftPVCs builds PVCs for disks mapped to NetApp Shift StorageClasses.
func (r *KubeVirt) NetAppShiftPVCs(vm *plan.VMStatus) ([]core.PersistentVolumeClaim, error) {
	labels := r.vmLabels(vm.Ref)
	return r.Builder.NetAppShiftPVCs(vm.Ref, labels)
}

// CsiImportPVCs builds PVCs for disks that have CsiVolumeImport configured.
func (r *KubeVirt) CsiImportPVCs(vm *plan.VMStatus) ([]core.PersistentVolumeClaim, error) {
	labels := r.vmLabels(vm.Ref)
	return r.Builder.CsiImportPVCs(vm.Ref, labels)
}

func (r *KubeVirt) vddkConfigMap(labels map[string]string) (*core.ConfigMap, error) {
	data := make(map[string]string)
	if r.Source.Provider.UseVddkAioOptimization() {
		vddkConfig := r.Source.Provider.Spec.Settings[api.VddkConfig]
		if vddkConfig != "" {
			data["vddk-config-file"] = vddkConfig
		} else {
			data["vddk-config-file"] = "VixDiskLib.nfcAio.Session.BufSizeIn64KB=16\n" +
				"VixDiskLib.nfcAio.Session.BufCount=4"
		}
	}
	configMap := core.ConfigMap{
		Data: data,
		ObjectMeta: meta.ObjectMeta{
			GenerateName: genVddkConfConfigMapName(r.Plan),
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       labels,
		},
	}
	return &configMap, nil
}

func (r *KubeVirt) ensureVddkConfigMap() (configMap *core.ConfigMap, err error) {
	labels := r.vddkLabels()
	newConfigMap, err := r.vddkConfigMap(labels)
	if err != nil {
		return
	}

	list := &core.ConfigMapList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		configMap = &list.Items[0]
		configMap.Data = newConfigMap.Data
		err = r.Destination.Client.Update(context.TODO(), configMap)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"VDDK extra args configmap updated.",
			"configmap",
			path.Join(
				configMap.Namespace,
				configMap.Name))
	} else {
		configMap = newConfigMap
		err = r.Destination.Client.Create(context.TODO(), configMap)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"VDDK extra args configmap created.",
			"configmap",
			path.Join(
				configMap.Namespace,
				configMap.Name))
	}
	return
}

func (r *KubeVirt) EnsurePopulatorVolumes(vm *plan.VMStatus, pvcs []*core.PersistentVolumeClaim) (err error) {
	var pendingPvcNames []string
	for _, pvc := range pvcs {
		if pvc.Status.Phase == core.ClaimPending {
			pendingPvcNames = append(pendingPvcNames, pvc.Name)
		}
	}
	err = r.createPodToBindPVCs(vm, pendingPvcNames)
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (r *KubeVirt) isDataVolumeExistsInList(dv *cdi.DataVolume, dataVolumeList *cdi.DataVolumeList) bool {
	for _, item := range dataVolumeList.Items {
		if r.Builder.ResolveDataVolumeIdentifier(dv) == r.Builder.ResolveDataVolumeIdentifier(&item) {
			return true
		}
	}
	return false
}

// Return DataVolumes associated with a VM.
func (r *KubeVirt) getDVs(vm *plan.VMStatus) (edvs []ExtendedDataVolume, err error) {
	dvsList := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvsList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vmAllButMigrationLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	edvs = []ExtendedDataVolume{}
	for i := range dvsList.Items {
		dv := &dvsList.Items[i]
		edvs = append(edvs, ExtendedDataVolume{
			DataVolume: dv,
		})
	}
	return
}

// Helper function to get disk index from PVC annotation.
func getDiskIndex(pvc *core.PersistentVolumeClaim) int {
	if idx, exists := pvc.Annotations[planbase.AnnDiskIndex]; exists {
		if val, err := strconv.Atoi(idx); err == nil {
			return val
		}
	}
	return -1 // Return -1 for PVCs without index annotation
}

// Return PersistentVolumeClaims associated with a VM.
func (r *KubeVirt) getPVCs(vmRef ref.Ref) (pvcs []*core.PersistentVolumeClaim, err error) {
	pvcsList := &core.PersistentVolumeClaimList{}
	// Add VM uuid
	labelSelector := map[string]string{
		kVM: vmRef.ID,
	}
	// We need to have this in getPVCs so we create VM with corect disks, this will also help us with the guest generation
	if r.Plan.Spec.Type == api.MigrationOnlyConversion {
		v, err := r.Source.Inventory.VM(&vmRef)
		if err != nil {
			err = liberr.Wrap(err)
			return nil, err
		}
		if vm, ok := v.(*model.VM); ok {
			labelSelector[kVmUuid] = vm.UUID
		} else {
			return nil, fmt.Errorf("failed to parse the VM for only conversion mode, we need to UUID to prevent accidental overwrites, stopping migration")
		}
	} else {
		labelSelector[kMigration] = string(r.Migration.UID)
	}
	err = r.Destination.Client.List(
		context.TODO(),
		pvcsList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labelSelector),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	for i := range pvcsList.Items {
		pvc := &pvcsList.Items[i]
		if strings.HasPrefix(pvc.Name, "prime-") || !hasDiskIdentity(pvc) {
			continue
		}
		pvcs = append(pvcs, pvc)
	}

	// Sort the pvcs slice by disk index
	sort.Slice(pvcs, func(i, j int) bool {
		iIdx := getDiskIndex(pvcs[i])
		jIdx := getDiskIndex(pvcs[j])
		return iIdx < jIdx
	})

	return
}

// hasDiskIdentity reports whether the PVC carries the AnnDiskSource annotation
// that every adapter sets on real disk PVCs.
func hasDiskIdentity(pvc *core.PersistentVolumeClaim) bool {
	if pvc.Annotations == nil {
		return false
	}
	source, ok := pvc.Annotations[planbase.AnnDiskSource]
	return ok && strings.TrimSpace(source) != ""
}

// Creates the PVs and PVCs for LUN disks.
func (r *KubeVirt) createLunDisks(vmRef ref.Ref) (err error) {
	lunPvcs, err := r.Builder.LunPersistentVolumeClaims(vmRef)
	if err != nil {
		return
	}
	err = r.EnsurePersistentVolumeClaim(vmRef, lunPvcs)
	if err != nil {
		return
	}
	lunPvs, err := r.Builder.LunPersistentVolumes(vmRef)
	if err != nil {
		return
	}
	err = r.EnsurePersistentVolume(vmRef, lunPvs)
	if err != nil {
		return
	}
	return
}

// Creates a pod associated with PVCs to create node bind (wait for consumer)
func (r *KubeVirt) createPodToBindPVCs(vm *plan.VMStatus, pvcNames []string) (err error) {
	if len(pvcNames) == 0 {
		return
	}
	volumes := []core.Volume{}
	for _, pvcName := range pvcNames {
		volumes = append(volumes, core.Volume{
			Name: pvcName,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
	}
	nonRoot := true
	user := qemuUser
	allowPrivilageEscalation := false
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       r.consumerLabels(vm.Ref, false),
			GenerateName: r.getGeneratedName(vm) + "pvcinit-",
		},
		Spec: core.PodSpec{
			RestartPolicy: core.RestartPolicyNever,
			Containers: []core.Container{
				{
					Name: "main",
					// For v2v the consumer pod is used only when we execute cold migration with el9.
					// In that case, we could benefit from pulling the image of the conversion pod, so it will be present on the node.
					Image:   getVirtV2vImage(r.Plan),
					Command: []string{"/bin/sh", "-c", "exit 0"},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(Settings.Migration.VirtV2vContainerRequestsCpu),
							core.ResourceMemory: resource.MustParse(Settings.Migration.VirtV2vContainerRequestsMemory),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(Settings.Migration.VirtV2vContainerLimitsCpu),
							core.ResourceMemory: resource.MustParse(Settings.Migration.VirtV2vContainerLimitsMemory),
						},
					},
					SecurityContext: &core.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilageEscalation,
						RunAsNonRoot:             &nonRoot,
						RunAsUser:                &user,
						Capabilities: &core.Capabilities{
							Drop: []core.Capability{"ALL"},
						},
					},
				},
			},
			Volumes: volumes,
			SecurityContext: &core.PodSecurityContext{
				SeccompProfile: &core.SeccompProfile{
					Type: core.SeccompProfileTypeRuntimeDefault,
				},
			},
		},
	}
	if sa := resolveServiceAccount(r.Plan); sa != "" {
		pod.Spec.ServiceAccountName = sa
	}
	convbuilder.SetKvmOnPodSpec(&pod.Spec, shouldRequestKVM(r.Plan.Provider.Source))

	err = r.Client.Create(context.TODO(), pod, &client.CreateOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("Created pod '%s' to init the PVC node", pod.Name))
	return nil
}

func (r *KubeVirt) getListOptionsNamespaced() (listOptions *client.ListOptions) {
	return &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
	}
}

// shouldRequestKVM returns true for provider types that need KVM passthrough.
func shouldRequestKVM(provider *api.Provider) bool {
	if Settings.VirtV2vDontRequestKVM {
		return false
	}
	switch provider.Type() {
	case api.VSphere, api.Ova, api.HyperV:
		return true
	default:
		return false
	}
}

// EnsureProviderVirtV2VPVCStatus checks if the provider storage PVC is ready.
// Works for both OVA (NFS) and HyperV (SMB) PVCs.
func (r *KubeVirt) EnsureProviderVirtV2VPVCStatus(vmID string) (ready bool, err error) {
	pvcs := &core.PersistentVolumeClaimList{}

	// Build labels based on provider type
	var pvcLabels map[string]string
	switch r.Source.Provider.Type() {
	case api.Ova:
		pvcLabels = map[string]string{
			"migration": string(r.Migration.UID),
			"ova":       OvaPVCLabel,
			kVM:         vmID,
		}
	case api.HyperV:
		pvcLabels = map[string]string{
			"migration": string(r.Migration.UID),
			"hyperv":    HyperVPVCLabel,
			kVM:         vmID,
		}
	default:
		// Should not happen, but handle gracefully
		return false, nil
	}

	err = r.Destination.Client.List(
		context.TODO(),
		pvcs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(pvcLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil || len(pvcs.Items) == 0 {
		return
	}

	var pvc *core.PersistentVolumeClaim
	// In case we have leftovers for the PVCs from previous runs, and we get more than one PVC in the list,
	// we will filter by the creation timestamp.
	if len(pvcs.Items) > 1 {
		for i := range pvcs.Items {
			pvcVirtV2v := &pvcs.Items[i]
			if pvcVirtV2v.CreationTimestamp.Time.After(r.Migration.CreationTimestamp.Time) {
				pvc = pvcVirtV2v
			}
		}
		if pvc == nil {
			return
		}
	} else {
		pvc = &pvcs.Items[0]
	}

	switch pvc.Status.Phase {
	case core.ClaimBound:
		r.Log.Info("virt-v2v PVC bound", "pvc", pvc.Name)
		ready = true
	case core.ClaimPending:
		r.Log.Info("virt-v2v PVC pending", "pvc", pvc.Name)
	case core.ClaimLost:
		r.Log.Info("virt-v2v PVC lost", "pvc", pvc.Name)
		err = liberr.New("virt-v2v pvc lost")
	default:
		r.Log.Info("virt-v2v PVC status is unknown", "pvc", pvc.Name, "status", pvc.Status.Phase)
	}
	return
}

// Get the guest conversion pod for the VM.
func (r *KubeVirt) GetGuestConversionPod(vm *plan.VMStatus) (pod *core.Pod, err error) {
	list := &core.PodList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.conversionLabels(vm.Ref, false)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		pod = &list.Items[0]
	}
	return
}

func (r *KubeVirt) getInspectionXml(pod *core.Pod) (string, error) {
	if pod == nil {
		return "", liberr.New("no pod found to get the inspection")
	}
	inspectionUrl := fmt.Sprintf("http://%s:8080/inspection", pod.Status.PodIP)
	resp, err := http.Get(inspectionUrl)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	defer resp.Body.Close()
	inspectionBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", liberr.Wrap(err)
	}
	return string(inspectionBytes), nil
}

func (r *KubeVirt) UpdateVmByConvertedConfig(vm *plan.VMStatus, pod *core.Pod, step *plan.Step) error {
	if pod == nil || pod.Status.PodIP == "" {
		// we need the IP for fetching the configuration of the convered VM.
		return nil
	}

	url := fmt.Sprintf("http://%s:8080/vm", pod.Status.PodIP)

	/* Due to the virt-v2v operation, the ovf file is only available after the command's execution,
	meaning it appears following the copydisks phase.
	The server will be accessible via virt-v2v only after the command has finished.
	Until then, attempts to connect will result in a 'connection refused' error.
	Once the VM server is running, we can make a single call to obtain the OVF configuration,
	followed by a shutdown request. This will complete the pod process, allowing us to move to the next phase.
	*/
	resp, err := http.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil
		}
		return err
	}
	defer resp.Body.Close()

	vmConf, err := io.ReadAll(resp.Body)
	if err != nil {
		return liberr.Wrap(err)
	}

	switch r.Source.Provider.Type() {
	case api.Ova, api.HyperV:
		if vm.Firmware, err = util.GetFirmwareFromYaml(vmConf); err != nil {
			return liberr.Wrap(err)
		}
	case api.VSphere:
		inspectionXML, err := r.getInspectionXml(pod)
		if err != nil {
			return liberr.Wrap(err)
		}
		if vm.OperatingSystem, err = inspectionparser.GetOperationSystemFromConfig(inspectionXML); err != nil {
			return liberr.Wrap(err)
		}
		r.Log.Info("Setting the vm OS ", vm.OperatingSystem, "vmId", vm.ID)
	}

	if bootDiskIndex, bootOrderErr := util.GetDiskBootOrderFromYaml(vmConf); bootOrderErr != nil {
		r.Log.Error(bootOrderErr, "Failed to extract boot order from virt-v2v output", "vmId", vm.ID)
	} else if bootDiskIndex >= 0 {
		vm.DetectedBootDisk = &bootDiskIndex
		r.Log.Info("Detected boot disk from virt-v2v output", "bootDiskIndex", bootDiskIndex, "vmId", vm.ID)
	}

	// Fetch warnings before shutting down
	warningsURL := fmt.Sprintf("http://%s:8080/warnings", pod.Status.PodIP)
	if resp, err = http.Get(warningsURL); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var body []byte
			if resp.Body != nil {
				if data, err := io.ReadAll(resp.Body); err == nil {
					body = data
				}
			}

			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				r.Log.Info("contentType=%s, expect application/json", contentType)
			}

			var warnings []struct {
				Reason  string `json:"reason"`
				Message string `json:"message"`
			}
			if err = json.Unmarshal(body, &warnings); err == nil {
				for _, warning := range warnings {
					vm.SetCondition(libcnd.Condition{
						Type:     ConversionHasWarnings,
						Status:   True,
						Category: "Warning",
						Reason:   warning.Reason,
						Message:  warning.Message,
					})
					r.Log.Info("Conversion warning detected", "reason", warning.Reason, "vmId", vm.ID)
				}
			}
		}
	}

	shutdownURL := fmt.Sprintf("http://%s:8080/shutdown", pod.Status.PodIP)
	resp, err = http.Post(shutdownURL, "application/json", nil)
	if err == nil {
		defer resp.Body.Close()
	} else {
		// This error indicates that the server was shut down
		if strings.Contains(err.Error(), "EOF") {
			err = nil
		}
	}
	step.MarkCompleted()
	step.Progress.Completed = step.Progress.Total
	return err
}

// Delete the PVC consumer pod on the destination cluster.
func (r *KubeVirt) DeletePVCConsumerPod(vm *plan.VMStatus) (err error) {
	list, err := r.GetPodsWithLabels(r.consumerLabels(vm.Ref, true))
	if err != nil {
		return err
	}
	r.Log.Info("Found PVC consumer pods to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		err = r.DeleteObject(&object, vm, "Deleted PVC consumer pod.", "pod")
		if err != nil {
			return err
		}
	}
	return
}

// Delete the inspection pod.
func (r *KubeVirt) DeletePreflightInspectionPod(vm *plan.VMStatus) (err error) {
	list, err := r.GetPodsWithLabels(r.inspectionLabels(vm.Ref))
	if err != nil {
		return liberr.Wrap(err)
	}
	r.Log.Info("Found preflight inspection pods to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		err := r.DeleteObject(&object, vm, "Deleted preflight inspection pod.", "pod")
		if err != nil {
			return err
		}
	}
	return
}

// DeleteDeepInspectionPods deletes any deep inspection pods for the given VM
// that live on the management cluster.
func (r *KubeVirt) DeleteDeepInspectionPods(vm *plan.VMStatus) error {
	matchLabels := map[string]string{
		convctx.LabelPlan:           string(r.Plan.UID),
		convctx.LabelVM:             vm.ID,
		convctx.LabelConversionType: string(api.DeepInspection),
	}
	list := &core.PodList{}
	if err := r.List(context.TODO(), list,
		client.InNamespace(r.Plan.Namespace),
		client.MatchingLabels(matchLabels),
	); err != nil {
		return liberr.Wrap(err)
	}
	r.Log.Info("Found deep inspection pods to delete.", "count", len(list.Items), "vm", vm.String())
	for i := range list.Items {
		pod := &list.Items[i]
		if err := r.Delete(context.TODO(), pod); err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		r.Log.Info("Deleted deep inspection pod.", "pod", pod.Name, "vm", vm.String())
	}
	return nil
}

// Delete the guest conversion pod on the destination cluster.
func (r *KubeVirt) DeleteGuestConversionPod(vm *plan.VMStatus) (err error) {
	list, err := r.GetPodsWithLabels(r.conversionLabels(vm.Ref, true))
	if err != nil {
		return liberr.Wrap(err)
	}
	r.Log.Info("Found guest conversion pods to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		err := r.DeleteObject(&object, vm, "Deleted guest conversion pod.", "pod")
		if err != nil {
			return err
		}
	}
	return
}

// Gets pods associated with the VM.
func (r *KubeVirt) GetPods(vm *plan.VMStatus) (pods *core.PodList, err error) {
	return r.GetPodsWithLabels(r.vmAllButMigrationLabels(vm.Ref))
}

// Gets pods associated with the VM.
func (r *KubeVirt) GetPodsWithLabels(podLabels map[string]string) (pods *core.PodList, err error) {
	pods = &core.PodList{}
	err = r.Destination.Client.List(
		context.TODO(),
		pods,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(podLabels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return nil, err
	}
	return
}

// Deletes an object from destination cluster associated with the VM.
func (r *KubeVirt) DeleteObject(object client.Object, vm *plan.VMStatus, message, objType string, options ...client.DeleteOption) (err error) {
	err = r.Destination.Client.Delete(context.TODO(), object, options...)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		} else {
			return liberr.Wrap(err)
		}
	} else {
		r.Log.Info(
			message,
			objType,
			path.Join(
				object.GetNamespace(),
				object.GetName()),
			"vm",
			vm.String())
	}
	return
}

// Delete any hook jobs that belong to a VM migration.
func (r *KubeVirt) DeleteHookJobs(vm *plan.VMStatus) (err error) {
	// Build labels that match hook jobs (plan + vmID + resource:hook-config)
	labels := map[string]string{
		kPlan:     string(r.Plan.UID),
		kVM:       vm.Ref.ID,
		kResource: ResourceHookConfig,
	}

	list := &batch.JobList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labels),
			Namespace:     r.Plan.Namespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info("Found hook jobs to delete.", "count", len(list.Items), "vm", vm.String())
	for _, object := range list.Items {
		err = r.DeleteObject(&object, vm, "Deleted hook job.", "job",
			client.PropagationPolicy(meta.DeletePropagationForeground))
		if err != nil {
			return err
		}
	}
	return
}

// Deletes PVCs that were populated using a volume populator, including prime PVCs
func (r *KubeVirt) DeletePopulatedPVCs(vm *plan.VMStatus) error {
	pvcs, err := r.getPVCs(vm.Ref)
	if err != nil {
		return err
	}
	r.Log.Info("Found populated PVCs to delete.", "count", len(pvcs), "vm", vm.String())
	for _, pvc := range pvcs {
		if err = r.deleteCorrespondingPrimePVC(pvc, vm); err != nil {
			return err
		}
		if err = r.deletePopulatedPVC(pvc, vm); err != nil {
			return err
		}
	}
	return nil
}

func (r *KubeVirt) deleteCorrespondingPrimePVC(pvc *core.PersistentVolumeClaim, vm *plan.VMStatus) error {
	primePVC := core.PersistentVolumeClaim{}
	err := r.Destination.Client.Get(context.TODO(), client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: fmt.Sprintf("prime-%s", string(pvc.UID))}, &primePVC)
	switch {
	case err != nil && !k8serr.IsNotFound(err):
		return err
	case err == nil:
		err = r.DeleteObject(&primePVC, vm, "Deleted prime PVC.", "pvc")
		if err != nil && !k8serr.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *KubeVirt) deletePopulatedPVC(pvc *core.PersistentVolumeClaim, vm *plan.VMStatus) error {
	err := r.DeleteObject(pvc, vm, "Deleted PVC.", "pvc")
	switch {
	case err != nil && !k8serr.IsNotFound(err):
		return err
	case err == nil:
		pvcCopy := pvc.DeepCopy()
		pvc.Finalizers = nil
		patch := client.MergeFrom(pvcCopy)
		if err = r.Destination.Client.Patch(context.TODO(), pvc, patch); err != nil {
			return err
		}
	}
	return nil
}

// Delete any populator pods that belong to a VM's migration.
func (r *KubeVirt) DeletePopulatorPods(vm *plan.VMStatus) (err error) {
	if Settings.RetainPopulatorPods {
		if r.Plan.Spec.DeleteVmOnFailMigration || vm.DeleteVmOnFailMigration {
			r.Log.Info(
				"WARNING: FEATURE_RETAIN_POPULATOR_PODS is enabled but DeleteVmOnFailMigration is also set;"+
					" on failure the PVCs will be deleted and Kubernetes GC will remove the populator pods via OwnerReference.",
				"vm", vm.String())
		}
		r.Log.Info("Retaining populator pods (feature flag enabled).", "vm", vm.String())
		return
	}
	list, err := r.getPopulatorPods(vm.ID)
	if err != nil {
		return
	}
	r.Log.Info("Found populator pods to delete.", "count", len(list), "vm", vm.String())
	for _, object := range list {
		err = r.DeleteObject(&object, vm, "Deleted populator pod.", "pod")
	}
	return
}

// Get populator pods that belong to a specific VM in a migration.
func (r *KubeVirt) getPopulatorPods(vmID string) (pods []core.Pod, err error) {
	labelSelector := map[string]string{kMigration: string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)}
	if vmID != "" {
		labelSelector[kVM] = vmID
	}
	migrationPods, err := r.GetPodsWithLabels(labelSelector)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for _, pod := range migrationPods.Items {
		if strings.HasPrefix(pod.Name, PopulatorPodPrefix) {
			pods = append(pods, pod)
		}
	}
	return
}

// Build the DataVolume CRs.
func (r *KubeVirt) dataVolumes(vm *plan.VMStatus, secret *core.Secret, configMap *core.ConfigMap, vddkConfigMap *core.ConfigMap) (dataVolumes []cdi.DataVolume, err error) {
	_, err = r.Source.Inventory.VM(&vm.Ref)
	if err != nil {
		return
	}

	annotations := r.vmLabels(vm.Ref)
	if Settings.RetainPrecopyImporterPods {
		annotations[planbase.AnnRetainAfterCompletion] = "true"
	}
	if r.Plan.Spec.TransferNetwork != nil {
		err = r.setTransferNetwork(annotations)
		if err != nil {
			return
		}
	}

	if r.Plan.IsWarm() {
		if r.Builder.SupportsVolumePopulators() {
			// For storage offload, tie DataVolume to pre-imported PVC
			annotations[planbase.AnnAllowClaimAdoption] = "true"
			annotations[planbase.AnnPrePopulated] = "true"
		} else {
			// For warm migrations that use traditional  (ImageIO, VDDK) import sources (not populators),
			// explicitly disable CDI's populator auto-detection to avoid webhook validation errors
			annotations[planbase.AnnUsePopulator] = "false"
		}
	}

	if r.Plan.IsWarm() || !r.Destination.Provider.IsHost() || r.Plan.IsSourceProviderOCP() {
		// Set annotation for WFFC storage classes. Note that we create data volumes while
		// running a cold migration to the local cluster only when the source is either OpenShift
		// or vSphere, and in the latter case the conversion pod acts as the first-consumer
		annotations[planbase.AnnBindImmediate] = "true"
	}

	if sa := resolveServiceAccount(r.Plan); sa != "" {
		annotations[AnnCDIPodServiceAccount] = sa
	}

	// Do not delete the DV when the import completes as we check the DV to get the current
	// disk transfer status.
	annotations[AnnDeleteAfterCompletion] = "false"
	dvTemplate := cdi.DataVolume{
		ObjectMeta: meta.ObjectMeta{
			Namespace:   r.Plan.Spec.TargetNamespace,
			Annotations: annotations,
		},
	}
	if !(r.Builder.SupportsVolumePopulators() && r.Plan.IsWarm()) {
		// For storage offload warm migrations, the template should have already
		// been applied to the PVC that will be adopted by this DataVolume, so
		// only add generateName for other migration types.
		dvTemplate.ObjectMeta.GenerateName = r.getGeneratedName(vm)
	}
	dvTemplate.Labels = r.vmLabels(vm.Ref)

	dataVolumes, err = r.Builder.DataVolumes(vm.Ref, secret, configMap, &dvTemplate, vddkConfigMap)
	if err != nil {
		return
	}

	err = r.createLunDisks(vm.Ref)

	return
}

// Return the generated name for a specific VM and plan.
func (r *KubeVirt) getGeneratedName(vm *plan.VMStatus) string {
	return strings.Join(
		[]string{
			r.Plan.Name,
			vm.ID,
		},
		"-") + "-"
}

// Return the generated name for a specific VM and plan.
// If the VM name is incompatible with DNS1123 RFC, use the new name,
// otherwise use the original name.
func (r *KubeVirt) getNewVMName(vm *plan.VMStatus) string {
	if vm.NewName != "" {
		r.Log.Info("VM name is incompatible with DNS1123 RFC, renaming",
			"originalName", vm.Name, "newName", vm.NewName)
		return vm.NewName
	}

	return vm.Name
}

// Build the Kubevirt VM CR.
func (r *KubeVirt) virtualMachine(vm *plan.VMStatus, sortVolumesByLibvirt bool) (object *cnv.VirtualMachine, err error) {
	pvcs, err := r.getPVCs(vm.Ref)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	var ok bool
	object, err = r.vmPreference(vm)
	if err != nil {
		r.Log.Info("Building VirtualMachine without a VirtualMachinePreference.",
			"vm",
			vm.String(),
			"err",
			err)
		object, ok = r.vmTemplate(vm)
		if !ok {
			r.Log.Info("Building VirtualMachine without template.",
				"vm",
				vm.String())
			object = r.emptyVm(vm)
		}
	}

	err = r.setInstanceType(vm, object)
	if err != nil {
		return
	}

	if object.Spec.Template.ObjectMeta.Labels == nil {
		object.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}

	// Set the custom labels for the VM if specified in the plan
	if len(r.Plan.Spec.TargetLabels) > 0 {
		maps.Copy(object.Spec.Template.ObjectMeta.Labels, r.Plan.Spec.TargetLabels)
	}

	// Set the target node name if specified in the plan
	if len(r.Plan.Spec.TargetNodeSelector) > 0 {
		// If the node selector is not set, set it to an empty map
		if object.Spec.Template.Spec.NodeSelector == nil {
			object.Spec.Template.Spec.NodeSelector = make(map[string]string)
		}
		maps.Copy(object.Spec.Template.Spec.NodeSelector, r.Plan.Spec.TargetNodeSelector)
	}

	// Set the target affinity if specified in the plan
	if r.Plan.Spec.TargetAffinity != nil {
		object.Spec.Template.Spec.Affinity = r.Plan.Spec.TargetAffinity
	}

	// Set the 'app' label for identification of the virtual machine instance(s)
	object.Spec.Template.ObjectMeta.Labels["app"] = r.getNewVMName(vm)

	err = r.setVmLabels(object)
	if err != nil {
		return
	}

	// Add the original name and ID info to the VM annotations
	if len(vm.NewName) > 0 {
		if object.ObjectMeta.Annotations == nil {
			object.ObjectMeta.Annotations = make(map[string]string)
		}
		object.ObjectMeta.Annotations[AnnDisplayName] = vm.Name
		object.ObjectMeta.Annotations[AnnOriginalID] = vm.ID
	}

	sourceLabels, sourceAnnotations, sanitizationReport, tagErr := r.Builder.SourceVMLabelsAndAnnotations(vm.Ref, r.Plan.Spec.TagMapping)
	if tagErr != nil {
		r.Log.Error(tagErr, "Failed to get source VM labels/annotations", "vm", vm.String())
	} else {
		if object.ObjectMeta.Labels == nil {
			object.ObjectMeta.Labels = make(map[string]string)
		}
		maps.Copy(object.ObjectMeta.Labels, sourceLabels)
		if object.ObjectMeta.Annotations == nil {
			object.ObjectMeta.Annotations = make(map[string]string)
		}
		maps.Copy(object.ObjectMeta.Annotations, sourceAnnotations)
		if len(sanitizationReport) > 0 {
			reportJSON, jsonErr := json.Marshal(sanitizationReport)
			if jsonErr != nil {
				r.Log.Error(jsonErr, "Failed to marshal sanitization report",
					"vm", object.Name,
					"namespace", object.Namespace,
					"annotation", planbase.AnnSanitizedMetadata)
			} else {
				object.ObjectMeta.Annotations[planbase.AnnSanitizedMetadata] = string(reportJSON)
			}
		}
	}

	// Assign the determined run strategy to the object
	runStrategy := r.determineRunStrategy(vm)
	object.Spec.RunStrategy = &runStrategy
	object.Spec.Running = nil // Ensure running is not set

	// Add kubevirt template annotations if they are missing
	kubevirtWorkloadAnn := []string{
		"vm.kubevirt.io/flavor",
		"vm.kubevirt.io/os",
		"vm.kubevirt.io/workload",
	}
	if object.Spec.Template.ObjectMeta.Annotations == nil {
		object.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	for _, ann := range kubevirtWorkloadAnn {
		if _, ok := object.Spec.Template.ObjectMeta.Annotations[ann]; !ok {
			object.Spec.Template.ObjectMeta.Annotations[ann] = ""
		}
	}

	var configmaps []core.ConfigMap
	configmaps, err = r.Builder.ConfigMaps(vm.Ref)
	if err != nil {
		return
	}
	err = r.Ensurer.SharedConfigMaps(vm, configmaps)
	if err != nil {
		return
	}

	var secrets []core.Secret
	secrets, err = r.Builder.Secrets(vm.Ref)
	if err != nil {
		return
	}
	err = r.Ensurer.SharedSecrets(vm, secrets)
	if err != nil {
		return
	}

	err = r.Builder.VirtualMachine(vm.Ref, &object.Spec, pvcs, vm.InstanceType != "", sortVolumesByLibvirt)
	if err != nil {
		return
	}

	return
}

// Attempt to find a suitable preference.
func (r *KubeVirt) vmPreference(vm *plan.VMStatus) (virtualMachine *cnv.VirtualMachine, err error) {
	config, err := r.getOsMapConfig(r.Source.Provider.Type())
	if err != nil {
		return
	}
	preferenceName, err := r.Builder.PreferenceName(vm.Ref, config)
	if err != nil {
		return
	}
	if preferenceName == "" {
		err = liberr.New("couldn't find a corresponding preference", "vm", vm)
		return
	}

	preferenceName, kind, err := r.getPreference(vm, preferenceName)
	if err != nil {
		return
	}

	virtualMachine = r.emptyVm(vm)
	virtualMachine.Spec.Preference = &cnv.PreferenceMatcher{Name: preferenceName, Kind: kind}
	return
}

func (r *KubeVirt) setInstanceType(vm *plan.VMStatus, object *cnv.VirtualMachine) (err error) {
	if vm.InstanceType == "" {
		return
	}
	kind, err := r.getInstanceType(vm, vm.InstanceType)
	if err != nil {
		return
	}
	object.Spec.Instancetype = &cnv.InstancetypeMatcher{Name: vm.InstanceType, Kind: kind}
	return
}

func (r *KubeVirt) setVmLabels(object *cnv.VirtualMachine) (err error) {
	if object.ObjectMeta.Labels == nil {
		object.ObjectMeta.Labels = make(map[string]string)
	}
	if r.Plan.Provider.Source.RequiresConversion() {
		object.ObjectMeta.Labels["guestConverted"] = strconv.FormatBool(!r.Plan.Spec.SkipGuestConversion)
	}
	return
}

// Attempt to find a suitable instance type
func (r *KubeVirt) getInstanceType(vm *plan.VMStatus, instanceTypeName string) (kind string, err error) {
	kind, err = r.getVirtualMachineInstanceType(instanceTypeName)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("could not find a namespaced instance type for destination VM. trying cluster wide",
				"vm",
				vm.String())
		} else {
			r.Log.Error(err, "could not fetch a namespaced instance type for destination VM. trying cluster wide",
				"vm",
				vm.String())
		}
		kind, err = r.getVirtualMachineClusterInstanceType(vm, instanceTypeName)
	}

	return
}

func (r *KubeVirt) getVirtualMachineInstanceType(instanceTypeName string) (kind string, err error) {
	virtualMachineInstancetype := &instancetype.VirtualMachineInstancetype{}
	err = r.Destination.Client.Get(
		context.TODO(),
		client.ObjectKey{Name: instanceTypeName, Namespace: r.Plan.Spec.TargetNamespace},
		virtualMachineInstancetype)
	if err != nil {
		return
	}

	return instancetypeapi.SingularResourceName, nil
}

func (r *KubeVirt) getVirtualMachineClusterInstanceType(vm *plan.VMStatus, instanceTypeName string) (kind string, err error) {
	virtualMachineClusterInstancetype := &instancetype.VirtualMachineClusterInstancetype{}
	err = r.Destination.Client.Get(
		context.TODO(),
		client.ObjectKey{Name: instanceTypeName},
		virtualMachineClusterInstancetype)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("could not find instance type for destination VM.",
				"vm",
				vm.String(),
				"error",
				err)
		}
		return
	}
	return instancetypeapi.ClusterSingularResourceName, nil
}

func (r *KubeVirt) getOsMapConfig(providerType api.ProviderType) (configMap *core.ConfigMap, err error) {
	configMap = &core.ConfigMap{}
	var configMapName string
	switch providerType {
	case api.VSphere:
		configMapName = Settings.VsphereOsConfigMap
	case api.OVirt:
		configMapName = Settings.OvirtOsConfigMap
	default:
		return
	}
	err = r.Client.Get(
		context.TODO(),
		client.ObjectKey{Name: configMapName, Namespace: os.Getenv("POD_NAMESPACE")},
		configMap,
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *KubeVirt) getPreference(vm *plan.VMStatus, preferenceName string) (name, kind string, err error) {
	name, kind, err = r.getVirtualMachinePreference(preferenceName)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("could not find a local instance type preference for destination VM. trying cluster wide",
				"vm",
				vm.String())
		} else {
			r.Log.Error(err, "could not fetch a local instance type preference for destination VM. trying cluster wide",
				"vm",
				vm.String())
		}
		name, kind, err = r.getVirtualMachineClusterPreference(vm, preferenceName)
	}
	return
}

func (r *KubeVirt) getVirtualMachinePreference(preferenceName string) (name, kind string, err error) {
	virtualMachinePreference := &instancetype.VirtualMachinePreference{}
	err = r.Destination.Client.Get(
		context.TODO(),
		client.ObjectKey{Name: preferenceName, Namespace: r.Plan.Spec.TargetNamespace},
		virtualMachinePreference)
	if err != nil {
		return
	}
	return preferenceName, instancetypeapi.SingularPreferenceResourceName, nil
}

func (r *KubeVirt) getVirtualMachineClusterPreference(vm *plan.VMStatus, preferenceName string) (name, kind string, err error) {
	virtualMachineClusterPreference := &instancetype.VirtualMachineClusterPreference{}
	err = r.Destination.Client.Get(
		context.TODO(),
		client.ObjectKey{Name: preferenceName},
		virtualMachineClusterPreference)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("could not find instance type preference for destination VM.",
				"vm",
				vm.String(),
				"error",
				err)
		}
		return
	}
	return preferenceName, instancetypeapi.ClusterSingularPreferenceResourceName, nil
}

// Attempt to find a suitable template and extract a VirtualMachine definition from it.
func (r *KubeVirt) vmTemplate(vm *plan.VMStatus) (virtualMachine *cnv.VirtualMachine, ok bool) {
	if vm.InstanceType != "" {
		r.Log.Info("InstanceType is set, not setting a template", "vm", vm.String())
		return
	}
	tmpl, err := r.findTemplate(vm)
	if err != nil {
		r.Log.Info("Can't find matching template, not setting a template", "vm", vm.String())
		return
	}

	err = r.processTemplate(vm, tmpl)
	if err != nil {
		r.Log.Error(err,
			"Could not process Template for destination VM.",
			"vm",
			vm.String(),
			"template",
			tmpl.String())
		return
	}

	virtualMachine, err = r.decodeTemplate(tmpl)
	if err != nil {
		r.Log.Error(err,
			"Could not decode Template for destination VM.",
			"vm",
			vm.String(),
			"template",
			tmpl.String())
		return
	}

	vmLabels := r.vmLabels(vm.Ref)
	if virtualMachine.Labels != nil {
		for k, v := range vmLabels {
			virtualMachine.Labels[k] = v
		}
	} else {
		virtualMachine.Labels = vmLabels
	}

	// For OCP source
	if virtualMachine.Spec.Template == nil {
		virtualMachine.Spec.Template = &cnv.VirtualMachineInstanceTemplateSpec{}
	}

	virtualMachine.Name = r.getNewVMName(vm)
	virtualMachine.Namespace = r.Plan.Spec.TargetNamespace
	virtualMachine.Spec.Template.Spec.Volumes = []cnv.Volume{}
	virtualMachine.Spec.Template.Spec.Networks = []cnv.Network{}
	// Preserve DataVolumeTemplates for OCP source VMs to maintain user workflows
	// that may expect the VM's DataVolume to be present. The OCP builder will
	// set them from the source VM spec if they exist.
	if !r.Plan.IsSourceProviderOCP() {
		virtualMachine.Spec.DataVolumeTemplates = []cnv.DataVolumeTemplateSpec{}
	}
	delete(virtualMachine.Annotations, AnnKubevirtValidations)

	ok = true
	return
}

// Create empty VM definition.
func (r *KubeVirt) emptyVm(vm *plan.VMStatus) (virtualMachine *cnv.VirtualMachine) {
	virtualMachine = &cnv.VirtualMachine{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       util.VirtualMachineKind,
		},
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Spec.TargetNamespace,
			Labels:    r.vmLabels(vm.Ref),
			Name:      r.getNewVMName(vm),
		},
		Spec: cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{},
		},
	}
	return
}

// Decode the VirtualMachine object embedded in the template.
func (r *KubeVirt) decodeTemplate(tmpl *template.Template) (vm *cnv.VirtualMachine, err error) {
	if len(tmpl.Objects) == 0 {
		err = liberr.New("Could not find VirtualMachine in Template objects.")
		return
	}

	// Convert the RawExtension to a unstructured object
	var obj runtime.Object
	var scope conversion.Scope
	err = runtime.Convert_runtime_RawExtension_To_runtime_Object(&tmpl.Objects[0], &obj, scope)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	unstructured := obj.(runtime.Unstructured)

	// Convert the unstructured object into a VirtualMachine.
	vm = &cnv.VirtualMachine{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

// Process the template parameters.
func (r *KubeVirt) processTemplate(vm *plan.VMStatus, tmpl *template.Template) (err error) {
	source := rand.NewSource(time.Now().UTC().UnixNano())
	seed := rand.New(source)
	expr := generator.NewExpressionValueGenerator(seed)
	generators := map[string]generator.Generator{
		"expression": expr,
	}

	for i, param := range tmpl.Parameters {
		if param.Name == "NAME" {
			tmpl.Parameters[i].Value = vm.Name
		} else {
			tmpl.Parameters[i].Value = "other"
		}
	}

	processor := templateprocessing.NewProcessor(generators)
	errs := processor.Process(tmpl)
	if len(errs) > 0 {
		var msg []string
		for _, e := range errs {
			msg = append(msg, e.Error())
		}
		err = liberr.New(fmt.Sprintf("Failed to process template: %s", strings.Join(msg, ", ")))
	}

	return
}

// Attempt to find an OpenShift template that matches the VM's guest OS.
func (r *KubeVirt) findTemplate(vm *plan.VMStatus) (tmpl *template.Template, err error) {
	var templateLabels map[string]string
	templateLabels, err = r.Builder.TemplateLabels(vm.Ref)
	if err != nil {
		return
	}

	templateList := &template.TemplateList{}
	err = r.Destination.Client.List(
		context.TODO(),
		templateList,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(templateLabels),
			Namespace:     "openshift",
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(templateList.Items) == 0 {
		err = liberr.New("No matching templates found")
		return
	}

	if len(templateList.Items) > 1 {
		sort.Slice(templateList.Items, func(i, j int) bool {
			return templateList.Items[j].CreationTimestamp.Before(&templateList.Items[i].CreationTimestamp)
		})
	}
	tmpl = &templateList.Items[0]
	return
}

// getConvertorAffinity returns the affinity configuration for virt-v2v convertor pods.
// If ConvertorAffinity is specified in the plan, it uses that; otherwise, spread virt-v2v pods across nodes.
func (r *KubeVirt) getConvertorAffinity() *core.Affinity {
	// If custom convertor affinity is specified, use it
	if r.Plan.Spec.ConvertorAffinity != nil {
		return r.Plan.Spec.ConvertorAffinity.DeepCopy()
	}

	// Default pod anti-affinity behavior to spread virt-v2v pods across nodes
	return &core.Affinity{
		PodAntiAffinity: &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: core.PodAffinityTerm{
						NamespaceSelector: &meta.LabelSelector{},
						TopologyKey:       "kubernetes.io/hostname",
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key: kApp,
									Values: []string{
										"virt-v2v",
									},
									Operator: meta.LabelSelectorOpIn,
								},
							},
						},
					},
				},
			},
		},
	}
}

// netAppShiftDiskPermsInitContainer delegates to the shared builder function.
func netAppShiftDiskPermsInitContainer(mounts []core.VolumeMount, image string) core.Container {
	return convbuilder.NetAppShiftDiskPermsInitContainer(mounts, image)
}

// Build the inspection pod environment
func (r *KubeVirt) buildInspectionPodEnvironment(env []core.EnvVar, vm *plan.VMStatus, step *plan.Step) (newEnv []core.EnvVar, success bool, err error) {
	newEnv = append(env,
		core.EnvVar{
			Name:  "V2V_remoteInspection",
			Value: "true",
		})

	// Get VM model and data from inventory
	virtualMachine := &model.VM{}
	err = r.Context.Source.Inventory.Find(virtualMachine, vm.Ref)
	if err != nil {
		err = liberr.Wrap(err, "vm", vm.Ref.String())
		return
	}

	var retries int
	var limitExceeded bool
	if step.Annotations == nil {
		step.Annotations = make(map[string]string)
	}
	retriesAnnotation := step.Annotations[ParentBackingRetriesAnnotation]
	if retriesAnnotation == "" {
		step.Annotations[ParentBackingRetriesAnnotation] = "1"
	} else {
		retries, err = strconv.Atoi(retriesAnnotation)
		if err != nil {
			return
		}
		limitExceeded = retries > settings.Settings.MaxParentBackingRetries
	}

	// Add disks to be inspected
	for i, disk := range virtualMachine.SortedDisksAsLibvirt() {
		// If parent disk is empty then fail with error message
		if disk.ParentFile != "" {
			newEnv = append(newEnv, core.EnvVar{
				Name:  fmt.Sprintf("V2V_remoteInspectDisk_%d", i),
				Value: disk.ParentFile,
			})
		} else if limitExceeded {
			// If retry limit was exceeded then collect all the failing disks and put them as a step errors
			errMsg := fmt.Sprintf("Parent disk of %s was not found. This is possibly an environment issue. Please investigate if a precopy snapshot has a parent backing.", disk.File)
			step.AddError(errMsg)
			err = liberr.New(errMsg)
			r.Log.Error(err, "Failed to get parent backing of VM disk.", "vm", vm.Ref.String())
		} else {
			// Retry on the next run and log the missing parent disk
			retries += 1
			step.Annotations[ParentBackingRetriesAnnotation] = strconv.Itoa(retries)
			errMsg := fmt.Sprintf("Parent disk of %s was not found, will retry on next attempt", disk.File)
			r.Log.Info(errMsg,
				"vm", vm.Ref.String())
			return
		}
	}
	if limitExceeded {
		return
	}
	return newEnv, true, nil
}

func (r *KubeVirt) podVolumeMounts(vmVolumes []cnv.Volume, vddkConfigmap *core.ConfigMap, pvcs []*core.PersistentVolumeClaim, vm *plan.VMStatus) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, extraVolumes []core.Volume, extraMounts []core.VolumeMount, err error) {
	pvcsByName := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range pvcs {
		pvcsByName[pvc.Name] = pvc
	}

	for i, v := range vmVolumes {
		pvc := pvcsByName[v.PersistentVolumeClaim.ClaimName]
		if pvc == nil {
			r.Log.V(1).Info(
				"Failed to find the PVC to the Volume for the pod volume mount",
				"volume",
				v.Name,
				"pvc",
				v.PersistentVolumeClaim.ClaimName)
			continue
		}
		vol := core.Volume{
			Name: pvc.Name,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  false,
				},
			},
		}
		volumes = append(volumes, vol)
		if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == core.PersistentVolumeBlock {
			devices = append(devices, core.VolumeDevice{
				Name:       pvc.Name,
				DevicePath: fmt.Sprintf("/dev/block%v", i),
			})
		} else {
			mounts = append(mounts, core.VolumeMount{
				Name:      pvc.Name,
				MountPath: fmt.Sprintf("/mnt/disks/disk%v", i),
			})
		}
	}

	extraConfigMapExists := len(Settings.Migration.VirtV2vExtraConfConfigMap) > 0
	if extraConfigMapExists {
		volumes = append(volumes, core.Volume{
			Name: ExtraV2vConf,
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: genExtraV2vConfConfigMapName(r.Plan),
					},
				},
			},
		})
	}
	if vddkConfigmap != nil {
		volumes = append(volumes, core.Volume{
			Name: VddkConf,
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: vddkConfigmap.Name,
					},
				},
			},
		})
	}

	switch r.Source.Provider.Type() {
	case api.Ova, api.HyperV:
		var pvc *core.PersistentVolumeClaim
		var volumeName, mountPath string

		if r.Source.Provider.Type() == api.Ova {
			// OVA: Static NFS PV/PVC
			pv := r.BuildPVForNFS(vm)
			pv, err = r.EnsurePVForNFS(pv)
			if err != nil {
				return
			}
			pvc = r.BuildPVCForNFS(pv, vm)
			volumeName = "store-pv"
			mountPath = "/ova"
		} else {
			// HyperV: Static SMB CSI PV/PVC
			pv := r.BuildPVForSMB(vm)
			pv, err = r.EnsurePVForSMB(pv)
			if err != nil {
				return
			}
			pvc = r.BuildPVCForSMB(pv, vm)
			volumeName = "hyperv-storage"
			mountPath = "/hyperv"
		}

		// Ensure PVC exists (common logic)
		pvc, err = r.EnsureProviderStoragePVC(pvc, r.Source.Provider.Type())
		if err != nil {
			return
		}

		// Mount provider storage (common pattern)
		providerVol := core.Volume{
			Name: volumeName,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		}
		providerMount := core.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		}
		volumes = append(volumes, providerVol)
		mounts = append(mounts, providerMount)
		extraVolumes = append(extraVolumes, providerVol)
		extraMounts = append(extraMounts, providerMount)
	case api.VSphere:
		mounts = append(mounts,
			core.VolumeMount{
				Name:      VddkVolumeName,
				MountPath: "/opt",
			},
		)
		if extraConfigMapExists {
			mounts = append(mounts,
				core.VolumeMount{
					Name:      ExtraV2vConf,
					MountPath: fmt.Sprintf("/mnt/%s", ExtraV2vConf),
				},
			)
		}
		if vddkConfigmap != nil {
			mounts = append(mounts,
				core.VolumeMount{
					Name:      VddkConf,
					MountPath: fmt.Sprintf("/mnt/%s", VddkConf),
				},
			)
		}
	}

	// Use plan-level ConfigMap if specified
	if r.Plan.Spec.CustomizationScripts != nil {
		configMapName := r.Plan.Spec.CustomizationScripts.Name
		configMapNamespace := r.Plan.Spec.CustomizationScripts.Namespace
		if configMapNamespace == "" {
			configMapNamespace = r.Plan.Namespace
		}

		// When the CM was copied to TargetNamespace, use the generated name
		volumeConfigMapName := configMapName
		if configMapNamespace != r.Plan.Spec.TargetNamespace {
			volumeConfigMapName = genCustomizationScriptsConfigMapName(r.Plan)
		}

		var exists bool
		_, exists, err = r.findConfigMapInNamespace(volumeConfigMapName, r.Plan.Spec.TargetNamespace)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		if !exists {
			err = liberr.New(
				fmt.Sprintf("CustomizationScripts ConfigMap %s not found in namespace %s",
					volumeConfigMapName, r.Plan.Spec.TargetNamespace))
			return
		}
		scriptsVol := core.Volume{
			Name: DynamicScriptsVolumeName,
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: volumeConfigMapName,
					},
				},
			},
		}
		scriptsMount := core.VolumeMount{
			Name:      DynamicScriptsVolumeName,
			MountPath: DynamicScriptsMountPath,
		}
		volumes = append(volumes, scriptsVol)
		mounts = append(mounts, scriptsMount)
		extraVolumes = append(extraVolumes, scriptsVol)
		extraMounts = append(extraMounts, scriptsMount)
	}

	// Temporary space for VDDK library
	volumes = append(volumes, core.Volume{
		Name: VddkVolumeName,
		VolumeSource: core.VolumeSource{
			EmptyDir: &core.EmptyDirVolumeSource{},
		},
	})
	if vm.LUKS.Name != "" {
		labels := r.vmLabels(vm.Ref)
		labels[kLUKS] = "true"
		var secret *core.Secret
		if secret, err = r.ensureSecret(vm.Ref, r.secretLUKS(vm.LUKS.Name, r.Plan.Namespace), labels); err != nil {
			err = liberr.Wrap(err)
			return
		}
		volumes = append(volumes, core.Volume{
			Name: "luks",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		})
		mounts = append(mounts,
			core.VolumeMount{
				Name:      "luks",
				MountPath: "/etc/luks",
				ReadOnly:  true,
			})
	}
	return
}

// DiskRefsFromPodVolumeMounts calls podVolumeMounts and converts the
// PVC-backed volumes into DiskRef entries for a Conversion CR.
func (r *KubeVirt) DiskRefsFromPodVolumeMounts(vmVolumes []cnv.Volume, pvcs []*core.PersistentVolumeClaim, vm *plan.VMStatus, podType int) (refs []api.DiskRef, err error) {
	volumes, mounts, devices, extraVols, _, err := r.podVolumeMounts(vmVolumes, nil, pvcs, vm)
	if err != nil {
		return
	}
	extraNames := make(map[string]bool, len(extraVols))
	for _, v := range extraVols {
		extraNames[v.Name] = true
	}
	var diskVolumes []core.Volume
	for _, v := range volumes {
		if !extraNames[v.Name] {
			diskVolumes = append(diskVolumes, v)
		}
	}
	return convbuilder.DiskRefsFromVolumes(diskVolumes, mounts, devices, pvcs), nil
}

func (r *KubeVirt) findConfigMapInNamespace(name string, namespace string) (configMap *core.ConfigMap, exists bool, err error) {
	configmap := &core.ConfigMap{}
	err = r.Destination.Client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: namespace, Name: name},
		configmap,
	)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return configmap, true, nil
}

// Ensure the config map exists on the destination.
func (r *KubeVirt) ensureConfigMap(vmRef ref.Ref) (configMap *core.ConfigMap, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		return
	}

	list := &core.ConfigMapList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vmLabels(vmRef)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		configMap = &list.Items[0]
	} else {
		configMap, err = r.configMap(vmRef)
		if err != nil {
			return
		}
		err = r.Destination.Client.Create(context.TODO(), configMap)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"ConfigMap created.",
			"configMap",
			path.Join(
				configMap.Namespace,
				configMap.Name),
			"vm",
			vmRef.String())
	}

	return
}

// Build the config map.
func (r *KubeVirt) configMap(vmRef ref.Ref) (object *core.ConfigMap, err error) {
	object = &core.ConfigMap{
		ObjectMeta: meta.ObjectMeta{
			Labels:    r.vmLabels(vmRef),
			Namespace: r.Plan.Spec.TargetNamespace,
			GenerateName: strings.Join(
				[]string{
					r.Plan.Name,
					vmRef.ID,
				},
				"-") + "-",
		},
		BinaryData: make(map[string][]byte),
	}
	err = r.Builder.ConfigMap(vmRef, r.Source.Secret, object)

	return
}

func (r *KubeVirt) copyDataFromProviderSecret(secret *core.Secret) error {
	secret.Data = r.Source.Secret.Data
	return nil
}

func (r *KubeVirt) secretDataSetterForCDI(vmRef ref.Ref) func(*core.Secret) error {
	return func(secret *core.Secret) error {
		return r.Builder.Secret(vmRef, r.Source.Secret, secret)
	}
}

func (r *KubeVirt) secretLUKS(name, namespace string) func(*core.Secret) error {
	return func(secret *core.Secret) error {
		sourceSecret := &core.Secret{}
		err := r.Client.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, sourceSecret)
		if err != nil {
			return err
		}
		secret.Data = sourceSecret.Data
		return nil
	}
}

// Ensure the credential secret for the data transfer exists on the destination.
func (r *KubeVirt) ensureSecret(vmRef ref.Ref, setSecretData func(*core.Secret) error, labels map[string]string) (secret *core.Secret, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		return
	}

	newSecret, err := r.secret(vmRef, setSecretData, labels)
	if err != nil {
		return
	}

	list := &core.SecretList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		secret = &list.Items[0]
		// Copy Data because Builder.Secret() puts credentials (accessKeyId, secretKey) there, not in StringData.
		secret.Data = newSecret.Data
		secret.StringData = newSecret.StringData
		err = r.Destination.Client.Update(context.TODO(), secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"Secret updated.",
			"secret",
			path.Join(
				secret.Namespace,
				secret.Name),
			"vm",
			vmRef.String())
	} else {
		secret = newSecret
		err = r.Destination.Client.Create(context.TODO(), secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"Secret created.",
			"secret",
			path.Join(
				secret.Namespace,
				secret.Name),
			"vm",
			vmRef.String())
	}

	return
}

// Build the credential secret for the data transfer (CDI importer / popoulator pod).
func (r *KubeVirt) secret(vmRef ref.Ref, setSecretData func(*core.Secret) error, labels map[string]string) (secret *core.Secret, err error) {
	secret = &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Labels:    labels,
			Namespace: r.Plan.Spec.TargetNamespace,
			GenerateName: strings.Join(
				[]string{
					r.Plan.Name,
					vmRef.ID,
				},
				"-") + "-",
		},
	}
	err = setSecretData(secret)
	return
}

// Labels for plan and migration.
func (r *KubeVirt) planLabels() map[string]string {
	return map[string]string{
		kMigration:     string(r.Migration.UID),
		kPlan:          string(r.Plan.GetUID()),
		kPlanName:      r.Plan.Name,
		kPlanNamespace: r.Plan.Namespace,
	}
}

// Label for a PVC consumer pod.
func (r *KubeVirt) consumerLabels(vmRef ref.Ref, filterOutMigrationLabel bool) (labels map[string]string) {
	if filterOutMigrationLabel {
		labels = r.vmAllButMigrationLabels(vmRef)
	} else {
		labels = r.vmLabels(vmRef)
	}
	labels[kApp] = "consumer"
	return
}

// Label for a conversion pod.
func (r *KubeVirt) conversionLabels(vmRef ref.Ref, filterOutMigrationLabel bool) (labels map[string]string) {
	if filterOutMigrationLabel {
		labels = r.vmAllButMigrationLabels(vmRef)
	} else {
		labels = r.vmLabels(vmRef)
	}
	labels[kApp] = "virt-v2v"
	return
}

// Labels for an inspection pod.
func (r *KubeVirt) inspectionLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.vmLabels(vmRef)
	labels[kApp] = "virt-v2v-inspection"
	return
}

func (r *KubeVirt) waitForRebootLabels(vmRef ref.Ref) map[string]string {
	labels := r.vmLabels(vmRef)
	labels[kApp] = "forklift-wait-for-reboot"
	labels[kResource] = ResourceWaitForReboot
	return labels
}

// Labels for a VM on a plan.
func (r *KubeVirt) vmLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.planLabels()
	labels[kVM] = vmRef.ID
	labels[kResource] = ResourceVMConfig
	return
}

// Labels for a vddk config
// We need to distinguish between the libvirt configmap which uses also the plan labels and the vddk configmap
func (r *KubeVirt) vddkLabels() (labels map[string]string) {
	labels = r.planLabels()
	labels[kUse] = VddkConf
	labels[kResource] = ResourceVDDKConfig
	return
}

// Labels for a VM on a plan without migration label.
func (r *KubeVirt) vmAllButMigrationLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.vmLabels(vmRef)
	delete(labels, kMigration)
	return
}

// guessTransferNetworkDefaultRoute determines the default gateway IP address for the transfer network
// by checking the NetworkAttachmentDefinition in the following priority order:
//
//  1. Checks the AnnForkliftNetworkRoute annotation on the NAD
//  2. Parses the NAD's spec.config JSON and looks for the default route (0.0.0.0/0 or ::/0)
//     in the ipam.routes array, extracting the gateway IP from the matching route entry
//
// Returns:
//   - route: The gateway IP address as a string (e.g., "192.168.1.1")
//   - found: true if a route was found, false otherwise
func (r *KubeVirt) guessTransferNetworkDefaultRoute(netAttachDef *k8snet.NetworkAttachmentDefinition) (route string, found bool) {
	// First, try to get the default route from the annotation.
	route, found = netAttachDef.Annotations[AnnForkliftNetworkRoute]
	if found {
		return route, true
	}

	// If the route annotation is not set, try to get the default route from the gw config value.
	// Parse the Config string which is a JSON string containing network configuration.
	if netAttachDef.Spec.Config != "" {
		var config CNINetworkConfig
		err := json.Unmarshal([]byte(netAttachDef.Spec.Config), &config)
		if err != nil {
			// If we can't parse the config, just return not found
			return "", false
		}

		// Look for the default route (0.0.0.0/0 or ::/0) in the routes
		for _, r := range config.IPAM.Routes {
			if r.Dst == "0.0.0.0/0" || r.Dst == "::/0" {
				return r.GW, true
			}
		}
	}

	return "", false
}

// setTransferNetwork configures the transfer network for the DataVolume's importer pod
// by setting appropriate annotations based on whether a default gateway route can be determined.
//
// Behavior:
//   - If a default gateway is found (via annotation or NAD config): Sets the
//     k8s.v1.cni.cncf.io/networks annotation with the gateway IP in default-route
//   - If route annotation is explicitly set to "none" (AnnForkliftRouteValueNone):
//     Sets k8s.v1.cni.cncf.io/networks annotation without default-route field.
//     Useful for example when only ESXi hosts are accessible via transfer network
//     but vCenter is not.
//   - If no route annotation exists and no gateway found in NAD config:
//     Falls back to setting the legacy
//     v1.multus-cni.io/default-network annotation with the NAD's namespaced name
//
// The default gateway is discovered by checking the NAD annotation and IPAM config
// (see guessTransferNetworkDefaultRoute for details).
//
// FIXME: the codepath using the multus annotation should be phased out.
func (r *KubeVirt) setTransferNetwork(annotations map[string]string) (err error) {
	key := client.ObjectKey{
		Namespace: r.Plan.Spec.TransferNetwork.Namespace,
		Name:      r.Plan.Spec.TransferNetwork.Name,
	}
	netAttachDef := &k8snet.NetworkAttachmentDefinition{}
	err = r.Get(context.TODO(), key, netAttachDef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	route, found := r.guessTransferNetworkDefaultRoute(netAttachDef)
	if found {
		nse := k8snet.NetworkSelectionElement{
			Namespace: key.Namespace,
			Name:      key.Name,
		}

		if route != AnnForkliftRouteValueNone {
			ip := net.ParseIP(route)
			if ip != nil {
				nse.GatewayRequest = []net.IP{ip}
			} else {
				err = liberr.New(
					"Transfer network default route is not a valid IP address.",
					"route", route)
				return
			}
		}

		transferNetwork, jErr := json.Marshal([]k8snet.NetworkSelectionElement{nse})
		if jErr != nil {
			err = liberr.Wrap(jErr)
			return
		}
		annotations[AnnTransferNetwork] = string(transferNetwork)
	} else {
		annotations[AnnLegacyTransferNetwork] = path.Join(key.Namespace, key.Name)
	}

	return
}

// Represents a CDI DataVolume, its associated PVC, and added behavior.
type ExtendedDataVolume struct {
	*cdi.DataVolume
	PVC *core.PersistentVolumeClaim
}

// Get conditions.
func (r *ExtendedDataVolume) Conditions() (cnd *libcnd.Conditions) {
	cnd = &libcnd.Conditions{}
	for _, c := range r.Status.Conditions {
		cnd.SetCondition(libcnd.Condition{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}

	return
}

// Convert the Status.Progress into a
// percentage (float).
func (r *ExtendedDataVolume) PercentComplete() (pct float64) {
	s := string(r.Status.Progress)
	if strings.HasSuffix(s, "%") {
		s = s[:len(s)-1]
		n, err := strconv.ParseFloat(s, 64)
		if err == nil {
			pct = n / 100
		}
	}

	return
}

// Represents Kubevirt VirtualMachine with associated DataVolumes.
type VirtualMachine struct {
	*cnv.VirtualMachine
	DataVolumes []ExtendedDataVolume
}

// Determine if `this` VirtualMachine is the
// owner of the CDI DataVolume.
func (r *VirtualMachine) Owner(dv *cdi.DataVolume) bool {
	for _, vol := range r.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil && vol.PersistentVolumeClaim.ClaimName == dv.Name {
			return true
		}
	}

	return false
}

// Get conditions.
func (r *VirtualMachine) Conditions() (cnd *libcnd.Conditions) {
	cnd = &libcnd.Conditions{}
	for _, c := range r.Status.Conditions {
		newCnd := libcnd.Condition{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		}
		cnd.SetCondition(newCnd)
	}

	return
}

// Create an OwnerReference from a VM.
func vmOwnerReference(vm *cnv.VirtualMachine) (ref meta.OwnerReference) {
	blockOwnerDeletion := true
	isController := false
	ref = meta.OwnerReference{
		APIVersion:         "kubevirt.io/v1",
		Kind:               util.VirtualMachineKind,
		Name:               vm.Name,
		UID:                vm.UID,
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
	return
}

func (r *KubeVirt) setPopulatorPodLabels(pod core.Pod, migrationId string) (err error) {
	podCopy := pod.DeepCopy()
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[kMigration] = migrationId
	pod.Labels[kPlan] = string(r.Plan.GetUID())
	patch := client.MergeFrom(podCopy)
	err = r.Destination.Client.Patch(context.TODO(), &pod, patch)
	return
}

// Ensure the PV exist on the destination.
func (r *KubeVirt) EnsurePersistentVolume(vmRef ref.Ref, persistentVolumes []core.PersistentVolume) (err error) {
	list := &core.PersistentVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.vmLabels(vmRef)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	for _, pv := range persistentVolumes {
		pvVolume := pv.Labels["volume"]
		exists := false
		for _, item := range list.Items {
			if val, ok := item.Labels["volume"]; ok && val == pvVolume {
				exists = true
				break
			}
		}

		if !exists {
			err = r.Destination.Client.Create(context.TODO(), &pv)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			r.Log.Info("Created PersistentVolume.",
				"pv",
				path.Join(
					pv.Namespace,
					pv.Name),
				"vm",
				vmRef.String())
		}
	}
	return
}

// getPvListByLabels is a generic helper function to get PVs by labels.
func getPvListByLabels(dClient client.Client, labels map[string]string) (pvs *core.PersistentVolumeList, found bool, err error) {
	pvs = &core.PersistentVolumeList{}
	err = dClient.List(
		context.TODO(),
		pvs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labels),
		},
	)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil, false, nil
		}
		err = liberr.Wrap(err)
		return
	}
	return
}

// getPvcListByLabels is a generic helper function to get PVCs by labels in a namespace.
func getPvcListByLabels(dClient client.Client, labels map[string]string, namespace string) (pvcs *core.PersistentVolumeClaimList, found bool, err error) {
	pvcs = &core.PersistentVolumeClaimList{}
	err = dClient.List(
		context.TODO(),
		pvcs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(labels),
			Namespace:     namespace,
		},
	)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil, false, nil
		}
		err = liberr.Wrap(err)
		return
	}
	return
}

func GetOvaPvListNfs(dClient client.Client, planID string) (pvs *core.PersistentVolumeList, found bool, err error) {
	labels := map[string]string{
		"plan": planID,
		"ova":  OvaPVLabel,
	}
	return getPvListByLabels(dClient, labels)
}

func GetOvaPvcListNfs(dClient client.Client, planID string, planNamespace string) (pvcs *core.PersistentVolumeClaimList, found bool, err error) {
	labels := map[string]string{
		"plan": planID,
		"ova":  OvaPVCLabel,
	}
	return getPvcListByLabels(dClient, labels, planNamespace)
}

// GetHyperVPvcListSmb returns HyperV SMB PVCs for a plan.
func GetHyperVPvcListSmb(dClient client.Client, planID string, planNamespace string) (pvcs *core.PersistentVolumeClaimList, found bool, err error) {
	labels := map[string]string{
		"plan":   planID,
		"hyperv": HyperVPVCLabel,
	}
	return getPvcListByLabels(dClient, labels, planNamespace)
}

// GetHyperVPvListSmb returns HyperV SMB PVs for a plan.
func GetHyperVPvListSmb(dClient client.Client, planID string) (pvs *core.PersistentVolumeList, found bool, err error) {
	labels := map[string]string{
		"plan":   planID,
		"hyperv": HyperVPVLabel,
	}
	return getPvListByLabels(dClient, labels)
}

func (r *KubeVirt) EnsurePVForNFS(pv *core.PersistentVolume) (out *core.PersistentVolume, err error) {
	list := &core.PersistentVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(pv.Labels),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		out = &list.Items[0]
	} else {
		err = r.Destination.Client.Create(context.TODO(), pv)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created NFS PV for virt-v2v pod.", "pv", pv.Name)
		out = pv
	}
	return
}

func (r *KubeVirt) BuildPVForNFS(vm *plan.VMStatus) (pv *core.PersistentVolume) {
	sourceProvider := r.Source.Provider
	splitted := strings.Split(sourceProvider.Spec.URL, ":")
	nfsServer := splitted[0]
	nfsPath := splitted[1]
	pvcNamePrefix := getEntityPrefixName("pv", r.Source.Provider.Name, r.Plan.Name)

	pv = &core.PersistentVolume{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: pvcNamePrefix,
			Labels:       r.nfsPVLabels(vm.ID),
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(PVSize),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			PersistentVolumeSource: core.PersistentVolumeSource{
				NFS: &core.NFSVolumeSource{
					Path:   nfsPath,
					Server: nfsServer,
				},
			},
		},
	}
	return
}

// EnsureProviderStoragePVC returns existing PVC if found by labels, creates if not found (OVA NFS or HyperV SMB).
func (r *KubeVirt) EnsureProviderStoragePVC(pvc *core.PersistentVolumeClaim, providerType api.ProviderType) (out *core.PersistentVolumeClaim, err error) {
	// Query k8s for existing PVC matching labels (plan, migration, vmID)
	list := &core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(pvc.Labels),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	// Reuse existing PVC if found
	if len(list.Items) > 0 {
		out = &list.Items[0]
	} else {
		// Create PVC in k8s (triggers CSI provisioning for SMB)
		err = r.Destination.Client.Create(context.TODO(), pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		storageType := "NFS"
		if providerType == api.HyperV {
			storageType = "SMB"
		}
		r.Log.Info(fmt.Sprintf("Created %s PVC for virt-v2v pod.", storageType), "pvc", path.Join(pvc.Namespace, pvc.Name))
		out = pvc
	}
	return
}

// EnsurePVCForNFS is deprecated, use EnsureProviderStoragePVC instead.
// Kept for backwards compatibility with existing code paths.
func (r *KubeVirt) EnsurePVCForNFS(pvc *core.PersistentVolumeClaim) (out *core.PersistentVolumeClaim, err error) {
	return r.EnsureProviderStoragePVC(pvc, api.Ova)
}

func (r *KubeVirt) BuildPVCForNFS(pv *core.PersistentVolume, vm *plan.VMStatus) (pvc *core.PersistentVolumeClaim) {
	sc := ""
	pvcNamePrefix := getEntityPrefixName("pvc", r.Source.Provider.Name, r.Plan.Name)
	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: pvcNamePrefix,
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       r.nfsPVCLabels(vm.ID),
		},
		Spec: core.PersistentVolumeClaimSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse(PVSize),
				},
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			VolumeName:       pv.Name,
			StorageClassName: &sc,
		},
	}
	return
}

func (r *KubeVirt) nfsPVLabels(vmID string) map[string]string {
	return map[string]string{
		"provider":     r.Plan.Provider.Source.Name,
		"app":          "forklift",
		"migration":    r.Migration.Name,
		"plan":         string(r.Plan.UID),
		kPlanName:      r.Plan.Name,
		kPlanNamespace: r.Plan.Namespace,
		"ova":          OvaPVLabel,
		kVM:            vmID,
	}
}

func (r *KubeVirt) nfsPVCLabels(vmID string) map[string]string {
	return map[string]string{
		"provider":     r.Plan.Provider.Source.Name,
		"app":          "forklift",
		"migration":    string(r.Migration.UID),
		"plan":         string(r.Plan.UID),
		kPlanName:      r.Plan.Name,
		kPlanNamespace: r.Plan.Namespace,
		"ova":          OvaPVCLabel,
		kVM:            vmID,
	}
}

func getEntityPrefixName(resourceType, providerName, planName string) string {
	return fmt.Sprintf("ova-store-%s-%s-%s-", resourceType, providerName, planName)
}

// BuildPVForSMB creates a static PV for HyperV using SMB CSI driver.
func (r *KubeVirt) BuildPVForSMB(vm *plan.VMStatus) (pv *core.PersistentVolume) {
	sourceProvider := r.Source.Provider
	smbUrl := hvutil.SMBUrl(r.Source.Secret)
	smbSource := ctrlutil.ParseSMBSource(smbUrl)
	pvNamePrefix := fmt.Sprintf("hyperv-store-pv-%s-%s-", r.Source.Provider.Name, r.Plan.Name)

	// Get secret reference from provider
	secretName := sourceProvider.Spec.Secret.Name
	secretNamespace := sourceProvider.Spec.Secret.Namespace

	pv = &core.PersistentVolume{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: pvNamePrefix,
			Labels:       r.smbPVLabels(vm.ID),
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(PVSize),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			PersistentVolumeSource: core.PersistentVolumeSource{
				CSI: &core.CSIPersistentVolumeSource{
					Driver:       SMBCSIDriver,
					VolumeHandle: fmt.Sprintf("hyperv-%s-%s-%s", r.Source.Provider.Name, r.Plan.Name, vm.ID),
					VolumeAttributes: map[string]string{
						"source": smbSource,
					},
					NodeStageSecretRef: &core.SecretReference{
						Name:      secretName,
						Namespace: secretNamespace,
					},
				},
			},
		},
	}
	return
}

// EnsurePVForSMB ensures the static PV exists for HyperV SMB.
func (r *KubeVirt) EnsurePVForSMB(pv *core.PersistentVolume) (out *core.PersistentVolume, err error) {
	list := &core.PersistentVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(pv.Labels),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(list.Items) > 0 {
		out = &list.Items[0]
	} else {
		err = r.Destination.Client.Create(context.TODO(), pv)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info("Created SMB PV for virt-v2v pod.", "pv", pv.Name)
		out = pv
	}
	return
}

// BuildPVCForSMB creates a PVC bound to a static SMB PV for HyperV.
func (r *KubeVirt) BuildPVCForSMB(pv *core.PersistentVolume, vm *plan.VMStatus) (pvc *core.PersistentVolumeClaim) {
	sc := ""
	pvcNamePrefix := fmt.Sprintf("hyperv-pvc-%s-%s-", r.Source.Provider.Name, r.Plan.Name)
	pvc = &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: pvcNamePrefix,
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       r.smbPVCLabels(vm.ID),
		},
		Spec: core.PersistentVolumeClaimSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse(PVSize),
				},
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			VolumeName:       pv.Name,
			StorageClassName: &sc,
		},
	}
	return
}

// smbPVLabels returns labels for HyperV SMB PV.
func (r *KubeVirt) smbPVLabels(vmID string) map[string]string {
	return map[string]string{
		"provider":     r.Plan.Provider.Source.Name,
		"app":          "forklift",
		"migration":    r.Migration.Name,
		"plan":         string(r.Plan.UID),
		kPlanName:      r.Plan.Name,
		kPlanNamespace: r.Plan.Namespace,
		"hyperv":       HyperVPVLabel,
		kVM:            vmID,
	}
}

// smbPVCLabels returns labels for HyperV SMB PVC.
func (r *KubeVirt) smbPVCLabels(vmID string) map[string]string {
	return map[string]string{
		"provider":     r.Plan.Provider.Source.Name,
		"app":          "forklift",
		"migration":    string(r.Migration.UID),
		"plan":         string(r.Plan.UID),
		kPlanName:      r.Plan.Name,
		kPlanNamespace: r.Plan.Namespace,
		"hyperv":       HyperVPVCLabel,
		kVM:            vmID,
	}
}

// Ensure the PV exist on the destination.
func (r *KubeVirt) EnsurePersistentVolumeClaim(vmRef ref.Ref, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
	list, err := r.getPVCs(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	for _, pvc := range persistentVolumeClaims {
		pvcVolume := pvc.Labels["volume"]
		exists := false
		for _, item := range list {
			if val, ok := item.Labels["volume"]; ok && val == pvcVolume {
				exists = true
				break
			}
		}

		if !exists {
			err = r.Destination.Client.Create(context.TODO(), &pvc)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			r.Log.Info("Created PersistentVolumeClaim.",
				"pvc",
				path.Join(
					pvc.Namespace,
					pvc.Name),
				"vmRef",
				vmRef.String())
		}
	}
	return
}

// Load host CRs.
func (r *KubeVirt) loadHosts() (hosts map[string]*api.Host, err error) {
	list := &api.HostList{}
	err = r.List(
		context.TODO(),
		list,
		&client.ListOptions{
			Namespace: r.Source.Provider.Namespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	hostMap := map[string]*api.Host{}
	for i := range list.Items {
		host := &list.Items[i]
		ref := host.Spec.Ref
		if !libref.Equals(&host.Spec.Provider, &r.Plan.Spec.Provider.Source) {
			continue
		}

		if !host.Status.HasCondition(libcnd.Ready) {
			continue
		}
		// it's not that great to have a vSphere-specific entity here but as we don't
		// intend to do the same for other providers, doing it here for simplicity
		m := &model.Host{}
		pErr := r.Source.Inventory.Find(m, ref)
		if pErr != nil {
			if errors.As(pErr, &web.NotFoundError{}) {
				continue
			} else {
				err = pErr
				return
			}
		}
		ref.ID = m.ID
		ref.Name = m.Name
		hostMap[ref.ID] = host
	}

	hosts = hostMap

	return
}

// IsCopyOffload is determined by PVC having the copy-offload label, which is
// set by the builder earlier in #PopulatorVolumes
// TODO rgolan - for now the check will be done if any PVC match in the migration - this is obviously coarse
// and should be per a disk's storage class, for example a disk from NFS or local doesn't support that
// (specifically referring to vmkfstools xcopy for RDM)
func (r *KubeVirt) IsCopyOffload(pvcs []*core.PersistentVolumeClaim) bool {
	for _, p := range pvcs {
		for a := range p.Annotations {
			if a == "copy-offload" {
				return true
			}
		}
	}
	return false
}

// determineRunStrategy determines the appropriate run strategy based on the target power state configuration
func (r *KubeVirt) determineRunStrategy(vm *plan.VMStatus) cnv.VirtualMachineRunStrategy {
	// Determine the target power state based on plan configuration
	targetPowerState := vm.TargetPowerState
	if targetPowerState == "" {
		targetPowerState = r.Plan.Spec.TargetPowerState
	}

	if settings.Settings.WindowsWaitForReboot &&
		targetPowerState != plan.TargetPowerStateOff {
		win, wErr := migbase.IsWindowsFromInventory(r.Source.Inventory, vm.Ref)
		if wErr != nil {
			r.Log.Error(wErr, "Windows inventory lookup failed; falling back to default run strategy.", "vm", vm.String())
		} else if win {
			return cnv.RunStrategyAlways
		}
	}

	switch targetPowerState {
	case plan.TargetPowerStateOn:
		// Force target VM to be powered on
		return cnv.RunStrategyAlways
	case plan.TargetPowerStateOff:
		// Force target VM to be powered off
		return cnv.RunStrategyHalted
	default:
		// Default behavior: match the source VM's power state
		if vm.RestorePowerState == plan.VMPowerStateOn {
			return cnv.RunStrategyAlways
		}
		return cnv.RunStrategyHalted
	}
}

func getVirtV2vImage(plan *api.Plan) string {
	cfg := convctx.PodConfigFromPlan(plan)
	return convctx.GetVirtV2vImage(&cfg)
}

// buildUDNAnnotation returns the YAML-encoded value for the
// k8s.ovn.org/open-default-ports annotation required for User Defined Networks.
func buildUDNAnnotation() (string, error) {
	ports := []OpenPort{
		{Protocol: "tcp", Port: 2112},
		{Protocol: "tcp", Port: 8080},
	}
	out, err := yaml.Marshal(ports)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
