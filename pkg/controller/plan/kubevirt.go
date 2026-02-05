package plan

import (
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
	"sort"
	"strconv"
	"strings"
	"time"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	inspectionparser "github.com/kubev2v/forklift/pkg/controller/plan/adapter/vsphere"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
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
	"gopkg.in/yaml.v2"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
	instancetypeapi "kubevirt.io/api/instancetype"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
)

// Labels
const (
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// VM label (value=vmID)
	kVM = "vmID"
	// VM UUID label
	kVmUuid = "vmUUID"
	// App label
	kApp = "forklift.app"
	// LUKS
	kLUKS = "isLUKS"
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
	ResourceVMConfig   = "vm-config"
	ResourceVDDKConfig = "vddk-config"
)

// Finalizers
const (
	// PopulatorPVCFinalizer is the finalizer added by the volume populator controller
	// to protect PVCs during population. Must be removed when archiving.
	PopulatorPVCFinalizer = "forklift.konveyor.io/populate-target-protection"
)

// User
const (
	// Qemu user
	qemuUser = int64(107)
	// Qemu group
	qemuGroup = int64(107)
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
	ExtraV2vConf = "extra-v2v-conf"
	VddkConf     = "vddk-conf"

	VddkAioBufSizeDefault  = "16"
	VddkAioBufCountDefault = "4"
)

