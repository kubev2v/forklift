package plan

import (
	"context"
	"encoding/xml"
	"fmt"
	"math/rand"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	template "github.com/openshift/api/template/v1"
	"github.com/openshift/library-go/pkg/template/generator"
	"github.com/openshift/library-go/pkg/template/templateprocessing"
	batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/klog/v2"
	cnv "kubevirt.io/api/core/v1"
	libvirtxml "libvirt.org/libvirt-go-xml"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/util"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Annotations
const (
	// Transfer network annotation (value=network-attachment-definition name)
	AnnDefaultNetwork = "v1.multus-cni.io/default-network"
	// Causes the importer pod to be retained after import.
	AnnRetainAfterCompletion = "cdi.kubevirt.io/storage.pod.retainAfterCompletion"
	// Contains validations for a Kubevirt VM. Needs to be removed when
	// creating a VM from a template.
	AnnKubevirtValidations = "vm.kubevirt.io/validations"
	// PVC annotation containing the name of the importer pod.
	AnnImporterPodName = "cdi.kubevirt.io/storage.import.importPodName"
	//  Original VM name on source (value=vmOriginalName)
	AnnOriginalName = "original-name"
	//  Original VM name on source (value=vmOriginalID)
	AnnOriginalID = "original-ID"
	// DV deletion on completion
	AnnDeleteAfterCompletion = "cdi.kubevirt.io/storage.deleteAfterCompletion"
	// DV immediate bind to WaitForFirstConsumer storage class
	AnnBindImmediate = "cdi.kubevirt.io/storage.bind.immediate.requested"
	// Max Length for vm name
	NameMaxLength = 63
)

// Labels
const (
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// VM label (value=vmID)
	kVM = "vmID"
	// App label
	kApp = "forklift.app"
)

// User
const (
	// Qemu user
	qemuUser = int64(107)
	// Qemu group
	qemuGroup = int64(107)
)

// Map of VirtualMachines keyed by vmID.
type VirtualMachineMap map[string]VirtualMachine

// Represents kubevirt.
type KubeVirt struct {
	*plancontext.Context
	// Builder
	Builder adapter.Builder
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
			LabelSelector: labels.SelectorFromSet(planLabels),
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
					DataVolume{
						DataVolume: dv,
						PVC:        pvc,
					})
			}
		}
	}

	return list, nil
}

// Ensure the namespace exists on the destination.
func (r *KubeVirt) EnsureNamespace() (err error) {
	ns := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: r.Plan.Spec.TargetNamespace,
		},
	}
	err = r.Destination.Client.Create(context.TODO(), ns)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			err = nil
		}
	}
	r.Log.Info(
		"Created namespace.",
		"import",
		ns.Name)

	return
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

// Delete the importer pod for a PersistentVolumeClaim.
func (r *KubeVirt) DeleteImporterPod(pvc core.PersistentVolumeClaim) (err error) {
	var pod *core.Pod
	var found bool
	pod, found, err = r.GetImporterPod(pvc)
	if err != nil || !found {
		return
	}
	err = r.Destination.Client.Delete(context.TODO(), pod)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	r.Log.Info(
		"Deleted importer pod.",
		"pod",
		path.Join(
			pod.Namespace,
			pod.Name),
		"pvc",
		pvc.Name)
	return
}

