package plan

import (
	"context"
	"encoding/xml"
	"fmt"
	"math/rand"
	"path"
	"regexp"
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
	cnv "kubevirt.io/client-go/api/v1"
	libvirtxml "libvirt.org/libvirt-go-xml"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	libcnd "github.com/konveyor/forklift-controller/pkg/lib/condition"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
)

// Labels
const (
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// VM label (value=vmID)
	kVM = "vmID"
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
		pvc.SetOwnerReferences(ownerRefs)
		err = r.Destination.Client.Update(context.TODO(), &pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

// Delete the Secret that was created for this VM.
func (r *KubeVirt) DeleteSecret(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmLabels(vm.Ref)
	delete(vmLabels, kMigration)
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
		err = r.Destination.Client.Delete(context.TODO(), &object)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
			} else {
				return liberr.Wrap(err)
			}
		} else {
			r.Log.Info(
				"Deleted secret.",
				"secret",
				path.Join(
					object.Namespace,
					object.Name),
				"vm",
				vm.String())
		}
	}
	return
}

// Delete the ConfigMap that was created for this VM.
func (r *KubeVirt) DeleteConfigMap(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmLabels(vm.Ref)
	delete(vmLabels, kMigration)
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
		err = r.Destination.Client.Delete(context.TODO(), &object)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
			} else {
				return liberr.Wrap(err)
			}
		} else {
			r.Log.Info(
				"Deleted configMap.",
				"configMap",
				path.Join(
					object.Namespace,
					object.Name),
				"vm",
				vm.String())
		}
	}
	return
}

// Delete the VirtualMachine CR on the destination cluster.
func (r *KubeVirt) DeleteVM(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmLabels(vm.Ref)
	delete(vmLabels, kMigration)
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
	secret, err := r.ensureSecret(vm.Ref)
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
		}
	}
	return
}

// Return namespace specific ListOption.
func (r *KubeVirt) getListOptionsNamespaced() (listOptions *client.ListOptions) {
	return &client.ListOptions{
		Namespace: r.Plan.Spec.TargetNamespace,
	}
}

// Ensure the guest conversion (virt-v2v) pod exists on the destination.
func (r *KubeVirt) EnsureGuestConversionPod(vm *plan.VMStatus, vmCr *VirtualMachine, pvcs *[]core.PersistentVolumeClaim) (err error) {
	v2vSecret, err := r.ensureSecret(vm.Ref)
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

	list := &core.PodList{}
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
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
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

// Delete the guest conversion pod on the destination cluster.
func (r *KubeVirt) DeleteGuestConversionPod(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmLabels(vm.Ref)
	delete(vmLabels, kMigration)
	list := &core.PodList{}
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
		err = r.Destination.Client.Delete(context.TODO(), &object)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
			} else {
				return liberr.Wrap(err)
			}
		} else {
			r.Log.Info(
				"Deleted guest conversion pod.",
				"pod",
				path.Join(
					object.Namespace,
					object.Name),
				"vm",
				vm.String())
		}
	}
	return
}