// VirtV2V pod types
const (
	VirtV2vConversionPod = 0
	VirtV2vInspectionPod = 1
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
	annotations := r.vmLabels(vmRef)
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

func (r *KubeVirt) vddkConfigMap(labels map[string]string) (*core.ConfigMap, error) {
	data := make(map[string]string)
	if r.Source.Provider.UseVddkAioOptimization() {
		vddkConfig := r.Source.Provider.Spec.Settings[api.VddkConfig]
		if vddkConfig != "" {
			data["vddk-config-file"] = vddkConfig
		} else {
			data["vddk-config-file"] =
				"VixDiskLib.nfcAio.Session.BufSizeIn64KB=16\n" +
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
		labelSelector[kMigration] = r.ActiveMigrationUID()
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

	pvcs = make([]*core.PersistentVolumeClaim, len(pvcsList.Items))
	for i, pvc := range pvcsList.Items {
		// loopvar
		pvc := pvc
		pvcs[i] = &pvc
	}

	// Sort the pvcs slice by disk index
	sort.Slice(pvcs, func(i, j int) bool {
		iIdx := getDiskIndex(pvcs[i])
		jIdx := getDiskIndex(pvcs[j])
		return iIdx < jIdx
	})

	return
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
					Image:   Settings.Migration.VirtV2vImage,
					Command: []string{"/bin/sh"},
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
	// Align with the conversion pod request, to prevent breakage
	r.setKvmOnPodSpec(&pod.Spec)

	err = r.Client.Create(context.TODO(), pod, &client.CreateOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("Created pod '%s' to init the PVC node", pod.Name))
	return nil
}

// Sets KVM requirement to the pod and container.
func (r *KubeVirt) setKvmOnPodSpec(podSpec *core.PodSpec) {
	if Settings.VirtV2vDontRequestKVM {
		return
	}
	switch *r.Plan.Provider.Source.Spec.Type {
	case api.VSphere, api.Ova:
		if podSpec.NodeSelector == nil {
			podSpec.NodeSelector = make(map[string]string)
		}
		podSpec.NodeSelector["kubevirt.io/schedulable"] = "true"
		container := &podSpec.Containers[0]
		if container.Resources.Limits == nil {
			container.Resources.Limits = make(map[core.ResourceName]resource.Quantity)
		}
		container.Resources.Limits["devices.kubevirt.io/kvm"] = resource.MustParse("1")
		if container.Resources.Requests == nil {
			container.Resources.Requests = make(map[core.ResourceName]resource.Quantity)
		}
		// Ensure that the pod is deployed on a node where /dev/kvm is present.
		container.Resources.Requests["devices.kubevirt.io/kvm"] = resource.MustParse("1")
	}
}

func (r *KubeVirt) getListOptionsNamespaced() (listOptions *client.ListOptions) {
	return &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
	}
}

// Ensure the guest conversion/inspection (virt-v2v) pod exists on the destination.
func (r *KubeVirt) EnsureVirtV2vPod(vm *plan.VMStatus, vmCr *VirtualMachine, pvcs []*core.PersistentVolumeClaim, podType int, step *plan.Step) (err error) {
	labels := r.vmLabels(vm.Ref)
	labels[kV2V] = "true"
	v2vSecret, err := r.ensureSecret(vm.Ref, r.secretDataSetterForCDI(vm.Ref), labels)
	if err != nil {
		return
	}

	var vddkConfigMap *core.ConfigMap
	if r.Source.Provider.UseVddkAioOptimization() {
		vddkConfigMap, err = r.ensureVddkConfigMap()
		if err != nil {
			return err
		}
	}

	// vmVolumes is not used when creating inspection pod so it can be empty
	vmVolumes := []cnv.Volume{}
	if podType == VirtV2vConversionPod {
		vmVolumes = vmCr.Spec.Template.Spec.Volumes
	}
	newPod, err := r.getVirtV2vPod(vm, vmVolumes, vddkConfigMap, pvcs, v2vSecret, podType, step)
	if err != nil {
		return
	}
	if newPod == nil {
		r.Log.Info("Couldn't prepare virt-v2v pod for vm.", "vm", vm.String())
		return
	}

	var podTypeLabels = map[string]string{}
	switch podType {
	case VirtV2vConversionPod:
		podTypeLabels = r.conversionLabels(vm.Ref, true)
	case VirtV2vInspectionPod:
		podTypeLabels = r.inspectionLabels(vm.Ref)
	}
	list, err := r.GetPodsWithLabels(podTypeLabels)
	if err != nil {
		return
	}

	pod := &core.Pod{}
	if len(list.Items) == 0 {
		pod = newPod
		err = r.Destination.Client.Create(context.TODO(), pod)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created virt-v2v pod.",
			"pod",
			path.Join(
				pod.Namespace,
				pod.Name),
			"vm",
			vm.String())
	}

	return
}

// EnsureOVAVirtV2VPVCStatus checks if the provider storage PVC is ready.
// Works for both OVA (NFS) and HyperV (SMB) PVCs.
func (r *KubeVirt) EnsureOVAVirtV2VPVCStatus(vmID string) (ready bool, err error) {
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
		//we need the IP for fetching the configuration of the convered VM.
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

	switch r.Source.Provider.Type() {
	case api.Ova, api.HyperV:
		vmConf, err := io.ReadAll(resp.Body)
		if err != nil {
			return liberr.Wrap(err)
		}
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
	for _, object := range list.Items {
		err := r.DeleteObject(&object, vm, "Deleted preflight inspection pod.", "pod")
		if err != nil {
			return err
		}
	}
	return
}

// Delete the guest conversion pod on the destination cluster.
func (r *KubeVirt) DeleteGuestConversionPod(vm *plan.VMStatus) (err error) {
	list, err := r.GetPodsWithLabels(r.conversionLabels(vm.Ref, true))
	if err != nil {
		return liberr.Wrap(err)
	}
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
	err = r.Destination.Client.Delete(context.TODO(), object)
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
	for _, object := range list.Items {
		err = r.DeleteObject(&object, vm, "Deleted hook job.", "job")
		if err != nil {
			return err
		}
	}
	return
}

// Set the Populator Pod Ownership.
func (r *KubeVirt) SetPopulatorPodOwnership(vm *plan.VMStatus) (err error) {
	pvcs, err := r.getPVCs(vm.Ref)
	if err != nil {
		return
	}
	pods, err := r.getPopulatorPods()
	if err != nil {
		return
	}
	for _, pod := range pods {
		pvcId := pod.Name[len(PopulatorPodPrefix):]
		for _, pvc := range pvcs {
			if string(pvc.UID) != pvcId {
				continue
			}
			podCopy := pod.DeepCopy()
			err = k8sutil.SetOwnerReference(pvc, &pod, r.Scheme())
			if err != nil {
				continue
			}
			patch := client.MergeFrom(podCopy)
			err = r.Destination.Client.Patch(context.TODO(), &pod, patch)
			if err != nil {
				break
			}
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

// DeletePrimePVCs deletes only the prime-* PVCs for a VM (not the disk PVCs).
// Prime PVCs are temporary PVCs created by the volume populator controller and
// should be cleaned up after migration completes, even for successful VMs.
func (r *KubeVirt) DeletePrimePVCs(vm *plan.VMStatus) error {
	pvcs, err := r.getPVCs(vm.Ref)
	if err != nil {
		return err
	}
	for _, pvc := range pvcs {
		if err = r.deleteCorrespondingPrimePVC(pvc, vm); err != nil {
			r.Log.Error(err, "Failed to delete prime PVC.", "pvc", pvc.Name, "vm", vm.String())
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
		// Remove all finalizers to allow the PVC to be deleted
		pvcCopy := pvc.DeepCopy()
		pvc.Finalizers = nil
		patch := client.MergeFrom(pvcCopy)
		if err = r.Destination.Client.Patch(context.TODO(), pvc, patch); err != nil {
			return err
		}
	}
	return nil
}

// removePopulatorFinalizerFromPVC removes the populator finalizer from a PVC.
// This should be called when archiving a plan to allow users to delete preserved PVCs.
func (r *KubeVirt) removePopulatorFinalizerFromPVC(pvc *core.PersistentVolumeClaim) error {
	// Check if finalizer exists
	hasFinalizer := false
	for _, f := range pvc.Finalizers {
		if f == PopulatorPVCFinalizer {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		return nil
	}

	// Remove the finalizer
	pvcCopy := pvc.DeepCopy()
	var newFinalizers []string
	for _, f := range pvc.Finalizers {
		if f != PopulatorPVCFinalizer {
			newFinalizers = append(newFinalizers, f)
		}
	}
	pvc.Finalizers = newFinalizers
	patch := client.MergeFrom(pvcCopy)
	if err := r.Destination.Client.Patch(context.TODO(), pvc, patch); err != nil {
		r.Log.Error(err, "Failed to remove populator finalizer from PVC.", "pvc", pvc.Name)
		return err
	}
	return nil
}

// Delete any populator pods that belong to a VM's migration.
func (r *KubeVirt) DeletePopulatorPods(vm *plan.VMStatus) (err error) {
	list, err := r.getPopulatorPods()
	for _, object := range list {
		err = r.DeleteObject(&object, vm, "Deleted populator pod.", "pod")
	}
	return
}

// Get populator pods that belong to a VM's migration.
func (r *KubeVirt) getPopulatorPods() (pods []core.Pod, err error) {
	migrationUID := r.ActiveMigrationUID()
	migrationPods, err := r.GetPodsWithLabels(map[string]string{kMigration: migrationUID})
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
			vm.ID},
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

	//Add the original name and ID info to the VM annotations
	if len(vm.NewName) > 0 {
		annotations := make(map[string]string)
		annotations[AnnDisplayName] = vm.Name
		annotations[AnnOriginalID] = vm.ID
		object.ObjectMeta.Annotations = annotations
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
	labels := object.ObjectMeta.Labels
	if labels == nil {
		object.ObjectMeta.Labels = map[string]string{}
	}
	if r.Plan.Provider.Source.RequiresConversion() {
		labels["guestConverted"] = strconv.FormatBool(!r.Plan.Spec.SkipGuestConversion)
	}
	object.ObjectMeta.Labels = labels
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
	virtualMachine.Spec.DataVolumeTemplates = []cnv.DataVolumeTemplateSpec{}
	delete(virtualMachine.Annotations, AnnKubevirtValidations)

	ok = true
	return
}

// Create empty VM definition.
func (r *KubeVirt) emptyVm(vm *plan.VMStatus) (virtualMachine *cnv.VirtualMachine) {
	virtualMachine = &cnv.VirtualMachine{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "VirtualMachine",
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

func (r *KubeVirt) getVirtV2vPod(vm *plan.VMStatus, vmVolumes []cnv.Volume, vddkConfigmap *core.ConfigMap, pvcs []*core.PersistentVolumeClaim, v2vSecret *core.Secret, podType int, step *plan.Step) (pod *core.Pod, err error) {
	volumes, volumeMounts, volumeDevices, err := r.podVolumeMounts(vmVolumes, vddkConfigmap, pvcs, vm, podType)
	if err != nil {
		return
	}

	// pod environment
	environment, err := r.Builder.PodEnvironment(vm.Ref, r.Source.Secret)
	if err != nil {
		return
	}

	// qemu group
	fsGroup := qemuGroup
	user := qemuUser
	nonRoot := true
	allowPrivilageEscalation := false
	// virt-v2v image
	useV2vForTransfer, vErr := r.Context.Plan.ShouldUseV2vForTransfer()
	if vErr != nil {
		err = vErr
		return
	}
	volumes = append(volumes, core.Volume{
		Name: "secret-volume",
		VolumeSource: core.VolumeSource{
			Secret: &core.SecretVolumeSource{
				SecretName: v2vSecret.Name,
			},
		},
	})
	volumeMounts = append(volumeMounts, core.VolumeMount{
		Name:      "secret-volume",
		ReadOnly:  true,
		MountPath: "/etc/secret",
	})

	// Add temporary conversion storage if configured
	if r.Plan.Spec.ConversionTempStorageClass != "" && r.Plan.Spec.ConversionTempStorageSize != "" {
		// Use Generic Ephemeral Volume for temporary conversion storage
		// This creates a temporary PVC that is automatically deleted with the pod
		storageClass := r.Plan.Spec.ConversionTempStorageClass
		// VolumeMode must be Filesystem since we mount it at /var/tmp/virt-v2v for use as a filesystem.
		// Without this, Kubernetes may default to block mode which cannot be mounted as a filesystem.
		volumeMode := core.PersistentVolumeFilesystem
		volumes = append(volumes, core.Volume{
			Name: "conversion-temp-storage",
			VolumeSource: core.VolumeSource{
				Ephemeral: &core.EphemeralVolumeSource{
					VolumeClaimTemplate: &core.PersistentVolumeClaimTemplate{
						Spec: core.PersistentVolumeClaimSpec{
							AccessModes: []core.PersistentVolumeAccessMode{
								core.ReadWriteOnce,
							},
							StorageClassName: &storageClass,
							VolumeMode:       &volumeMode,
							Resources: core.VolumeResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse(r.Plan.Spec.ConversionTempStorageSize),
								},
							},
						},
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, core.VolumeMount{
			Name:      "conversion-temp-storage",
			MountPath: "/var/tmp/virt-v2v",
		})
		// Tell virt-v2v to use the custom scratch directory
		environment = append(environment,
			core.EnvVar{
				Name:  "TMPDIR",
				Value: "/var/tmp/virt-v2v",
			})
	}

	if !useV2vForTransfer || r.IsCopyOffload(pvcs) {
		environment = append(environment,
			core.EnvVar{
				Name:  "V2V_inPlace",
				Value: "1",
			})
	}
	// VDDK image
	var initContainers []core.Container

	vddkImage := settings.GetVDDKImage(r.Source.Provider.Spec.Settings)
	if vddkImage != "" {
		initContainers = append(initContainers, core.Container{
			Name:            "vddk-side-car",
			Image:           vddkImage,
			ImagePullPolicy: core.PullIfNotPresent,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      VddkVolumeName,
					MountPath: "/opt",
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
			SecurityContext: &core.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilageEscalation,
				Capabilities: &core.Capabilities{
					Drop: []core.Capability{"ALL"},
				},
			},
		})
	}
	if vm.RootDisk != "" {
		environment = append(environment,
			core.EnvVar{
				Name:  "V2V_RootDisk",
				Value: vm.RootDisk,
			})
	}

	if vm.NewName != "" {
		environment = append(environment,
			core.EnvVar{
				Name:  "V2V_NewName",
				Value: vm.NewName,
			})
	}

	environment = append(environment,
		core.EnvVar{
			Name:  "LOCAL_MIGRATION",
			Value: strconv.FormatBool(r.Destination.Provider.IsHost()),
		},
	)
	// pod annotations
	annotations := map[string]string{}
	if r.Plan.Spec.TransferNetwork != nil {
		err = r.setTransferNetwork(annotations)
		if err != nil {
			return
		}
	}
	if r.Plan.DestinationHasUdnNetwork(r.Destination) {
		metricsPort := OpenPort{Protocol: "tcp", Port: 2112}
		dataServerPort := OpenPort{Protocol: "tcp", Port: 8080}
		ports := []OpenPort{metricsPort, dataServerPort}
		var yamlPorts []byte
		yamlPorts, err = yaml.Marshal(ports)
		if err != nil {
			return
		}
		/*
		   For the User Defined Networks we need to open some port so we can communicate with our metrics server inside the User Defined Network Namespace.
		   Docs: https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/multiple_networks/primary-networks#opening-default-network-ports-udn_about-user-defined-networks
		*/
		annotations[planbase.AnnOpenDefaultPorts] = string(yamlPorts)
	}
	var seccompProfile core.SeccompProfile
	if settings.Settings.OpenShift {
		unshare := "profiles/unshare.json"
		seccompProfile = core.SeccompProfile{
			Type:             core.SeccompProfileTypeLocalhost,
			LocalhostProfile: &unshare,
		}
	} else {
		seccompProfile = core.SeccompProfile{
			Type: core.SeccompProfileTypeRuntimeDefault,
		}
	}

	// Get provider-specific conversion pod configuration
	providerConfig, err := r.Builder.ConversionPodConfig(vm.Ref)
	if err != nil {
		return nil, err
	}

	var podName string
	var containerName string
	// pod labels - merge order: provider config -> user labels -> system labels (system overrides all)
	podLabels := make(map[string]string)
	if providerConfig.Labels != nil {
		maps.Copy(podLabels, providerConfig.Labels)
	}
	switch podType {
	case VirtV2vConversionPod:
		if r.Plan.Spec.ConvertorLabels != nil {
			maps.Copy(podLabels, r.Plan.Spec.ConvertorLabels)
		}
		// System conversion labels override user labels
		maps.Copy(podLabels, r.conversionLabels(vm.Ref, false))
		podName = r.getGeneratedName(vm)
		containerName = "virt-v2v"
	case VirtV2vInspectionPod:
		maps.Copy(podLabels, r.inspectionLabels(vm.Ref))
		// Add inspection pod specific settings
		podName = r.getGeneratedName(vm) + "inspection-"
		containerName = "virt-v2v-inspection"

		var success bool
		environment, success, err = r.buildInspectionPodEnvironment(environment, vm, step)
		if err != nil {
			return nil, err
		}
		if !success {
			// This is intentional and it means that pod was not created and no error occured (yet), e.g. retry
			return nil, nil //nolint:nilnil
		}
	}

	// pod annotations - merge provider config after system annotations
	if providerConfig.Annotations != nil {
		maps.Copy(annotations, providerConfig.Annotations)
	}

	// pod node selector - merge provider config with user settings (user takes precedence)
	var podNodeSelector map[string]string
	if providerConfig.NodeSelector != nil {
		podNodeSelector = make(map[string]string)
		maps.Copy(podNodeSelector, providerConfig.NodeSelector)
	}
	if r.Plan.Spec.ConvertorNodeSelector != nil {
		if podNodeSelector == nil {
			podNodeSelector = make(map[string]string)
		}
		maps.Copy(podNodeSelector, r.Plan.Spec.ConvertorNodeSelector)
	}

	// pod
	pod = &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Spec.TargetNamespace,
			Annotations:  annotations,
			Labels:       podLabels,
			GenerateName: podName,
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				FSGroup:        &fsGroup,
				RunAsUser:      &user,
				RunAsNonRoot:   &nonRoot,
				SeccompProfile: &seccompProfile,
			},
			NodeSelector:   podNodeSelector,
			Affinity:       r.getConvertorAffinity(),
			RestartPolicy:  core.RestartPolicyNever,
			InitContainers: initContainers,
			Containers: []core.Container{
				{
					Name:            containerName,
					Env:             environment,
					ImagePullPolicy: core.PullAlways,
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
					EnvFrom: []core.EnvFromSource{
						{
							Prefix: "V2V_",
							SecretRef: &core.SecretEnvSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: v2vSecret.Name,
								},
							},
						},
					},
					Image:         Settings.Migration.VirtV2vImage,
					VolumeMounts:  volumeMounts,
					VolumeDevices: volumeDevices,
					Ports: []core.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 2112,
							Protocol:      core.ProtocolTCP,
						},
					},
					SecurityContext: &core.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilageEscalation,
						Capabilities: &core.Capabilities{
							Drop: []core.Capability{"ALL"},
						},
					},
				},
			},
			Volumes: volumes,
		},
	}
	// Request access to /dev/kvm via Kubevirt's Device Manager
	// That is to ensure the appliance virt-v2v uses would not
	// run in emulation mode, which is significantly slower
	r.setKvmOnPodSpec(&pod.Spec)

	return
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
	for i, disk := range virtualMachine.Disks {
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

func (r *KubeVirt) podVolumeMounts(vmVolumes []cnv.Volume, vddkConfigmap *core.ConfigMap, pvcs []*core.PersistentVolumeClaim, vm *plan.VMStatus, podType int) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, err error) {
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
		volumes = append(volumes, core.Volume{
			Name: volumeName,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		})
		mounts = append(mounts, core.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
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
			configMapNamespace = r.Plan.Spec.TargetNamespace
		}

		var exists bool
		_, exists, err = r.findConfigMapInNamespace(configMapName, configMapNamespace)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		if !exists {
			err = liberr.New(
				fmt.Sprintf("CustomizationScripts ConfigMap %s not found in namespace %s",
					configMapName, configMapNamespace))
			return
		}
		volumes = append(volumes, core.Volume{
			Name: DynamicScriptsVolumeName,
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		})
		mounts = append(mounts, core.VolumeMount{
			Name:      DynamicScriptsVolumeName,
			MountPath: DynamicScriptsMountPath,
		})
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
					vmRef.ID},
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
					vmRef.ID},
				"-") + "-",
		},
	}
	err = setSecretData(secret)
	return
}

