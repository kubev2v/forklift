package plan

import (
	"context"
	"encoding/json"
	"encoding/xml"
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
	libvirtxml "libvirt.org/libvirt-go-xml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	OvaPVCLabel = "nfs-pvc"
	OvaPVLabel  = "nfs-pv"
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
func (r *KubeVirt) EnsureVirtV2vPod(vm *plan.VMStatus, vmCr *VirtualMachine, pvcs []*core.PersistentVolumeClaim, podType int) (err error) {
	labels := r.vmLabels(vm.Ref)
	v2vSecret, err := r.ensureSecret(vm.Ref, r.secretDataSetterForCDI(vm.Ref), labels)
	if err != nil {
		return
	}

	var libvirtConfigMap = &core.ConfigMap{}
	if podType == VirtV2vConversionPod {
		libvirtConfigMap, err = r.ensureLibvirtConfigMap(vm.Ref, vmCr, pvcs)
		if err != nil {
			return
		}
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
	newPod, err := r.getVirtV2vPod(vm, vmVolumes, libvirtConfigMap, vddkConfigMap, pvcs, v2vSecret, podType)
	if err != nil {
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

func (r *KubeVirt) EnsureOVAVirtV2VPVCStatus(vmID string) (ready bool, err error) {
	pvcs := &core.PersistentVolumeClaimList{}
	pvcLabels := map[string]string{
		"migration": string(r.Migration.UID),
		"ova":       OvaPVCLabel,
		kVM:         vmID,
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
		for _, pvcVirtV2v := range pvcs.Items {
			if pvcVirtV2v.CreationTimestamp.Time.After(r.Migration.CreationTimestamp.Time) {
				pvc = &pvcVirtV2v
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
	case api.Ova:
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
	list, err := r.getPopulatorPods()
	for _, object := range list {
		err = r.DeleteObject(&object, vm, "Deleted populator pod.", "pod")
	}
	return
}

// Get populator pods that belong to a VM's migration.
func (r *KubeVirt) getPopulatorPods() (pods []core.Pod, err error) {
	migrationPods, err := r.GetPodsWithLabels(map[string]string{kMigration: string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)})
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

	if r.Plan.IsWarm() || !r.Destination.Provider.IsHost() || r.Plan.IsSourceProviderOCP() {
		// Set annotation for WFFC storage classes. Note that we create data volumes while
		// running a cold migration to the local cluster only when the source is either OpenShift
		// or vSphere, and in the latter case the conversion pod acts as the first-consumer
		annotations[planbase.AnnBindImmediate] = "true"
	}
	if r.Plan.IsWarm() && r.Builder.SupportsVolumePopulators() {
		// For storage offload, tie DataVolume to pre-imported PVC
		annotations[planbase.AnnAllowClaimAdoption] = "true"
		annotations[planbase.AnnPrePopulated] = "true"
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

func (r *KubeVirt) getVirtV2vPod(vm *plan.VMStatus, vmVolumes []cnv.Volume, libvirtConfigMap *core.ConfigMap, vddkConfigmap *core.ConfigMap, pvcs []*core.PersistentVolumeClaim, v2vSecret *core.Secret, podType int) (pod *core.Pod, err error) {
	volumes, volumeMounts, volumeDevices, err := r.podVolumeMounts(vmVolumes, libvirtConfigMap, vddkConfigmap, pvcs, vm, podType)
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
	if (useV2vForTransfer && !r.IsCopyOffload(pvcs)) || podType == VirtV2vInspectionPod {
		// mount the secret for the password and CA certificate
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
	} else {
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

	var podName string
	var containerName string
	// pod labels - start with user-defined labels, then system conversion labels override them
	podLabels := make(map[string]string)
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
		environment = append(environment,
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

		// Add disks to be inspected
		for i, disk := range virtualMachine.Disks {
			environment = append(environment, core.EnvVar{
				Name:  fmt.Sprintf("V2V_remoteInspectDisk_%d", i),
				Value: disk.ParentFile,
			})
		}
	}

	// pod node selector
	var podNodeSelector map[string]string
	if r.Plan.Spec.ConvertorNodeSelector != nil {
		podNodeSelector = make(map[string]string)
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

func (r *KubeVirt) podVolumeMounts(vmVolumes []cnv.Volume, libvirtConfigMap *core.ConfigMap, vddkConfigmap *core.ConfigMap, pvcs []*core.PersistentVolumeClaim, vm *plan.VMStatus, podType int) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, err error) {
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

	if podType == VirtV2vConversionPod {
		// add volume and mount for the libvirt domain xml config map.
		// the virt-v2v pod expects to see the libvirt xml at /mnt/v2v/input.xml
		volumes = append(volumes, core.Volume{
			Name: "libvirt-domain-xml",
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: libvirtConfigMap.Name,
					},
				},
			},
		})
		mounts = append(mounts,
			core.VolumeMount{
				Name:      "libvirt-domain-xml",
				MountPath: "/mnt/v2v",
			},
		)
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
	case api.Ova:
		var pvName string
		pvName, err = r.CreatePvForNfs()
		if err != nil {
			return
		}
		pvcNamePrefix := getEntityPrefixName("pvc", r.Source.Provider.Name, r.Plan.Name)
		var pvcName string
		pvcName, err = r.CreatePvcForNfs(pvcNamePrefix, pvName, vm.ID)
		if err != nil {
			return
		}

		//path from disk
		volumes = append(volumes, core.Volume{
			Name: "store-pv",
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})
		mounts = append(mounts,
			core.VolumeMount{
				Name:      VddkVolumeName,
				MountPath: "/opt",
			},
			core.VolumeMount{
				Name:      "store-pv",
				MountPath: "/ova",
			},
		)
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

	_, exists, err := r.findConfigMapInNamespace(Settings.VirtCustomizeConfigMap, r.Plan.Spec.TargetNamespace)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if exists {
		volumes = append(volumes, core.Volume{
			Name: DynamicScriptsVolumeName,
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: Settings.VirtCustomizeConfigMap,
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

func (r *KubeVirt) libvirtDomain(vmCr *VirtualMachine, pvcs []*core.PersistentVolumeClaim) (domain *libvirtxml.Domain) {
	pvcsByName := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range pvcs {
		pvcsByName[pvc.Name] = pvc
	}

	// FIXME: this should really be as complete an XML domain definition as possible
	// to give virt-v2v the best chance of converting the disk correctly. Things
	// like block device name translation and network translation may not work properly
	// without the full metadata, so we may see weird things happening in some
	// conversions. For now, this xml definition is just a minimal domain XML file
	// with the locations of each disk on the VM that is to be converted, but it
	// should be fixed properly in the future.
	libvirtDisks := make([]libvirtxml.DomainDisk, 0)
	for i, vol := range vmCr.Spec.Template.Spec.Volumes {
		diskSource := libvirtxml.DomainDiskSource{}

		pvc := pvcsByName[vol.PersistentVolumeClaim.ClaimName]
		if pvc == nil {
			r.Log.V(1).Info(
				"Failed to find the PVC to the Volume for the libvirt domain",
				"volume",
				vol.Name,
				"pvc",
				vol.PersistentVolumeClaim.ClaimName)
			continue
		}
		if pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == core.PersistentVolumeBlock {
			diskSource.Block = &libvirtxml.DomainDiskSourceBlock{
				Dev: fmt.Sprintf("/dev/block%v", i),
			}
		} else {
			diskSource.File = &libvirtxml.DomainDiskSourceFile{
				// the location where the disk images will be found on
				// the virt-v2v pod. See also podVolumeMounts.
				File: fmt.Sprintf("/mnt/disks/disk%v/disk.img", i),
			}
		}

		libvirtDisk := libvirtxml.DomainDisk{
			Device: "disk",
			Driver: &libvirtxml.DomainDiskDriver{
				Name: "qemu",
				Type: "raw",
			},
			Source: &diskSource,
			Target: &libvirtxml.DomainDiskTarget{
				Dev: "sd" + string(rune('a'+i)),
				Bus: "scsi",
			},
		}
		libvirtDisks = append(libvirtDisks, libvirtDisk)
	}

	kDomain := vmCr.Spec.Template.Spec.Domain
	domain = &libvirtxml.Domain{
		Type: "kvm",
		Name: vmCr.Name,
		Memory: &libvirtxml.DomainMemory{
			Value: uint(kDomain.Resources.Requests.Memory().Value()),
		},
		CPU: &libvirtxml.DomainCPU{
			Topology: &libvirtxml.DomainCPUTopology{
				Sockets: int(kDomain.CPU.Sockets),
				Cores:   int(kDomain.CPU.Cores),
			},
		},
		OS: &libvirtxml.DomainOS{
			Type: &libvirtxml.DomainOSType{
				Type: "hvm",
			},
			BootDevices: []libvirtxml.DomainBootDevice{
				{
					Dev: "sd",
				},
			},
		},
		Devices: &libvirtxml.DomainDeviceList{
			Disks: libvirtDisks,
		},
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

// Ensure the Libvirt domain config map exists on the destination.
func (r *KubeVirt) ensureLibvirtConfigMap(vmRef ref.Ref, vmCr *VirtualMachine, pvcs []*core.PersistentVolumeClaim) (configMap *core.ConfigMap, err error) {
	configMap, err = r.ensureConfigMap(vmRef)
	if err != nil {
		return
	}
	domain := r.libvirtDomain(vmCr, pvcs)
	domainXML, err := xml.Marshal(domain)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if configMap.BinaryData == nil {
		configMap.BinaryData = make(map[string][]byte)
	}
	configMap.BinaryData["input.xml"] = domainXML
	err = r.Destination.Client.Update(context.TODO(), configMap)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.V(1).Info(
		"ConfigMap updated.",
		"configMap",
		path.Join(
			configMap.Namespace,
			configMap.Name),
		"vm",
		vmRef.String())

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
	return
}

// Labels for a vddk config
// We need to distinguish between the libvirt configmap which uses also the plan labels and the vddk configmap
func (r *KubeVirt) vddkLabels() (labels map[string]string) {
	labels = r.planLabels()
	labels[kUse] = VddkConf
	return
}

// Labels for a VM on a plan without migration label.
func (r *KubeVirt) vmAllButMigrationLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.vmLabels(vmRef)
	delete(labels, kMigration)
	return
}

// setTransferNetwork sets the transfer network annotation on the DataVolume so
// that it can be used by the importer pod. If the `forklift.konveyor.io/route` annotation
// is present on the referenced NAD, then it will be used with the `k8s.v1.cni.cncf.io/networks` annotation
// to set the default route. If not, this will fall back to setting the `v1.multus-cni.io/default-network` annotation
// with the namespaced name of the NAD.
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

	route, found := netAttachDef.Annotations[AnnForkliftNetworkRoute]
	if found {
		nse := k8snet.NetworkSelectionElement{
			Namespace: key.Namespace,
			Name:      key.Name,
		}
		ip := net.ParseIP(route)
		if ip != nil {
			nse.GatewayRequest = []net.IP{ip}
		} else {
			err = liberr.New(
				"Transfer network default route annotation is not a valid IP address.",
				"route", route)
			return
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

func (r *KubeVirt) setPopulatorPodLabels(pod core.Pod, migrationId string) (err error) {
	podCopy := pod.DeepCopy()
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[kMigration] = migrationId
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

func GetOvaPvListNfs(dClient client.Client, planID string) (pvs *core.PersistentVolumeList, found bool, err error) {
	pvs = &core.PersistentVolumeList{}
	pvLabels := map[string]string{
		"plan": planID,
		"ova":  OvaPVLabel,
	}

	err = dClient.List(
		context.TODO(),
		pvs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(pvLabels),
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

func GetOvaPvcListNfs(dClient client.Client, planID string, planNamespace string) (pvcs *core.PersistentVolumeClaimList, found bool, err error) {
	pvcs = &core.PersistentVolumeClaimList{}
	pvcLabels := map[string]string{
		"plan": planID,
		"ova":  OvaPVCLabel,
	}

	err = dClient.List(
		context.TODO(),
		pvcs,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(pvcLabels),
			Namespace:     planNamespace,
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

func (r *KubeVirt) CreatePvForNfs() (pvName string, err error) {
	sourceProvider := r.Source.Provider
	splitted := strings.Split(sourceProvider.Spec.URL, ":")
	nfsServer := splitted[0]
	nfsPath := splitted[1]
	pvcNamePrefix := getEntityPrefixName("pv", r.Source.Provider.Name, r.Plan.Name)

	labels := map[string]string{"provider": r.Plan.Provider.Source.Name, "app": "forklift", "migration": r.Migration.Name, "plan": string(r.Plan.UID), "ova": OvaPVLabel}
	pv := &core.PersistentVolume{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: pvcNamePrefix,
			Labels:       labels,
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse("1Gi"),
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
	err = r.Destination.Create(context.TODO(), pv)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA plan PV")
		return
	}
	pvName = pv.Name
	return
}

func (r *KubeVirt) CreatePvcForNfs(pvcNamePrefix, pvName, vmID string) (pvcName string, err error) {
	sc := ""
	labels := map[string]string{"provider": r.Plan.Provider.Source.Name, "app": "forklift", "migration": string(r.Migration.UID), "plan": string(r.Plan.UID), "ova": OvaPVCLabel, kVM: vmID}
	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: pvcNamePrefix,
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       labels,
		},
		Spec: core.PersistentVolumeClaimSpec{
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadOnlyMany,
			},
			VolumeName:       pvName,
			StorageClassName: &sc,
		},
	}
	err = r.Destination.Create(context.TODO(), pvc)
	if err != nil {
		r.Log.Error(err, "Failed to create OVA plan PVC")
		return
	}

	pvcName = pvc.Name
	return
}

func getEntityPrefixName(resourceType, providerName, planName string) string {
	return fmt.Sprintf("ova-store-%s-%s-%s-", resourceType, providerName, planName)
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