// Delete any hook jobs that belong to a VM migration.
func (r *KubeVirt) DeleteHookJobs(vm *plan.VMStatus) (err error) {
	vmLabels := r.vmLabels(vm.Ref)
	delete(vmLabels, kMigration)
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
		err = r.Destination.Client.Delete(context.TODO(), &object)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
			} else {
				return liberr.Wrap(err)
			}
		} else {
			r.Log.Info(
				"Deleted hook job.",
				"job",
				path.Join(
					object.Namespace,
					object.Name),
				"vm",
				vm.String())
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

		generatedName := changeVmName(vm.Name, vm.ID)
		nameExist, errName := r.checkIfVmNameExist(generatedName)
		if errName != nil {
			err = liberr.Wrap(errName)
			return
		}
		if nameExist {
			generatedName = generatedName + "-" + vm.ID[:4]
		}
		vm.Name = generatedName
		r.Log.Info("VM name ", originalName, " was incompatible with DNS1123 RFC, changing to ",
			vm.Name)
	}

	var ok bool
	object, ok = r.vmTemplate(vm)
	if !ok {
		r.Log.Info("Building VirtualMachine without template.",
			"vm",
			vm.String())
		object = r.emptyVm(vm)
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
	resourceReq := core.ResourceRequirements{}

	// Request access to /dev/kvm via Kubevirt's Device Manager
	// That is to ensure the appliance virt-v2v uses would not
	// run in emulation mode, which is significantly slower
	if !Settings.VirtV2vDontRequestKVM {
		resourceReq.Limits = core.ResourceList{
			"devices.kubevirt.io/kvm": resource.MustParse("1"),
		}
	}

	// qemu group
	fsGroup := int64(107)
	pod = &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    r.Plan.Spec.TargetNamespace,
			Labels:       r.vmLabels(vm.Ref),
			GenerateName: r.getGeneratedName(vm),
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				FSGroup: &fsGroup,
			},
			RestartPolicy: core.RestartPolicyNever,
			InitContainers: []core.Container{
				{
					Name:            "vddk-side-car",
					Image:           r.Source.Provider.Spec.Settings["vddkInitImage"],
					ImagePullPolicy: core.PullIfNotPresent,
					VolumeMounts: []core.VolumeMount{
						{
							Name:      "vddk-vol-mount",
							MountPath: "/opt",
						},
					},
				},
			},
			Containers: []core.Container{
				{
					Name: "virt-v2v",
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
					Image:           Settings.Migration.VirtV2vImage,
					VolumeMounts:    volumeMounts,
					VolumeDevices:   volumeDevices,
					ImagePullPolicy: core.PullIfNotPresent,
					Resources:       resourceReq,
				},
			},
			Volumes: volumes,
			// Ensure that the pod is deployed on a node where /dev/kvm is present.
			NodeSelector: map[string]string{
				"kubevirt.io/schedulable": "true",
			},
		},
	}

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
	mounts = append(mounts,
		core.VolumeMount{
			Name:      "libvirt-domain-xml",
			MountPath: "/mnt/v2v",
		},
		core.VolumeMount{
			Name:      "vddk-vol-mount",
			MountPath: "/opt",
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

// Ensure the DatVolume credential secret exists on the destination.
func (r *KubeVirt) ensureSecret(vmRef ref.Ref) (secret *core.Secret, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		return
	}
	newSecret, err := r.secret(vmRef)
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

// Build the DataVolume credential secret.
func (r *KubeVirt) secret(vmRef ref.Ref) (object *core.Secret, err error) {
	object = &core.Secret{
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
	err = r.Builder.Secret(vmRef, r.Source.Secret, object)

	return
}

// Labels for plan and migration.
func (r *KubeVirt) planLabels() map[string]string {
	return map[string]string{
		kMigration: string(r.Migration.UID),
		kPlan:      string(r.Plan.GetUID()),
	}
}

// Labels for a VM on a plan.
func (r *KubeVirt) vmLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.planLabels()
	labels[kVM] = vmRef.ID
	return
}

// Checks if VM with the newly generated name exists on the destination
func (r *KubeVirt) checkIfVmNameExist(name string) (nameExist bool, err error) {
	list := &cnv.VirtualMachineList{}
	nameFiled := "metadata.name"
	listOptions := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(nameFiled, name),
	}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		listOptions,
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		nameExist = true
		return
	}
	// Checks that the new name does not match a valid
	// VM name in the same plan
	for _, vm := range r.Migration.Status.VMs {
		if vm.Name == name {
			nameExist = true
			return
		}
	}
	nameExist = false
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

// Convert the combined progress of all DataVolumes
// into a percentage (float).
func (r *VirtualMachine) PercentComplete() (pct float64) {
	for _, dv := range r.DataVolumes {
		pct += dv.PercentComplete()
	}

	pct = pct / float64(len(r.DataVolumes))

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

// changes VM name to match DNS1123 RFC convention.
func changeVmName(currName string, vmID string) string {

	var nameMaxLength int = 63
	var nameExcludeChars = regexp.MustCompile("[^a-z0-9-]")

	newName := strings.ToLower(currName)
	if len(newName) > nameMaxLength {
		newName = newName[0:nameMaxLength]
	}
	if nameExcludeChars.MatchString(newName) {
		newName = nameExcludeChars.ReplaceAllString(newName, "")
	}
	for strings.HasPrefix(newName, "-") {
		newName = newName[1:]
	}
	for strings.HasSuffix(newName, "-") {
		newName = newName[:len(newName)-1]
	}
	if len(newName) == 0 {
		newName = "vm-" + vmID[:4]
	}
	return newName
}