// Labels for plan and migration.
func (r *KubeVirt) planLabels() map[string]string {
	return map[string]string{
		kMigration: string(r.Migration.UID),
		kPlan:      string(r.Plan.GetUID()),
	}
}

// Labels for plan only (no migration or VM).
func (r *KubeVirt) planOnlyLabels() map[string]string {
	return map[string]string{
		kPlan: string(r.Plan.GetUID()),
	}
}

// Labels for a specific migration.
func (r *KubeVirt) migrationOnlyLabels(migrationUID string) map[string]string {
	return map[string]string{
		kPlan:      string(r.Plan.GetUID()),
		kMigration: migrationUID,
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
		Kind:               "VirtualMachine",
		Name:               vm.Name,
		UID:                vm.UID,
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
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
		"provider":  r.Plan.Provider.Source.Name,
		"app":       "forklift",
		"migration": r.Migration.Name,
		"plan":      string(r.Plan.UID),
		"ova":       OvaPVLabel,
		kVM:         vmID,
	}
}

func (r *KubeVirt) nfsPVCLabels(vmID string) map[string]string {
	return map[string]string{
		"provider":  r.Plan.Provider.Source.Name,
		"app":       "forklift",
		"migration": string(r.Migration.UID),
		"plan":      string(r.Plan.UID),
		"ova":       OvaPVCLabel,
		kVM:         vmID,
	}
}

