package hypervovf

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	KeyName               = "Name"
	KeyHardDrives         = "HardDrives"
	KeyProcessorCount     = "ProcessorCount"
	KeyMemoryStartup      = "MemoryStartup"
	KeyNetworkAdapters    = "NetworkAdapters"
	KeyGuestOSInfo        = "GuestOSInfo"
	KeyPath               = "Path"
	KeyControllerType     = "ControllerType"
	KeyControllerNumber   = "ControllerNumber"
	KeyControllerLocation = "ControllerLocation"
	KeyCaption            = "Caption"
	KeyVersion            = "Version"
	KeyOSArchitecture     = "OSArchitecture"
	ControllerTypeIDE     = "IDE"
	ControllerTypeSCSI    = "SCSI"
)

func RemoveFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

type controllerKey struct {
	Type   string
	Number int
}

type diskInfo struct {
	Path               string
	ControllerType     string
	ControllerNumber   int
	ControllerLocation int
}

func extractDisksWithControllers(vmMap map[string]interface{}) []diskInfo {
	var disks []diskInfo

	hdList, ok := vmMap[KeyHardDrives].([]interface{})
	if !ok {
		if hd, ok := vmMap[KeyHardDrives].(map[string]interface{}); ok {
			hdList = []interface{}{hd}
		} else {
			return disks
		}
	}

	for _, hd := range hdList {
		hdMap, ok := hd.(map[string]interface{})
		if !ok {
			continue
		}

		disk := diskInfo{
			ControllerType:   ControllerTypeIDE,
			ControllerNumber: 0,
		}

		if path, ok := hdMap[KeyPath].(string); ok {
			disk.Path = path
		}
		if ct, ok := hdMap[KeyControllerType].(string); ok {
			disk.ControllerType = ct
		}
		if cn, ok := hdMap[KeyControllerNumber].(float64); ok {
			disk.ControllerNumber = int(cn)
		}
		if cl, ok := hdMap[KeyControllerLocation].(float64); ok {
			disk.ControllerLocation = int(cl)
		}

		if disk.Path != "" {
			disks = append(disks, disk)
		}
	}

	return disks
}