// Ensure the kubevirt VirtualMachine exists on the destination.
func (r *KubeVirt) EnsureVM(vm *plan.VMStatus) (err error) {
	newVM, err := r.virtualMachine(vm)
	if err != nil {
		return
	}

	list := &cnv.VirtualMachineList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	virtualMachine := &cnv.VirtualMachine{}
	if len(list.Items) == 0 {
		virtualMachine = newVM
		err = r.Destination.Client.Create(context.TODO(), virtualMachine)
		if err != nil {
			err = liberr.Wrap(err)
			return
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
		virtualMachine = &list.Items[0]
	}

	// set DataVolume owner references so that they'll be cleaned up
	// when the VirtualMachine is removed.
	dvs := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvs,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	pvcs, err := r.getPVCs(vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	for _, pvc := range pvcs {
		ownerRefs := []meta.OwnerReference{vmOwnerReference(virtualMachine)}
		pvcCopy := pvc.DeepCopy()
		pvc.OwnerReferences = ownerRefs
		patch := client.MergeFrom(pvcCopy)
		err = r.Destination.Client.Patch(context.TODO(), &pvc, patch)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

// Delete the Secret that was created for this VM.
func (r *KubeVirt) DeleteSecret(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmAllButMigrationLabels(vm.Ref)
	list := &core.SecretList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(vmLabels),
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
			LabelSelector: labels.SelectorFromSet(vmLabels),
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
			LabelSelector: labels.SelectorFromSet(vmLabels),
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

// Set the Running state on a Kubevirt VirtualMachine.
func (r *KubeVirt) SetRunning(vmCr *VirtualMachine, running bool) (err error) {
	vmCopy := vmCr.VirtualMachine.DeepCopy()
	vmCr.VirtualMachine.Spec.Running = &running
	patch := client.MergeFrom(vmCopy)
	err = r.Destination.Client.Patch(context.TODO(), vmCr.VirtualMachine, patch)
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

func (r *KubeVirt) DataVolumes(vm *plan.VMStatus) (dataVolumes []cdi.DataVolume, err error) {
	secret, err := r.ensureSecret(vm.Ref, r.secretDataSetterForCDI(vm.Ref))
	if err != nil {
		return
	}
	configMap, err := r.ensureConfigMap(vm.Ref)
	if err != nil {
		return
	}

	dataVolumes, err = r.dataVolumes(vm, secret, configMap)
	if err != nil {
		return
	}
	return
}

// Ensure the DataVolumes exist on the destination.
func (r *KubeVirt) EnsureDataVolumes(vm *plan.VMStatus, dataVolumes []cdi.DataVolume) (err error) {
	list := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	var pvcNames []string
	for _, dv := range dataVolumes {
		exists := false
		for _, item := range list.Items {
			if r.Builder.ResolveDataVolumeIdentifier(&dv) == r.Builder.ResolveDataVolumeIdentifier(&item) {
				exists = true
				break
			}
		}

		if !exists {
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
		// DataVolume and PVC names are the same
		pvcNames = append(pvcNames, dv.Name)
	}
	el9, el9Err := r.Context.Plan.VSphereUsesEl9VirtV2v()
	if el9Err != nil {
		err = el9Err
		return
	}
	if el9 {
		err = r.createPodToBindPVCs(vm, pvcNames)
		if err != nil {
			return err
		}
	}

	return
}

// Return DataVolumes associated with a VM.
func (r *KubeVirt) getDVs(vm *plan.VMStatus) (dvs []DataVolume, err error) {
	dvsList := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvsList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})

	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	dvs = []DataVolume{}
	for i := range dvsList.Items {
		dv := &dvsList.Items[i]
		dvs = append(dvs, DataVolume{
			DataVolume: dv,
		})
	}
	return
}

// Return PersistentVolumeClaims associated with a VM.
func (r *KubeVirt) getPVCs(vm *plan.VMStatus) (pvcs []core.PersistentVolumeClaim, err error) {
	pvcsList := &core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.TODO(),
		pvcsList,
		r.getListOptionsNamespaced(),
	)

	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	pvcs = []core.PersistentVolumeClaim{}
	vmLabels := r.vmLabels(vm.Ref)
	for i := range pvcsList.Items {
		pvc := &pvcsList.Items[i]
		pvcAnn := pvc.GetAnnotations()
		if pvcAnn[kVM] == vmLabels[kVM] && pvcAnn[kPlan] == vmLabels[kPlan] {
			pvcs = append(pvcs, *pvc)
		} else if r.Plan.IsSourceProviderOpenstack() {
			if _, ok := pvc.Labels["migration"]; ok {
				if pvc.Labels["migration"] == r.Migration.Name {
					pvcs = append(pvcs, *pvc)
				}
			}
		} else if r.useOvirtPopulator(vm) {
			ovirtVm := &ovirt.Workload{}
			err = r.Source.Inventory.Find(ovirtVm, vm.Ref)
			if err != nil {
				return
			}
			for _, da := range ovirtVm.DiskAttachments {
				if pvc.Spec.DataSource != nil && da.Disk.ID == pvc.Spec.DataSource.Name {
					pvcs = append(pvcs, *pvc)
					break
				}
			}
		}
	}

	return
}

func (r *KubeVirt) createVolumesForOvirt(vm *plan.VMStatus) (pvcNames []string, err error) {
	secret, err := r.ensureSecret(vm.Ref, r.copyDataFromProviderSecret)
	if err != nil {
		return
	}
	ovirtVm := &ovirt.Workload{}
	err = r.Source.Inventory.Find(ovirtVm, vm.Ref)
	if err != nil {
		return
	}
	sourceUrl, err := url.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		return
	}

	for _, da := range ovirtVm.DiskAttachments {
		if da.Disk.StorageType == "lun" {
			continue
		}
		// The VM has a disk image so the storage map is necessarily not empty, and we can read the storage class from it.
		storageName := &r.Context.Map.Storage.Spec.Map[0].Destination.StorageClass
		populatorCr := util.OvirtVolumePopulator(da, sourceUrl, r.Plan.Spec.TransferNetwork, r.Plan.Spec.TargetNamespace, secret.Name, vm.ID, string(r.Migration.UID))
		failure := r.Client.Create(context.Background(), populatorCr, &client.CreateOptions{})
		if failure != nil && !k8serr.IsAlreadyExists(failure) {
			return nil, failure
		}

		accessModes, volumeMode, failure := r.getDefaultVolumeAndAccessMode(*storageName)
		if failure != nil {
			return nil, failure
		}

		pvc := r.Builder.PersistentVolumeClaimWithSourceRef(da, storageName, populatorCr.Name, accessModes, volumeMode)
		if pvc == nil {
			klog.Errorf("Couldn't build the PVC %v", da.DiskAttachment.ID)
			return
		}
		err = r.Client.Create(context.TODO(), pvc, &client.CreateOptions{})
		if err != nil {
			return
		}
		pvcNames = append(pvcNames, pvc.Name)
	}

	err = r.createLunDisks(vm)

	return
}

// Creates the PVs and PVCs for LUN disks.
func (r *KubeVirt) createLunDisks(vm *plan.VMStatus) (err error) {
	lunPvcs, err := r.Builder.LunPersistentVolumeClaims(vm.Ref)
	if err != nil {
		return
	}
	err = r.EnsurePersistentVolumeClaim(vm, lunPvcs)
	if err != nil {
		return
	}
	lunPvs, err := r.Builder.LunPersistentVolumes(vm.Ref)
	if err != nil {
		return
	}
	err = r.EnsurePersistentVolume(vm, lunPvs)
	if err != nil {
		return
	}
	return
}

// Creates a pod associated with PVCs to create node bind (wait for consumer)
func (r *KubeVirt) createPodToBindPVCs(vm *plan.VMStatus, pvcNames []string) error {
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
					Image:   Settings.Migration.VirtV2vImageCold,
					Command: []string{"/bin/sh"},
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
		},
	}
	// Align with the conversion pod request, to prevent breakage
	r.setKvmOnPodSpec(&pod.Spec)

	err := r.Client.Create(context.TODO(), pod, &client.CreateOptions{})
	if err != nil {
		return err
	}
	r.Log.Info(fmt.Sprintf("Created pod '%s' to init the PVC node", pod.Name))
	return nil
}

// Sets KVM requirement to the pod and container.
func (r *KubeVirt) setKvmOnPodSpec(podSpec *core.PodSpec) {
	if *r.Plan.Provider.Source.Spec.Type == v1beta1.VSphere && !Settings.VirtV2vDontRequestKVM {
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

func (r *KubeVirt) areOvirtPVCsReady(vm ref.Ref, step *plan.Step) (ready bool, err error) {
	ovirtVm := &ovirt.Workload{}
	err = r.Source.Inventory.Find(ovirtVm, vm)
	if err != nil {
		return
	}
	ready = true

	for _, da := range ovirtVm.DiskAttachments {
		if da.Disk.StorageType == "lun" {
			continue
		}
		obj := client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: da.Disk.ID}
		pvc := core.PersistentVolumeClaim{}
		err = r.Client.Get(context.Background(), obj, &pvc)
		if err != nil {
			return
		}

		if pvc.Status.Phase != core.ClaimBound {
			ready = false
			break
		}

		task, found := step.FindTask(da.Disk.ID)
		if !found {
			continue
		}

		task.MarkCompleted()
	}

	return
}

var filesystemMode = core.PersistentVolumeFilesystem

// Using CDI logic to set the Volume mode and Access mode of the PVC - https://github.com/kubevirt/containerized-data-importer/blob/v1.56.0/pkg/controller/datavolume/util.go#L154
func (r *KubeVirt) getDefaultVolumeAndAccessMode(storageName string) ([]core.PersistentVolumeAccessMode, *core.PersistentVolumeMode, error) {
	storageProfile := &cdi.StorageProfile{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: storageName}, storageProfile)
	if err != nil {
		return nil, nil, liberr.Wrap(err, "cannot get StorageProfile")
	}

	if len(storageProfile.Status.ClaimPropertySets) > 0 &&
		len(storageProfile.Status.ClaimPropertySets[0].AccessModes) > 0 {
		accessModes := storageProfile.Status.ClaimPropertySets[0].AccessModes
		volumeMode := storageProfile.Status.ClaimPropertySets[0].VolumeMode
		if volumeMode == nil {
			// volumeMode is an optional API parameter. Filesystem is the default mode used when volumeMode parameter is omitted.
			volumeMode = &filesystemMode
		}
		return accessModes, volumeMode, nil
	}

	// no accessMode configured on storageProfile
	return nil, nil, errors.Errorf("no accessMode defined on StorageProfile for %s StorageClass", storageName)
}

// Return true when the import is done with OvirtVolumePopulator
func (r *KubeVirt) useOvirtPopulator(vm *plan.VMStatus) bool {
	return r.Plan.IsSourceProviderOvirt() && vm.Warm == nil && r.Destination.Provider.IsHost()
}

// Return namespace specific ListOption.
func (r *KubeVirt) getListOptionsNamespaced() (listOptions *client.ListOptions) {
	return &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
	}
}