func getEntityPrefixName(resourceType, providerName, planName string) string {
	return fmt.Sprintf("ova-store-%s-%s-%s-", resourceType, providerName, planName)
}

// BuildPVForSMB creates a static PV for HyperV using SMB CSI driver.
func (r *KubeVirt) BuildPVForSMB(vm *plan.VMStatus) (pv *core.PersistentVolume) {
	sourceProvider := r.Source.Provider
	smbSource := ctrlutil.ParseSMBSource(sourceProvider.Spec.URL)
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
		"provider":  r.Plan.Provider.Source.Name,
		"app":       "forklift",
		"migration": r.Migration.Name,
		"plan":      string(r.Plan.UID),
		"hyperv":    HyperVPVLabel,
		kVM:         vmID,
	}
}

// smbPVCLabels returns labels for HyperV SMB PVC.
func (r *KubeVirt) smbPVCLabels(vmID string) map[string]string {
	return map[string]string{
		"provider":  r.Plan.Provider.Source.Name,
		"app":       "forklift",
		"migration": string(r.Migration.UID),
		"plan":      string(r.Plan.UID),
		"hyperv":    HyperVPVCLabel,
		kVM:         vmID,
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

// DeleteAllPlanPods deletes all pods associated with this plan.
func (r *KubeVirt) DeleteAllPlanPods() error {
	selector := k8slabels.SelectorFromSet(r.planOnlyLabels())
	list := &core.PodList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		pod := &list.Items[i]
		err = r.Destination.Client.Delete(context.TODO(), pod)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete pod during plan cleanup.", "pod", pod.Name)
		} else if err == nil {
			r.Log.Info("Deleted pod during plan cleanup.", "pod", pod.Name)
		}
	}
	return nil
}

