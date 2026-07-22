package builder

import (
	"fmt"
	"path"
	"sort"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	"github.com/kubev2v/forklift/pkg/provider/azure"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	cnv "kubevirt.io/api/core/v1"
)

const (
	Pod     = "pod"
	Multus  = "multus"
	Ignored = "ignored"
)

const (
	Tablet = "tablet"
)

const (
	Virtio = "virtio"
	E1000e = "e1000e"
)

func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vmRef)
	if err != nil {
		return err
	}

	vmSize := ""
	if azureVM.Properties != nil && azureVM.Properties.HardwareProfile != nil && azureVM.Properties.HardwareProfile.VMSize != nil {
		vmSize = string(*azureVM.Properties.HardwareProfile.VMSize)
	}
	if vmSize == "" {
		vmSize = "Standard_D2s_v3"
		r.log.Info("VM size not found, using default", "vm", vmRef.Name, "default", vmSize)
	}

	vcpus, memoryMiB := r.mapVMSize(vmSize)

	object.Template = &cnv.VirtualMachineInstanceTemplateSpec{
		Spec: cnv.VirtualMachineInstanceSpec{
			Domain: cnv.DomainSpec{
				Resources: cnv.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memoryMiB)),
					},
				},
				Devices: cnv.Devices{
					Disks:      []cnv.Disk{},
					Interfaces: []cnv.Interface{},
					Inputs:     []cnv.Input{},
				},
			},
			Networks: []cnv.Network{},
			Volumes:  []cnv.Volume{},
		},
	}

	r.mapCPU(vcpus, object)
	r.mapFirmware(azureVM, object)
	r.mapInput(object)
	r.mapDisks(azureVM, persistentVolumeClaims, object)

	err = r.mapNetworks(azureVM, object)
	if err != nil {
		return err
	}

	return nil
}

func (r *Builder) mapCPU(vcpus int32, object *cnv.VirtualMachineSpec) {
	object.Template.Spec.Domain.CPU = &cnv.CPU{
		Sockets: 1,
		Cores:   uint32(vcpus),
	}
}

func (r *Builder) mapFirmware(azureVM *inventory.VMDetails, object *cnv.VirtualMachineSpec) {
	serial := azureVM.ID

	firmware := &cnv.Firmware{
		Serial: serial,
	}

	isGen2 := false
	if azureVM.Properties != nil && azureVM.Properties.StorageProfile != nil &&
		azureVM.Properties.StorageProfile.OSDisk != nil {
		// Gen2 VMs use UEFI
		if azureVM.Properties.SecurityProfile != nil &&
			azureVM.Properties.SecurityProfile.UefiSettings != nil {
			isGen2 = true
		}
	}

	if isGen2 {
		firmware.Bootloader = &cnv.Bootloader{
			EFI: &cnv.EFI{
				SecureBoot: ptr.To(false),
			},
		}
		object.Template.Spec.Domain.Features = &cnv.Features{
			ACPI: cnv.FeatureState{},
			SMM:  &cnv.FeatureState{Enabled: ptr.To(true)},
		}
	} else {
		firmware.Bootloader = &cnv.Bootloader{
			BIOS: &cnv.BIOS{},
		}
		object.Template.Spec.Domain.Features = &cnv.Features{
			ACPI: cnv.FeatureState{},
		}
	}

	object.Template.Spec.Domain.Firmware = firmware
}

func (r *Builder) useCompatibilityMode() bool {
	return r.Plan.Spec.SkipGuestConversion && r.Plan.Spec.UseCompatibilityMode
}

func (r *Builder) mapInput(object *cnv.VirtualMachineSpec) {
	bus := cnv.InputBusVirtio
	if r.useCompatibilityMode() {
		bus = cnv.InputBusUSB
	}
	tablet := cnv.Input{
		Type: Tablet,
		Name: Tablet,
		Bus:  bus,
	}
	object.Template.Spec.Domain.Devices.Inputs = []cnv.Input{tablet}
}

