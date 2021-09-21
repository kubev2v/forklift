package plan

import (
	"context"
	"encoding/xml"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/client-go/api/v1"
	libvirtxml "libvirt.org/libvirt-go-xml"
	"path"
	"strconv"
	"strings"

	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Annotations
const (
	// transfer network annotation (value=network-attachment-definition name)
	annDefaultNetwork = "v1.multus-cni.io/default-network"
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

//
// Map of VirtualMachines keyed by vmID.
type VirtualMachineMap map[string]VirtualMachine

//
// Represents kubevirt.
type KubeVirt struct {
	*plancontext.Context
	// Builder
	Builder adapter.Builder
}

//
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

//
// List VirtualMachine CRs.
// Each VirtualMachine represents an imported kubevirt VM with associated DataVolumes.
func (r *KubeVirt) ListVMs() ([]VirtualMachine, error) {
	vList := &cnv.VirtualMachineList{}
	err := r.Destination.Client.List(
		context.TODO(),
		vList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.planLabels()),
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
		&client.ListOptions{
			Namespace: r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for i := range list {
		vm := &list[i]
		for i := range dvList.Items {
			dv := &dvList.Items[i]
			if vm.Owner(dv) {
				vm.DataVolumes = append(
					vm.DataVolumes,
					DataVolume{
						DataVolume: dv,
					})
			}
		}
	}

	return list, nil
}

//
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

//
// Get the importer pod for a DataVolume.
func (r *KubeVirt) GetImporterPod(dv DataVolume) (pod *core.Pod, err error) {
	pod = &core.Pod{}
	name := fmt.Sprintf("importer-%s", dv.Name)
	err = r.Destination.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      name,
			Namespace: r.Plan.Spec.TargetNamespace,
		},
		pod,
	)
	if err != nil {
		err = liberr.Wrap(err)
	}
	return
}

//
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
	for _, dv := range dvs.Items {
		ownerRefs := dv.GetOwnerReferences()
		if ownerRefs == nil {
			ownerRefs = []meta.OwnerReference{}
		}
		ownerRefs = append(ownerRefs, vmOwnerReference(virtualMachine))
		updated := dv.DeepCopy()
		updated.SetOwnerReferences(ownerRefs)
		original := client.MergeFrom(&dv)
		err = r.Destination.Client.Patch(context.TODO(), updated, original)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

//
// Delete the VirtualMachine CR on the destination cluster.
func (r *KubeVirt) DeleteVM(vm *plan.VMStatus) (err error) {
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

//
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

//
// Ensure the DataVolumes exist on the destination.
func (r *KubeVirt) EnsureDataVolumes(vm *plan.VMStatus) (err error) {
	secret, err := r.ensureSecret(vm.Ref)
	if err != nil {
		return err
	}
	configMap, err := r.ensureConfigMap(vm.Ref)
	if err != nil {
		return
	}

	dataVolumes, err := r.dataVolumes(vm, secret, configMap)
	if err != nil {
		return
	}

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

func (r *KubeVirt) SetDataVolumeCheckpoint(vm *plan.VMStatus, final bool) (err error) {
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
	for _, dv := range list.Items {
		if len(dv.Spec.Checkpoints) >= len(vm.Warm.Precopies) {
			continue
		}
		n := len(vm.Warm.Precopies)
		dv.Spec.Checkpoints = append(dv.Spec.Checkpoints, cdi.DataVolumeCheckpoint{
			Current:  vm.Warm.Precopies[n-1].Snapshot,
			Previous: vm.Warm.Precopies[n-2].Snapshot,
		})
		dv.Spec.FinalCheckpoint = final
		err = r.Destination.Client.Update(context.TODO(), &dv)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

//
// Ensure the guest conversion (virt-v2v) pod exists on the destination.
func (r *KubeVirt) EnsureGuestConversionPod(vm *plan.VMStatus, vmCr *VirtualMachine) (err error) {
	configMap, err := r.ensureLibvirtConfigMap(vm.Ref, vmCr)
	if err != nil {
		return
	}

	newPod, err := r.guestConversionPod(vm, vmCr, configMap)
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

//
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

//
// Delete the guest conversion pod on the destination cluster.
func (r *KubeVirt) DeleteGuestConversionPod(vm *plan.VMStatus) (err error) {
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

//
// Build the DataVolume CRs.
func (r *KubeVirt) dataVolumes(vm *plan.VMStatus, secret *core.Secret, configMap *core.ConfigMap) (objects []cdi.DataVolume, err error) {
	_, err = r.Source.Inventory.VM(&vm.Ref)
	if err != nil {
		return
	}

	dataVolumes, err := r.Builder.DataVolumes(vm.Ref, secret, configMap)
	if err != nil {
		return
	}

	for i := range dataVolumes {
		annotations := make(map[string]string)
		if r.Plan.Spec.TransferNetwork != nil {
			annotations[annDefaultNetwork] = path.Join(
				r.Plan.Spec.TransferNetwork.Namespace, r.Plan.Spec.TransferNetwork.Name)
		}
		dv := cdi.DataVolume{
			ObjectMeta: meta.ObjectMeta{
				Namespace:   r.Plan.Spec.TargetNamespace,
				Annotations: annotations,
				GenerateName: strings.Join(
					[]string{
						r.Plan.Name,
						vm.ID},
					"-") + "-",
			},
			Spec: dataVolumes[i],
		}
		dv.Labels = r.vmLabels(vm.Ref)
		if vm.Warm != nil {
			dv.Spec.Checkpoints = []cdi.DataVolumeCheckpoint{
				{Previous: "", Current: vm.Warm.Precopies[0].Snapshot},
			}
		}
		objects = append(objects, dv)
	}

	return
}

//
// Build the Kubevirt VM CR.
func (r *KubeVirt) virtualMachine(vm *plan.VMStatus) (object *cnv.VirtualMachine, err error) {
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

	running := false
	object = &cnv.VirtualMachine{
		TypeMeta: meta.TypeMeta{
			APIVersion: "v1",
			Kind:       "VirtualMachine",
		},
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Spec.TargetNamespace,
			Labels:    r.vmLabels(vm.Ref),
			Name:      vm.Name,
		},
		Spec: cnv.VirtualMachineSpec{
			Running: &running,
		},
	}
	err = r.Builder.VirtualMachine(vm.Ref, &object.Spec, list.Items)
	if err != nil {
		return
	}

	return
}

func (r *KubeVirt) guestConversionPod(vm *plan.VMStatus, vmCr *VirtualMachine, configMap *core.ConfigMap) (pod *core.Pod, err error) {
	volumes, volumeMounts, volumeDevices := r.podVolumeMounts(vmCr, configMap)

	// qemu group
	fsGroup := int64(107)
	pod = &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.Plan.Spec.TargetNamespace,
			Labels:    r.vmLabels(vm.Ref),
			GenerateName: strings.Join(
				[]string{
					r.Plan.Name,
					vm.ID},
				"-") + "-",
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				FSGroup: &fsGroup,
			},
			RestartPolicy: core.RestartPolicyNever,
			Containers: []core.Container{
				{
					Name:            "virt-v2v",
					Image:           Settings.Migration.VirtV2vImage,
					VolumeMounts:    volumeMounts,
					VolumeDevices:   volumeDevices,
					ImagePullPolicy: core.PullIfNotPresent,
					// Request access to /dev/kvm via Kubevirt's Device Manager
					Resources: core.ResourceRequirements{
						Limits: core.ResourceList{
							"devices.kubevirt.io/kvm": resource.MustParse("1"),
						},
					},
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

func (r *KubeVirt) podVolumeMounts(vmCr *VirtualMachine, configMap *core.ConfigMap) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice) {
	dvsByName := make(map[string]DataVolume)
	for _, dv := range vmCr.DataVolumes {
		dvsByName[dv.Name] = dv
	}

	for i, v := range vmCr.Spec.Template.Spec.Volumes {
		dv, _ := dvsByName[v.DataVolume.Name]
		vol := core.Volume{
			Name: dv.Name,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: dv.Name,
					ReadOnly:  false,
				},
			},
		}
		volumes = append(volumes, vol)
		if *dv.Spec.Storage.VolumeMode == core.PersistentVolumeBlock {
			devices = append(devices, core.VolumeDevice{
				Name:       dv.Name,
				DevicePath: fmt.Sprintf("/dev/block%v", i),
			})
		} else {
			mounts = append(mounts, core.VolumeMount{
				Name:      dv.Name,
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
	mounts = append(mounts, core.VolumeMount{
		Name:      "libvirt-domain-xml",
		MountPath: "/mnt/v2v",
	})

	return
}

func (r *KubeVirt) libvirtDomain(vmCr *VirtualMachine) (domain *libvirtxml.Domain) {
	dvsByName := make(map[string]DataVolume)
	for _, dv := range vmCr.DataVolumes {
		dvsByName[dv.Name] = dv
	}

	// virt-v2v needs a very minimal libvirt domain XML file to be provided
	// with the locations of each of the disks on the VM that is to be converted.
	libvirtDisks := make([]libvirtxml.DomainDisk, 0)
	for i, vol := range vmCr.Spec.Template.Spec.Volumes {
		diskSource := libvirtxml.DomainDiskSource{}

		dv := dvsByName[vol.DataVolume.Name]
		if *dv.Spec.Storage.VolumeMode == core.PersistentVolumeBlock {
			diskSource.Block = &libvirtxml.DomainDiskSourceBlock{
				Dev: fmt.Sprintf("/dev/block%v", i),
			}
		} else {
			diskSource.File = &libvirtxml.DomainDiskSourceFile{
				// the location where the disk images will be found on
				// the virt-v2v pod. See also makePodVolumeMounts.
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

//
// Ensure the config map exists on the destination.
func (r *KubeVirt) ensureConfigMap(vmRef ref.Ref) (configMap *core.ConfigMap, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		return
	}
	newConfigMap, err := r.configMap(vmRef)
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
		configMap = newConfigMap
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

//
// Ensure the Libvirt domain config map exists on the destination.
func (r *KubeVirt) ensureLibvirtConfigMap(vmRef ref.Ref, vmCr *VirtualMachine) (configMap *core.ConfigMap, err error) {
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

	domain := r.libvirtDomain(vmCr)
	domainXML, err := xml.Marshal(domain)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(list.Items) > 0 {
		configMap = &list.Items[0]
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
	} else {
		configMap, err = r.configMap(vmRef)
		if err != nil {
			return
		}
		configMap.BinaryData["input.xml"] = domainXML
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

//
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

//
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

//
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

//
// Labels for plan and migration.
func (r *KubeVirt) planLabels() map[string]string {
	return map[string]string{
		kMigration: string(r.Migration.UID),
		kPlan:      string(r.Plan.GetUID()),
	}
}

//
// Labels for a VM on a plan.
func (r *KubeVirt) vmLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.planLabels()
	labels[kVM] = vmRef.ID
	return
}

//
// Represents a CDI DataVolume and add behavior.
type DataVolume struct {
	*cdi.DataVolume
}

//
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

//
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

//
// Represents Kubevirt VirtualMachine with associated DataVolumes.
type VirtualMachine struct {
	*cnv.VirtualMachine
	DataVolumes []DataVolume
}

//
// Determine if `this` VirtualMachine is the
// owner of the CDI DataVolume.
func (r *VirtualMachine) Owner(dv *cdi.DataVolume) bool {
	for _, vol := range r.Spec.Template.Spec.Volumes {
		if vol.DataVolume.Name == dv.Name {
			return true
		}
	}

	return false
}

//
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

//
// Convert the combined progress of all DataVolumes
// into a percentage (float).
func (r *VirtualMachine) PercentComplete() (pct float64) {
	for _, dv := range r.DataVolumes {
		pct += dv.PercentComplete()
	}

	pct = pct / float64(len(r.DataVolumes))

	return
}

//
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