// DeleteAllPlanSecrets deletes all secrets associated with this plan.
func (r *KubeVirt) DeleteAllPlanSecrets() error {
	selector := k8slabels.SelectorFromSet(r.planOnlyLabels())
	list := &core.SecretList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		secret := &list.Items[i]
		// Only delete secrets that have the 'resource' label (migration temp secrets)
		// VM-dependency secrets (OCP-to-OCP) don't have this label and should be preserved
		if _, hasResourceLabel := secret.Labels[kResource]; !hasResourceLabel {
			continue
		}
		err = r.Destination.Client.Delete(context.TODO(), secret)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete secret during plan cleanup.", "secret", secret.Name)
		} else if err == nil {
			r.Log.Info("Deleted secret during plan cleanup.", "secret", secret.Name)
		}
	}
	return nil
}

// DeleteAllPlanConfigMaps deletes all configmaps associated with this plan.
func (r *KubeVirt) DeleteAllPlanConfigMaps() error {
	selector := k8slabels.SelectorFromSet(r.planOnlyLabels())
	list := &core.ConfigMapList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		cm := &list.Items[i]
		// Only delete configmaps that have the 'resource' label (migration temp configmaps)
		// VM-dependency configmaps (OCP-to-OCP) don't have this label and should be preserved
		if _, hasResourceLabel := cm.Labels[kResource]; !hasResourceLabel {
			continue
		}
		err = r.Destination.Client.Delete(context.TODO(), cm)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete configmap during plan cleanup.", "configmap", cm.Name)
		} else if err == nil {
			r.Log.Info("Deleted configmap during plan cleanup.", "configmap", cm.Name)
		}
	}
	return nil
}