// Ensure the guest conversion (virt-v2v) pod exists on the destination.
func (r *KubeVirt) EnsureGuestConversionPod(vm *plan.VMStatus, vmCr *VirtualMachine, pvcs *[]core.PersistentVolumeClaim) (err error) {
	v2vSecret, err := r.ensureSecret(vm.Ref, r.secretDataSetterForCDI(vm.Ref))
	if err != nil {
		return
	}

	configMap, err := r.ensureLibvirtConfigMap(vm.Ref, vmCr, pvcs)
	if err != nil {
		return
	}

	newPod, err := r.guestConversionPod(vm, vmCr.Spec.Template.Spec.Volumes, configMap, pvcs, v2vSecret)
	if err != nil {
		return
	}

	list, err := r.GetPodsWithLabels(r.conversionLabels(vm.Ref, true))
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

// Get the guest conversion pod for the VM.
func (r *KubeVirt) GetGuestConversionPod(vm *plan.VMStatus) (pod *core.Pod, err error) {
	list := &core.PodList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.conversionLabels(vm.Ref, false)),
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
			LabelSelector: labels.SelectorFromSet(podLabels),
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
func (r *KubeVirt) DeleteObject(object client.Object, vm *plan.VMStatus, message, objType string) (err error) {
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
			LabelSelector: labels.SelectorFromSet(vmLabels),
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

// Get the OpenstackVolumePopulator CustomResource based on the image name.
func (r *KubeVirt) getOpenstackPopulatorCr(name string) (populatorCr v1beta1.OpenstackVolumePopulator, err error) {
	populatorCr = v1beta1.OpenstackVolumePopulator{}
	err = r.Destination.Client.Get(context.TODO(), client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: name}, &populatorCr)
	return
}

// Get the OvirtVolumePopulator CustomResource based on the PVC name.
func (r *KubeVirt) getOvirtPopulatorCr(name string) (populatorCr v1beta1.OvirtVolumePopulator, err error) {
	populatorCr = v1beta1.OvirtVolumePopulator{}
	err = r.Destination.Client.Get(context.TODO(), client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: name}, &populatorCr)
	return
}

