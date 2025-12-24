package inventory

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/kubev2v/forklift/cmd/provider-common/ovf"
)

var vmIDmap *UUIDMap
var diskIDMap *UUIDMap
var networkIDMap *UUIDMap

func init() {
	vmIDmap = NewUUIDMap()
	diskIDMap = NewUUIDMap()
	networkIDMap = NewUUIDMap()
}

// ResourceTypes
const (
	ResourceTypeProcessor       = 3
	ResourceTypeMemory          = 4
	ResourceTypeEthernetAdapter = 10
	ResourceTypeHardDiskDrive   = 17
)

func ConvertToVmStruct(envelope []ovf.Envelope, ovaPath []string) []ovf.VM {
	var vms []ovf.VM

	for i := 0; i < len(envelope); i++ {
		vmXml := envelope[i]
		for _, virtualSystem := range vmXml.VirtualSystem {

			// Initialize a new VM
			newVM := ovf.VM{
				OvfPath:      ovaPath[i],
				ExportSource: ovf.GuessSource(vmXml),
				Name:         virtualSystem.Name,
				OsType:       virtualSystem.OperatingSystemSection.OsType,
			}

			for _, item := range virtualSystem.HardwareSection.Items {
				switch item.ResourceType {
				case ResourceTypeProcessor:
					newVM.CpuCount = item.VirtualQuantity
					newVM.CpuUnits = item.AllocationUnits
					if item.CoresPerSocket != "" {
						num, err := strconv.ParseInt(item.CoresPerSocket, 10, 32)
						if err != nil {
							newVM.CoresPerSocket = 1
						} else {
							newVM.CoresPerSocket = int32(num)
						}
					}
				case ResourceTypeMemory:
					newVM.MemoryMB = item.VirtualQuantity
					newVM.MemoryUnits = item.AllocationUnits
				case ResourceTypeEthernetAdapter:
					newVM.NICs = append(newVM.NICs, ovf.NIC{
						Name:    item.ElementName,
						MAC:     item.Address,
						Network: item.Connection,
					})
				default:
					var itemKind string
					if len(item.ElementName) > 0 {
						// if the `ElementName` element has a name such as "Hard Disk 1", strip off the
						// number suffix to try to get a more generic name for the device type
						itemKind = strings.TrimRightFunc(item.ElementName, func(r rune) bool {
							return unicode.IsDigit(r) || unicode.IsSpace(r)
						})
					} else {
						// Some .ova files do not include an `ElementName` element for each device. Fall
						// back to using the `Description` element
						itemKind = item.Description
					}
					if len(itemKind) == 0 {
						itemKind = "Unknown"
					}
					newVM.Devices = append(newVM.Devices, ovf.Device{
						Kind: itemKind,
					})
				}
			}

			for j, disk := range vmXml.DiskSection.Disks {
				name := envelope[i].References.File[j].Href
				newVM.Disks = append(newVM.Disks, ovf.VmDisk{
					FilePath:                GetDiskPath(ovaPath[i]),
					Capacity:                disk.Capacity,
					CapacityAllocationUnits: disk.CapacityAllocationUnits,
					DiskId:                  disk.DiskId,
					FileRef:                 disk.FileRef,
					Format:                  disk.Format,
					PopulatedSize:           disk.PopulatedSize,
					Name:                    name,
				})
				newVM.Disks[j].ID = diskIDMap.GetUUID(newVM.Disks[j], ovaPath[i]+"/"+name)

			}

			for _, network := range vmXml.NetworkSection.Networks {
				newVM.Networks = append(newVM.Networks, ovf.VmNetwork{
					Name:        network.Name,
					Description: network.Description,
					ID:          networkIDMap.GetUUID(network.Name, network.Name),
				})
			}

			newVM.ApplyVirtualConfig(virtualSystem.HardwareSection.Configs)
			newVM.ApplyExtraVirtualConfig(virtualSystem.HardwareSection.ExtraConfig)

			var id string
			if isValidUUID(virtualSystem.ID) {
				id = virtualSystem.ID
			} else {
				id = vmIDmap.GetUUID(newVM, ovaPath[i])
			}
			newVM.UUID = id

			vms = append(vms, newVM)
		}
	}
	return vms
}

func ConvertToNetworkStruct(envelopes []ovf.Envelope) []ovf.VmNetwork {
	var networks []ovf.VmNetwork
	for _, envelope := range envelopes {
		for _, network := range envelope.NetworkSection.Networks {
			newNetwork := ovf.VmNetwork{
				Name:        network.Name,
				Description: network.Description,
				ID:          networkIDMap.GetUUID(network.Name, network.Name),
			}
			networks = append(networks, newNetwork)
		}
	}

	return networks
}

func ConvertToDiskStruct(envelopes []ovf.Envelope, ovaPath []string) []ovf.VmDisk {
	var disks []ovf.VmDisk
	for i, envelope := range envelopes {
		for j, disk := range envelope.DiskSection.Disks {
			name := envelope.References.File[j].Href
			newDisk := ovf.VmDisk{
				FilePath:                GetDiskPath(ovaPath[i]),
				Capacity:                disk.Capacity,
				CapacityAllocationUnits: disk.CapacityAllocationUnits,
				DiskId:                  disk.DiskId,
				FileRef:                 disk.FileRef,
				Format:                  disk.Format,
				PopulatedSize:           disk.PopulatedSize,
				Name:                    name,
			}
			newDisk.ID = diskIDMap.GetUUID(newDisk, ovaPath[i]+"/"+name)
			disks = append(disks, newDisk)
		}
	}

	return disks
}