// DeleteAllPlanJobs deletes all jobs associated with this plan.
func (r *KubeVirt) DeleteAllPlanJobs() error {
	selector := k8slabels.SelectorFromSet(r.planOnlyLabels())
	list := &batch.JobList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		job := &list.Items[i]
		background := meta.DeletePropagationBackground
		opts := &client.DeleteOptions{PropagationPolicy: &background}
		err = r.Destination.Client.Delete(context.TODO(), job, opts)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete job during plan cleanup.", "job", job.Name)
		} else if err == nil {
			r.Log.Info("Deleted job during plan cleanup.", "job", job.Name)
		}
	}
	return nil
}

// DeleteAllPlanPopulatorCRs deletes all populator CRs associated with this plan.
// This includes OvirtVolumePopulator, OpenstackVolumePopulator, and VSphereXcopyVolumePopulator.
func (r *KubeVirt) DeleteAllPlanPopulatorCRs() error {
	selector := k8slabels.SelectorFromSet(r.planOnlyLabels())
	opts := &client.ListOptions{
		LabelSelector: selector,
		Namespace:     r.Plan.Spec.TargetNamespace,
	}

	var listErrors []error

	// Delete OvirtVolumePopulator CRs
	ovirtList := &api.OvirtVolumePopulatorList{}
	if err := r.Destination.Client.List(context.TODO(), ovirtList, opts); err != nil {
		listErr := liberr.Wrap(err, "failed to list OvirtVolumePopulator CRs", "namespace", r.Plan.Spec.TargetNamespace)
		r.Log.Error(listErr, "Failed to list OvirtVolumePopulator CRs during plan cleanup.")
		listErrors = append(listErrors, listErr)
	} else {
		for i := range ovirtList.Items {
			cr := &ovirtList.Items[i]
			if err := r.Destination.Client.Delete(context.TODO(), cr); err != nil && !k8serr.IsNotFound(err) {
				r.Log.Error(err, "Failed to delete OvirtVolumePopulator during plan cleanup.", "name", cr.Name)
			} else if err == nil {
				r.Log.Info("Deleted OvirtVolumePopulator during plan cleanup.", "name", cr.Name)
			}
		}
	}

	// Delete OpenstackVolumePopulator CRs
	openstackList := &api.OpenstackVolumePopulatorList{}
	if err := r.Destination.Client.List(context.TODO(), openstackList, opts); err != nil {
		listErr := liberr.Wrap(err, "failed to list OpenstackVolumePopulator CRs", "namespace", r.Plan.Spec.TargetNamespace)
		r.Log.Error(listErr, "Failed to list OpenstackVolumePopulator CRs during plan cleanup.")
		listErrors = append(listErrors, listErr)
	} else {
		for i := range openstackList.Items {
			cr := &openstackList.Items[i]
			if err := r.Destination.Client.Delete(context.TODO(), cr); err != nil && !k8serr.IsNotFound(err) {
				r.Log.Error(err, "Failed to delete OpenstackVolumePopulator during plan cleanup.", "name", cr.Name)
			} else if err == nil {
				r.Log.Info("Deleted OpenstackVolumePopulator during plan cleanup.", "name", cr.Name)
			}
		}
	}

	// Delete VSphereXcopyVolumePopulator CRs
	vsphereList := &api.VSphereXcopyVolumePopulatorList{}
	if err := r.Destination.Client.List(context.TODO(), vsphereList, opts); err != nil {
		listErr := liberr.Wrap(err, "failed to list VSphereXcopyVolumePopulator CRs", "namespace", r.Plan.Spec.TargetNamespace)
		r.Log.Error(listErr, "Failed to list VSphereXcopyVolumePopulator CRs during plan cleanup.")
		listErrors = append(listErrors, listErr)
	} else {
		for i := range vsphereList.Items {
			cr := &vsphereList.Items[i]
			if err := r.Destination.Client.Delete(context.TODO(), cr); err != nil && !k8serr.IsNotFound(err) {
				r.Log.Error(err, "Failed to delete VSphereXcopyVolumePopulator during plan cleanup.", "name", cr.Name)
			} else if err == nil {
				r.Log.Info("Deleted VSphereXcopyVolumePopulator during plan cleanup.", "name", cr.Name)
			}
		}
	}

	return errors.Join(listErrors...)
}