// Set the Populator Pod Ownership.
func (r *KubeVirt) SetPopulatorPodOwnership(vm *plan.VMStatus) (err error) {
	pvcs, err := r.getPVCs(vm)
	if err != nil {
		return
	}
	pods, err := r.getPopulatorPods()
	if err != nil {
		return
	}
	for _, pod := range pods {
		pvcId := strings.Split(pod.Name, "populate-")[1]
		for _, pvc := range pvcs {
			if string(pvc.UID) == pvcId {
				podCopy := pod.DeepCopy()
				err = k8sutil.SetOwnerReference(&pvc, &pod, r.Scheme())
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
	}
	return
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
		if strings.HasPrefix(pod.Name, "populate-") {
			pods = append(pods, pod)
		}
	}
	return
}

// Build the DataVolume CRs.
func (r *KubeVirt) dataVolumes(vm *plan.VMStatus, secret *core.Secret, configMap *core.ConfigMap) (dataVolumes []cdi.DataVolume, err error) {
	_, err = r.Source.Inventory.VM(&vm.Ref)
	if err != nil {
		return
	}

	annotations := r.vmLabels(vm.Ref)
	if !r.Plan.Spec.Warm || Settings.RetainPrecopyImporterPods {
		annotations[AnnRetainAfterCompletion] = "true"
	}
	if r.Plan.Spec.TransferNetwork != nil {
		annotations[AnnDefaultNetwork] = path.Join(
			r.Plan.Spec.TransferNetwork.Namespace, r.Plan.Spec.TransferNetwork.Name)
	}
	if r.Plan.Spec.Warm || !r.Destination.Provider.IsHost() {
		annotations[AnnBindImmediate] = "true"
	}
	// Do not delete the DV when the import completes as we check the DV to get the current
	// disk transfer status.
	annotations[AnnDeleteAfterCompletion] = "false"
	dvTemplate := cdi.DataVolume{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Spec.TargetNamespace,
			Annotations:  annotations,
			GenerateName: r.getGeneratedName(vm),
		},
	}
	dvTemplate.Labels = r.vmLabels(vm.Ref)

	dataVolumes, err = r.Builder.DataVolumes(vm.Ref, secret, configMap, &dvTemplate)
	if err != nil {
		return
	}

	err = r.createLunDisks(vm)

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

// Build the Kubevirt VM CR.
func (r *KubeVirt) virtualMachine(vm *plan.VMStatus) (object *cnv.VirtualMachine, err error) {
	pvcs, err := r.getPVCs(vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	//If the VM name is not valid according to DNS1123 labeling
	//convention it will be automatically changed.
	var originalName string

	if errs := k8svalidation.IsDNS1123Label(vm.Name); len(errs) > 0 {
		originalName = vm.Name
		vm.Name, err = r.changeVmNameDNS1123(vm.Name, r.Plan.Spec.TargetNamespace)
		if err != nil {
			r.Log.Error(err, "Failed to update the VM name to meet DNS1123 protocol requirements.")
			return
		}
		r.Log.Info("VM name is incompatible with DNS1123 RFC, renaming",
			"originalName", originalName, "newName", vm.Name)
	}

	if r.Plan.IsSourceProviderOCP() {
		object = r.emptyVm(vm)
	} else {
		var ok bool
		object, ok = r.vmTemplate(vm)
		if !ok {
			r.Log.Info("Building VirtualMachine without template.",
				"vm",
				vm.String())
			object = r.emptyVm(vm)
		}
	}

	//Add the original name and ID info to the VM annotations
	if len(originalName) > 0 {
		annotations := make(map[string]string)
		annotations[AnnOriginalName] = originalName
		annotations[AnnOriginalID] = vm.ID
		object.ObjectMeta.Annotations = annotations
	}

	running := false
	object.Spec.Running = &running

	err = r.Builder.VirtualMachine(vm.Ref, &object.Spec, pvcs)
	if err != nil {
		return
	}

	return
}

// Attempt to find a suitable template and extract a VirtualMachine definition from it.
func (r *KubeVirt) vmTemplate(vm *plan.VMStatus) (virtualMachine *cnv.VirtualMachine, ok bool) {
	tmpl, err := r.findTemplate(vm)
	if err != nil {
		r.Log.Error(err,
			"Could not find Template for destination VM.",
			"vm",
			vm.String())
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

	virtualMachine.Name = vm.Name
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
			Name:      vm.Name,
		},
		Spec: cnv.VirtualMachineSpec{},
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
			LabelSelector: labels.SelectorFromSet(templateLabels),
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

func (r *KubeVirt) guestConversionPod(vm *plan.VMStatus, vmVolumes []cnv.Volume, configMap *core.ConfigMap, pvcs *[]core.PersistentVolumeClaim, v2vSecret *core.Secret) (pod *core.Pod, err error) {
	volumes, volumeMounts, volumeDevices := r.podVolumeMounts(vmVolumes, configMap, pvcs)

	// qemu group
	fsGroup := qemuGroup
	user := qemuUser
	nonRoot := true
	allowPrivilageEscalation := false
	// virt-v2v image
	var virtV2vImage string
	el9, el9Err := r.Context.Plan.VSphereUsesEl9VirtV2v()
	if el9Err != nil {
		err = el9Err
		return
	}
	if el9 {
		virtV2vImage = Settings.Migration.VirtV2vImageCold
	} else {
		virtV2vImage = Settings.Migration.VirtV2vImageWarm
	}
	// VDDK image
	var initContainers []core.Container
	if vddkImage, found := r.Source.Provider.Spec.Settings["vddkInitImage"]; found {
		initContainers = append(initContainers, core.Container{
			Name:            "vddk-side-car",
			Image:           vddkImage,
			ImagePullPolicy: core.PullIfNotPresent,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "vddk-vol-mount",
					MountPath: "/opt",
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
	// pod environment
	environment, err := r.Builder.PodEnvironment(vm.Ref, r.Source.Secret)
	if err != nil {
		return
	}
	// pod
	pod = &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       r.conversionLabels(vm.Ref, false),
			GenerateName: r.getGeneratedName(vm),
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				FSGroup:      &fsGroup,
				RunAsUser:    &user,
				RunAsNonRoot: &nonRoot,
			},
			RestartPolicy:  core.RestartPolicyNever,
			InitContainers: initContainers,
			Containers: []core.Container{
				{
					Name: "virt-v2v",
					Env:  environment,
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
					Image:         virtV2vImage,
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

func (r *KubeVirt) podVolumeMounts(vmVolumes []cnv.Volume, configMap *core.ConfigMap, pvcs *[]core.PersistentVolumeClaim) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice) {
	pvcsByName := make(map[string]core.PersistentVolumeClaim)
	for _, pvc := range *pvcs {
		pvcsByName[pvc.Name] = pvc
	}

	for i, v := range vmVolumes {
		pvc, _ := pvcsByName[v.PersistentVolumeClaim.ClaimName]
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

	// add volume and mount for the libvirt domain xml config map.
	// the virt-v2v pod expects to see the libvirt xml at /mnt/v2v/input.xml
	volumes = append(volumes, core.Volume{
		Name: "libvirt-domain-xml",
		VolumeSource: core.VolumeSource{
			ConfigMap: &core.ConfigMapVolumeSource{
				LocalObjectReference: core.LocalObjectReference{
					Name: configMap.Name,
				},
			},
		},
	})
	volumes = append(volumes, core.Volume{
		Name: "nfs",
		VolumeSource: core.VolumeSource{
			NFS: &core.NFSVolumeSource{
				Server: "10.46.9.67",
				Path:   "/sd1/sd/ova",
			},
		},
	})
	mounts = append(mounts,
		core.VolumeMount{
			Name:      "libvirt-domain-xml",
			MountPath: "/mnt/v2v",
		},
		core.VolumeMount{
			Name:      "vddk-vol-mount",
			MountPath: "/opt",
		},
		core.VolumeMount{
			Name:      "nfs",
			MountPath: "/mnt/nfs/",
		},
	)

	// Temporary space for VDDK library
	volumes = append(volumes, core.Volume{
		Name: "vddk-vol-mount",
		VolumeSource: core.VolumeSource{
			EmptyDir: &core.EmptyDirVolumeSource{},
		},
	})

	return
}

func (r *KubeVirt) libvirtDomain(vmCr *VirtualMachine, pvcs *[]core.PersistentVolumeClaim) (domain *libvirtxml.Domain) {
	pvcsByName := make(map[string]core.PersistentVolumeClaim)
	for _, pvc := range *pvcs {
		pvcsByName[pvc.Name] = pvc
	}

	// virt-v2v needs a very minimal libvirt domain XML file to be provided
	// with the locations of each of the disks on the VM that is to be converted.
	libvirtDisks := make([]libvirtxml.DomainDisk, 0)
	for i, vol := range vmCr.Spec.Template.Spec.Volumes {
		diskSource := libvirtxml.DomainDiskSource{}

		pvc := pvcsByName[vol.PersistentVolumeClaim.ClaimName]
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
				Dev: "hd" + string(rune('a'+i)),
				Bus: "virtio",
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
					Dev: "hd",
				},
			},
		},
		Devices: &libvirtxml.DomainDeviceList{
			Disks: libvirtDisks,
		},
	}

	return
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
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vmRef)),
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
func (r *KubeVirt) ensureLibvirtConfigMap(vmRef ref.Ref, vmCr *VirtualMachine, pvcs *[]core.PersistentVolumeClaim) (configMap *core.ConfigMap, err error) {
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

// Ensure the credential secret for the data transfer exists on the destination.
func (r *KubeVirt) ensureSecret(vmRef ref.Ref, setSecretData func(*core.Secret) error) (secret *core.Secret, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		return
	}

	newSecret, err := r.secret(vmRef, setSecretData)
	if err != nil {
		return
	}

	list := &core.SecretList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vmRef)),
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
func (r *KubeVirt) secret(vmRef ref.Ref, setSecretData func(*core.Secret) error) (secret *core.Secret, err error) {
	secret = &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Labels:    r.vmLabels(vmRef),
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

// Labels for a VM on a plan.
func (r *KubeVirt) vmLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.planLabels()
	labels[kVM] = vmRef.ID
	return
}

// Labels for a VM on a plan without migration label.
func (r *KubeVirt) vmAllButMigrationLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.vmLabels(vmRef)
	delete(labels, kMigration)
	return
}

// Represents a CDI DataVolume and add behavior.
type DataVolume struct {
	*cdi.DataVolume
	PVC *core.PersistentVolumeClaim
}

// Get conditions.
func (r *DataVolume) Conditions() (cnd *libcnd.Conditions) {
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
func (r *DataVolume) PercentComplete() (pct float64) {
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
	DataVolumes []DataVolume
}

// Determine if `this` VirtualMachine is the
// owner of the CDI DataVolume.
func (r *VirtualMachine) Owner(dv *cdi.DataVolume) bool {
	for _, vol := range r.Spec.Template.Spec.Volumes {
		if vol.PersistentVolumeClaim.ClaimName == dv.Name {
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

// TODO move elsewhere
func (r *KubeVirt) ensureOpenStackVolumes(vm ref.Ref, ready bool) (pvcNames []string, err error) {
	secret, err := r.ensureSecret(vm, r.copyDataFromProviderSecret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	openstackVm := &openstack.Workload{}
	err = r.Source.Inventory.Find(openstackVm, vm)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	sourceUrl, err := url.Parse(r.Source.Provider.Spec.URL)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	storageName := r.Context.Map.Storage.Spec.Map[0].Destination.StorageClass

	if len(openstackVm.Volumes) > 0 {
		for _, vol := range openstackVm.Volumes {
			image := &openstack.Image{}
			err = r.Source.Inventory.Find(image, ref.Ref{Name: fmt.Sprintf("%s-%s", r.Migration.Name, vol.ID)})
			if err != nil {
				if !ready {
					err = nil
					r.Log.Info("Image is not found yet")
					continue
				}
				err = liberr.Wrap(err)
				return
			}

			if image.Status != "active" {
				r.Log.Info("Image is not active yet", "image", image.Name)
				continue
			}
			populatorCr := util.OpenstackVolumePopulator(image, sourceUrl, r.Plan.Spec.TransferNetwork, r.Plan.Spec.TargetNamespace, secret.Name, vm.ID, string(r.Migration.UID))
			err = r.Client.Create(context.TODO(), populatorCr, &client.CreateOptions{})
			if k8serr.IsAlreadyExists(err) {
				err = nil
			} else if err != nil {
				err = liberr.Wrap(err)
				return
			}
			accessModes, volumeMode, failure := r.getDefaultVolumeAndAccessMode(storageName)
			if failure != nil {
				return nil, failure
			}

			pvc := r.Builder.PersistentVolumeClaimWithSourceRef(image, &storageName, populatorCr.Name, accessModes, volumeMode)
			err = r.Client.Create(context.TODO(), pvc, &client.CreateOptions{})
			if k8serr.IsAlreadyExists(err) {
				err = nil
				continue
			} else if err != nil {
				err = liberr.Wrap(err)
				return
			}
			pvcNames = append(pvcNames, pvc.Name)
		}
	}

	return
}

func (r *KubeVirt) openstackPVCsReady(vm ref.Ref, step *plan.Step) (ready bool, err error) {
	openstackVm := &openstack.Workload{}
	err = r.Source.Inventory.Find(openstackVm, vm)
	if err != nil {
		return
	}
	ready = true

	for _, vol := range openstackVm.Volumes {
		lookupName := fmt.Sprintf("%s-%s", r.Migration.Name, vol.ID)
		image := &openstack.Image{}
		err = r.Source.Inventory.Find(image, ref.Ref{Name: lookupName})
		if err != nil {
			return
		}

		obj := client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: image.ID}
		pvc := core.PersistentVolumeClaim{}
		err = r.Client.Get(context.Background(), obj, &pvc)
		if err != nil {
			return
		}

		if pvc.Status.Phase != core.ClaimBound {
			ready = false
			return
		}
		var task *plan.Task
		found := false
		task, found = step.FindTask(lookupName)
		if !found {
			return
		}
		task.MarkCompleted()
	}

	return
}

func (r *KubeVirt) setOpenStackPopulatorLabels(populatorCr v1beta1.OpenstackVolumePopulator, vmId, migrationId string) (err error) {
	populatorCrCopy := populatorCr.DeepCopy()
	if populatorCr.Labels == nil {
		populatorCr.Labels = make(map[string]string)
	}
	populatorCr.Labels["vmID"] = vmId
	populatorCr.Labels["migration"] = migrationId
	patch := client.MergeFrom(populatorCrCopy)
	err = r.Destination.Client.Patch(context.TODO(), &populatorCr, patch)
	return
}

func (r *KubeVirt) setOvirtPopulatorLabels(populatorCr v1beta1.OvirtVolumePopulator, vmId, migrationId string) (err error) {
	populatorCrCopy := populatorCr.DeepCopy()
	if populatorCr.Labels == nil {
		populatorCr.Labels = make(map[string]string)
	}
	populatorCr.Labels["vmID"] = vmId
	populatorCr.Labels["migration"] = migrationId
	patch := client.MergeFrom(populatorCrCopy)
	err = r.Destination.Client.Patch(context.TODO(), &populatorCr, patch)
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
func (r *KubeVirt) EnsurePersistentVolume(vm *plan.VMStatus, persistentVolumes []core.PersistentVolume) (err error) {
	list := &core.PersistentVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
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
				vm.String())
		}
	}
	return
}

// Ensure the PV exist on the destination.
func (r *KubeVirt) EnsurePersistentVolumeClaim(vm *plan.VMStatus, persistentVolumeClaims []core.PersistentVolumeClaim) (err error) {
	list, err := r.getPVCs(vm)
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
				"vm",
				vm.String())
		}
	}
	return
}

// OCP source
func (r *KubeVirt) ensureOCPVolumes(vm *plan.VMStatus) error {
	_, err := r.DataVolumes(vm)
	if err != nil {
		r.Log.Info("DataVolumes are not ready yet", "error", err)
		return liberr.Wrap(err)
	}

	return nil
}