func (r *Builder) mapDisks(azureVM *inventory.VMDetails, persistentVolumeClaims []*core.PersistentVolumeClaim, object *cnv.VirtualMachineSpec) {
	pvcByIndex := make(map[string]*core.PersistentVolumeClaim)
	for _, pvc := range persistentVolumeClaims {
		if idx, ok := pvc.Labels[azure.LabelDiskIndex]; ok {
			pvcByIndex[idx] = pvc
		}
	}

	bus := cnv.DiskBusVirtio
	if r.useCompatibilityMode() {
		bus = cnv.DiskBusSATA
	}

	diskIndex := 0

	// OS disk first
	if azureVM.Properties != nil && azureVM.Properties.StorageProfile != nil &&
		azureVM.Properties.StorageProfile.OSDisk != nil {
		pvc := pvcByIndex[fmt.Sprintf("%d", diskIndex)]
		if pvc != nil {
			diskName := fmt.Sprintf("disk-%d", diskIndex)
			disk := cnv.Disk{
				Name: diskName,
				DiskDevice: cnv.DiskDevice{
					Disk: &cnv.DiskTarget{
						Bus: bus,
					},
				},
				BootOrder: ptr.To(uint(1)),
			}
			object.Template.Spec.Domain.Devices.Disks = append(object.Template.Spec.Domain.Devices.Disks, disk)
			object.Template.Spec.Volumes = append(object.Template.Spec.Volumes, cnv.Volume{
				Name: diskName,
				VolumeSource: cnv.VolumeSource{
					PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			})
			diskIndex++
		}
	}

	// Data disks sorted by LUN
	if azureVM.Properties != nil && azureVM.Properties.StorageProfile != nil {
		dataDisks := azureVM.Properties.StorageProfile.DataDisks
		sort.Slice(dataDisks, func(i, j int) bool {
			luni := int32(0)
			lunj := int32(0)
			if dataDisks[i].Lun != nil {
				luni = *dataDisks[i].Lun
			}
			if dataDisks[j].Lun != nil {
				lunj = *dataDisks[j].Lun
			}
			return luni < lunj
		})

		for range dataDisks {
			pvc := pvcByIndex[fmt.Sprintf("%d", diskIndex)]
			if pvc != nil {
				diskName := fmt.Sprintf("disk-%d", diskIndex)
				disk := cnv.Disk{
					Name: diskName,
					DiskDevice: cnv.DiskDevice{
						Disk: &cnv.DiskTarget{
							Bus: bus,
						},
					},
				}
				object.Template.Spec.Domain.Devices.Disks = append(object.Template.Spec.Domain.Devices.Disks, disk)
				object.Template.Spec.Volumes = append(object.Template.Spec.Volumes, cnv.Volume{
					Name: diskName,
					VolumeSource: cnv.VolumeSource{
						PersistentVolumeClaim: &cnv.PersistentVolumeClaimVolumeSource{
							PersistentVolumeClaimVolumeSource: core.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				})
			}
			diskIndex++
		}
	}
}

func (r *Builder) mapNetworks(azureVM *inventory.VMDetails, object *cnv.VirtualMachineSpec) error {
	var kNetworks []cnv.Network
	var kInterfaces []cnv.Interface

	hasUDN := r.Plan.DestinationHasUdnNetwork(r.Destination)
	interfaceModel := Virtio
	if r.useCompatibilityMode() {
		interfaceModel = E1000e
	}

	nics := inventory.GetNetworkInterfaceIDs(azureVM)
	if len(nics) == 0 {
		kNetwork := cnv.Network{
			Name: "default",
			NetworkSource: cnv.NetworkSource{
				Pod: &cnv.PodNetwork{},
			},
		}
		kInterface := cnv.Interface{
			Name:  "default",
			Model: interfaceModel,
		}
		if hasUDN {
			kInterface.Binding = &cnv.PluginBinding{
				Name: planbase.UdnL2bridge,
			}
		} else {
			kInterface.InterfaceBindingMethod = cnv.InterfaceBindingMethod{
				Masquerade: &cnv.InterfaceMasquerade{},
			}
		}
		kNetworks = append(kNetworks, kNetwork)
		kInterfaces = append(kInterfaces, kInterface)
	} else {
		pool := planbase.NewNADPool()
		nicKeys, pairsBySource := r.buildNICResolver(nics)
		networkIndex := 0

		for i := range nics {
			var mapped *api.NetworkPair
			if pair, allocated := planbase.AllocateNetwork(pool, pairsBySource[nicKeys[i]]); allocated {
				mapped = &pair
			}

			if mapped != nil && mapped.Destination.Type == Ignored {
				continue
			}

			networkName := fmt.Sprintf("net-%d", networkIndex)
			kNetwork := cnv.Network{Name: networkName}
			kInterface := cnv.Interface{
				Name:  networkName,
				Model: interfaceModel,
			}

			if !hasUDN || settings.Settings.UdnSupportsMac {
				kInterface.MacAddress = "" // Azure NICs don't expose MAC in the VM model
			}

			if mapped == nil || mapped.Destination.Type == Pod {
				kNetwork.Pod = &cnv.PodNetwork{}
				if hasUDN {
					kInterface.Binding = &cnv.PluginBinding{
						Name: planbase.UdnL2bridge,
					}
				} else {
					kInterface.InterfaceBindingMethod = cnv.InterfaceBindingMethod{
						Masquerade: &cnv.InterfaceMasquerade{},
					}
				}
			} else if mapped.Destination.Type == Multus {
				kNetwork.Multus = &cnv.MultusNetwork{
					NetworkName: path.Join(mapped.Destination.Namespace, mapped.Destination.Name),
				}
				kInterface.InterfaceBindingMethod = cnv.InterfaceBindingMethod{
					Bridge: &cnv.InterfaceBridge{},
				}
			}

			kNetworks = append(kNetworks, kNetwork)
			kInterfaces = append(kInterfaces, kInterface)
			networkIndex++
		}
	}

	object.Template.Spec.Networks = kNetworks
	object.Template.Spec.Domain.Devices.Interfaces = kInterfaces

	return nil
}

func (r *Builder) buildNICResolver(nicIDs []string) ([]string, map[string][]api.NetworkPair) {
	pairsBySource := map[string][]api.NetworkPair{}
	if r.Map.Network != nil {
		for _, pair := range r.Map.Network.Spec.Map {
			if pair.Source.ID != "" {
				pairsBySource[pair.Source.ID] = append(pairsBySource[pair.Source.ID], pair)
			}
			if pair.Source.Name != "" && pair.Source.Name != pair.Source.ID {
				pairsBySource[pair.Source.Name] = append(pairsBySource[pair.Source.Name], pair)
			}
		}
	}
	return nicIDs, pairsBySource
}

func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}

func (r *Builder) NetAppShiftPVCs(vmRef ref.Ref, labels map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(vmRef ref.Ref, tagMapping *api.TagMapping) (map[string]string, map[string]string, map[string]string, error) {
	annotations := map[string]string{
		azure.AnnSourceID: r.vmARMID(vmRef.Name),
	}
	return nil, annotations, nil, nil
}