// RemoveAllPlanPVCFinalizers removes the populator finalizer from all PVCs
// associated with this plan. This allows PVCs to be deleted by users after archive.
func (r *KubeVirt) RemoveAllPlanPVCFinalizers() error {
	selector := k8slabels.SelectorFromSet(r.planOnlyLabels())
	list := &core.PersistentVolumeClaimList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		pvc := &list.Items[i]
		if err := r.removePopulatorFinalizerFromPVC(pvc); err != nil {
			r.Log.Error(err, "Failed to remove finalizer from PVC during plan cleanup.", "pvc", pvc.Name)
		}
	}
	return nil
}

// DeleteMigrationVMs deletes all VMs for a specific migration.
func (r *KubeVirt) DeleteMigrationVMs(migrationUID string) error {
	selector := k8slabels.SelectorFromSet(r.migrationOnlyLabels(migrationUID))
	list := &cnv.VirtualMachineList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		vm := &list.Items[i]
		foreground := meta.DeletePropagationForeground
		opts := &client.DeleteOptions{PropagationPolicy: &foreground}
		err = r.Destination.Client.Delete(context.TODO(), vm, opts)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete VM during migration cleanup.", "vm", vm.Name, "migration", migrationUID)
		} else if err == nil {
			r.Log.Info("Deleted VM during migration cleanup.", "vm", vm.Name, "migration", migrationUID)
		}
	}
	return nil
}