func FormatFromHyperV(vm interface{}, rawDiskPaths []string) error {
	vmMap, ok := vm.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid VM format: expected map[string]interface{}")
	}

	vmName, ok := vmMap[KeyName].(string)
	if !ok || vmName == "" {
		return fmt.Errorf("VM name is required")
	}

	if len(rawDiskPaths) == 0 {
		return fmt.Errorf("at least one disk path is required")
	}

	var (
		files          []File
		ovfDisks       []Disk
		networks       []Network
		hardwareItems  []Item
		itemInstanceID = 1
	)

	cpuCount := int64(1)
	if val, ok := vmMap[KeyProcessorCount].(float64); ok {
		cpuCount = int64(val)
	}
	hardwareItems = append(hardwareItems, Item{
		InstanceID:      strconv.Itoa(itemInstanceID),
		ResourceType:    ResourceTypeProcessor,
		Description:     "Number of virtual CPUs",
		AllocationUnits: "hertz * 10^6",
		ElementName:     fmt.Sprintf("%d virtual CPU(s)", cpuCount),
		VirtualQuantity: cpuCount,
	})
	itemInstanceID++

	memoryMB := int64(1024)
	if val, ok := vmMap[KeyMemoryStartup].(float64); ok {
		memoryMB = int64(val / 1024 / 1024)
	}
	hardwareItems = append(hardwareItems, Item{
		InstanceID:      strconv.Itoa(itemInstanceID),
		ResourceType:    ResourceTypeMemory,
		Description:     "Memory Size",
		AllocationUnits: "byte * 2^20",
		ElementName:     fmt.Sprintf("%dMB of memory", memoryMB),
		VirtualQuantity: memoryMB,
	})
	itemInstanceID++

	diskInfos := extractDisksWithControllers(vmMap)

	rawPathSet := make(map[string]bool)
	for _, p := range rawDiskPaths {
		rawPathSet[strings.ToLower(p)] = true
	}

	controllerIDs := make(map[controllerKey]string)
	for _, disk := range diskInfos {
		key := controllerKey{Type: disk.ControllerType, Number: disk.ControllerNumber}
		if _, exists := controllerIDs[key]; !exists {
			controllerID := strconv.Itoa(itemInstanceID)
			controllerIDs[key] = controllerID

			var resourceType ResourceType
			var elementName string
			var description string

			switch strings.ToUpper(disk.ControllerType) {
			case ControllerTypeSCSI:
				resourceType = ResourceTypeSCSIController
				elementName = fmt.Sprintf("SCSI Controller %d", disk.ControllerNumber)
				description = "SCSI Controller"
			default: // IDE
				resourceType = ResourceTypeIDEController
				elementName = fmt.Sprintf("VirtualIDEController %d", disk.ControllerNumber)
				description = "IDE Controller"
			}

			hardwareItems = append(hardwareItems, Item{
				InstanceID:   controllerID,
				ResourceType: resourceType,
				Address:      strconv.Itoa(disk.ControllerNumber),
				Description:  description,
				ElementName:  elementName,
			})
			itemInstanceID++
		}
	}

	for i, disk := range diskInfos {
		if !rawPathSet[strings.ToLower(disk.Path)] {
			continue
		}

		diskIndex := i + 1
		fileRefID := fmt.Sprintf("file%d", diskIndex)

		fileName := filepath.Base(disk.Path)
		var diskCapacity int64
		virtualSize, err := GetVHDXVirtualSize(disk.Path)
		if err != nil {
			if stat, statErr := os.Stat(disk.Path); statErr == nil {
				diskCapacity = stat.Size()
				fmt.Printf("Warning: Could not read VHDX virtual size for %s: %v, using file size\n", disk.Path, err)
			} else {
				return fmt.Errorf("failed to get size of disk file %s: %w", disk.Path, err)
			}
		} else {
			diskCapacity = int64(virtualSize)
		}

		files = append(files, File{
			ID:   fileRefID,
			Href: fileName,
			Size: diskCapacity,
		})

		diskID := fmt.Sprintf("vmdisk%d", diskIndex)
		ovfDisks = append(ovfDisks, Disk{
			Capacity:                diskCapacity,
			CapacityAllocationUnits: "byte",
			DiskID:                  diskID,
			FileRef:                 fileRefID,
			Format:                  "http://technet.microsoft.com/en-us/library/dd979539.aspx#VHDX",
		})

		key := controllerKey{Type: disk.ControllerType, Number: disk.ControllerNumber}
		parentControllerID := controllerIDs[key]

		hardwareItems = append(hardwareItems, Item{
			InstanceID:      strconv.Itoa(itemInstanceID),
			ResourceType:    ResourceTypeHardDisk,
			ElementName:     fmt.Sprintf("Hard Disk %d", diskIndex),
			Description:     "Hard Disk",
			HostResource:    fmt.Sprintf("ovf:/disk/%s", diskID),
			Parent:          parentControllerID,
			AddressOnParent: strconv.Itoa(disk.ControllerLocation),
		})
		itemInstanceID++
	}

	if adapters, ok := vmMap[KeyNetworkAdapters].([]interface{}); ok {
		for i, a := range adapters {
			adapter, ok := a.(map[string]interface{})
			if !ok {
				continue
			}

			networkIndex := i + 1
			networkName := fmt.Sprintf("VM Network %d", networkIndex)
			if n, ok := adapter[KeyName].(string); ok && n != "" {
				networkName = n
			}

			networks = append(networks, Network{
				Name:        networkName,
				Description: fmt.Sprintf("Network interface %d", networkIndex),
			})

			autoAlloc := true
			hardwareItems = append(hardwareItems, Item{
				InstanceID:          strconv.Itoa(itemInstanceID),
				ResourceType:        ResourceTypeEthernetAdapter,
				ResourceSubType:     "E1000",
				ElementName:         fmt.Sprintf("Ethernet %d", networkIndex),
				Description:         fmt.Sprintf("E1000 ethernet adapter on \"%s\"", networkName),
				Connection:          networkName,
				AutomaticAllocation: &autoAlloc,
			})
			itemInstanceID++
		}
	}

	var guestOSInfo GuestOSInfo
	if guestMap, ok := vmMap[KeyGuestOSInfo].(map[string]interface{}); ok {
		if caption, ok := guestMap[KeyCaption].(string); ok {
			guestOSInfo.Caption = caption
		}
		if version, ok := guestMap[KeyVersion].(string); ok {
			guestOSInfo.Version = version
		}
		if arch, ok := guestMap[KeyOSArchitecture].(string); ok {
			guestOSInfo.OSArchitecture = arch
		}
	}

	osType := MapCaptionToOsType(guestOSInfo.Caption, guestOSInfo.OSArchitecture)
	description := fmt.Sprintf("%s (%s)", guestOSInfo.Caption, guestOSInfo.OSArchitecture)

	env := &Envelope{
		Xmlns: "http://schemas.dmtf.org/ovf/envelope/1",
		Cim:   "http://schemas.dmtf.org/wbem/wscim/1/common",
		Ovf:   "http://schemas.dmtf.org/ovf/envelope/1",
		Rasd:  "http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_ResourceAllocationSettingData",
		Vmw:   "http://www.vmware.com/schema/ovf", // vmw:osType is the de facto standard for OS type in OVF
		Vssd:  "http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_VirtualSystemSettingData",
		Xsi:   "http://www.w3.org/2001/XMLSchema-instance",

		References: References{Files: files},
		DiskSection: DiskSection{
			Info:  "List of the virtual disks",
			Disks: ovfDisks,
		},
		NetworkSection: NetworkSection{
			Info:     "The list of logical networks",
			Networks: networks,
		},
		VirtualSystem: VirtualSystem{
			ID:   vmName,
			Info: "A Virtual system",
			Name: vmName,
			OperatingSystem: OperatingSystemSection{
				ID:          GetOVFOperatingSystemID(osType),
				OsType:      osType,
				Info:        "The operating system installed",
				Description: description,
			},
			VirtualHardware: VirtualHardwareSection{
				Info: "Virtual hardware requirements",
				System: System{
					ElementName:             "Virtual Hardware Family",
					InstanceID:              0,
					VirtualSystemIdentifier: vmName,
					VirtualSystemType:       "vmx-07",
				},
				Items: hardwareItems,
			},
		},
	}

	ovfBytes, err := MarshalOvf(env)
	if err != nil {
		return fmt.Errorf("failed to marshal OVF: %w", err)
	}

	var basePath string
	if len(rawDiskPaths) > 0 {
		basePath = rawDiskPaths[0]
	} else {
		basePath = vmName + ".vhdx"
	}
	ovfPath := RemoveFileExtension(basePath) + ".ovf"
	if err := os.WriteFile(ovfPath, ovfBytes, 0644); err != nil {
		return fmt.Errorf("failed to write OVF file: %w", err)
	}
	fmt.Println("OVF file written to:", ovfPath)

	return nil
}

func MarshalOvf(env *Envelope) ([]byte, error) {
	body, err := xml.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, err
	}
	return []byte(xmlHeader + string(body)), nil
}