// DeleteMigrationDataVolumes deletes all DataVolumes for a specific migration.
func (r *KubeVirt) DeleteMigrationDataVolumes(migrationUID string) error {
	selector := k8slabels.SelectorFromSet(r.migrationOnlyLabels(migrationUID))
	list := &cdi.DataVolumeList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		dv := &list.Items[i]
		err = r.Destination.Client.Delete(context.TODO(), dv)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete DataVolume during migration cleanup.", "datavolume", dv.Name, "migration", migrationUID)
		} else if err == nil {
			r.Log.Info("Deleted DataVolume during migration cleanup.", "datavolume", dv.Name, "migration", migrationUID)
		}
	}
	return nil
}

// DeleteMigrationPVCs deletes all PVCs for a specific migration.
func (r *KubeVirt) DeleteMigrationPVCs(migrationUID string) error {
	selector := k8slabels.SelectorFromSet(r.migrationOnlyLabels(migrationUID))
	list := &core.PersistentVolumeClaimList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}
	for i := range list.Items {
		pvc := &list.Items[i]
		err = r.Destination.Client.Delete(context.TODO(), pvc)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete PVC during migration cleanup.", "pvc", pvc.Name, "migration", migrationUID)
		} else if err == nil {
			r.Log.Info("Deleted PVC during migration cleanup.", "pvc", pvc.Name, "migration", migrationUID)
		}
	}
	return nil
}

// DeleteMigrationPods deletes all pods for a specific migration.
// Note: Uses only migration label since populator pods don't have the plan label.
func (r *KubeVirt) DeleteMigrationPods(migrationUID string) error {
	selector := k8slabels.SelectorFromSet(map[string]string{kMigration: migrationUID})
	list := &core.PodList{}
	err := r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: selector,
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return liberr.Wrap(err)
	}

	for i := range list.Items {
		pod := &list.Items[i]
		err = r.Destination.Client.Delete(context.TODO(), pod)
		if err != nil && !k8serr.IsNotFound(err) {
			r.Log.Error(err, "Failed to delete pod during migration cleanup.", "pod", pod.Name, "migration", migrationUID)
		}
	}
	return nil
}

// determineRunStrategy determines the appropriate run strategy based on the target power state configuration
func (r *KubeVirt) determineRunStrategy(vm *plan.VMStatus) cnv.VirtualMachineRunStrategy {
	// Determine the target power state based on plan configuration
	targetPowerState := vm.TargetPowerState
	if targetPowerState == "" {
		targetPowerState = r.Plan.Spec.TargetPowerState
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
