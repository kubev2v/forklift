package vsphere

import (
	"sort"
	"strings"

	vimtypes "github.com/vmware/govmomi/vim25/types"
)

var libvirtBusOrder = []string{"scsi", "sata", "ide", "nvme"}

type diskEntry struct {
	path          string
	bus           string
	controllerKey int32
	unitNumber    int32
	deviceKey     int32
}

func disksFromDevices(devices []vimtypes.BaseVirtualDevice) []string {
	controllerBus := map[int32]string{}
	for _, dev := range devices {
		key := dev.GetVirtualDevice().Key
		if bus := controllerType(dev); bus != "" {
			controllerBus[key] = bus
		}
	}

	var entries []diskEntry
	for _, dev := range devices {
		disk, ok := dev.(*vimtypes.VirtualDisk)
		if !ok {
			continue
		}
		path := diskFilePath(disk)
		if path == "" {
			continue
		}
		vd := disk.GetVirtualDevice()
		bus := ""
		if vd.ControllerKey != 0 {
			bus = controllerBus[vd.ControllerKey]
		}
		unit := int32(-1)
		if vd.UnitNumber != nil {
			unit = *vd.UnitNumber
		}
		entries = append(entries, diskEntry{
			path:          path,
			bus:           bus,
			controllerKey: vd.ControllerKey,
			unitNumber:    unit,
			deviceKey:     vd.Key,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if busRank(a.bus) != busRank(b.bus) {
			return busRank(a.bus) < busRank(b.bus)
		}
		if a.controllerKey != b.controllerKey {
			return a.controllerKey < b.controllerKey
		}
		if a.unitNumber != b.unitNumber {
			return a.unitNumber < b.unitNumber
		}
		return a.deviceKey < b.deviceKey
	})

	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.path)
	}
	return paths
}

func controllerType(dev vimtypes.BaseVirtualDevice) string {
	switch dev.(type) {
	case *vimtypes.VirtualLsiLogicController,
		*vimtypes.ParaVirtualSCSIController,
		*vimtypes.VirtualBusLogicController,
		*vimtypes.VirtualSCSIController:
		return "scsi"
	case *vimtypes.VirtualSATAController:
		return "sata"
	case *vimtypes.VirtualIDEController:
		return "ide"
	case *vimtypes.VirtualNVMEController:
		return "nvme"
	default:
		return ""
	}
}

func busRank(bus string) int {
	for i, name := range libvirtBusOrder {
		if bus == name {
			return i
		}
	}
	return len(libvirtBusOrder)
}

func diskFilePath(disk *vimtypes.VirtualDisk) string {
	switch backing := disk.Backing.(type) {
	case *vimtypes.VirtualDiskFlatVer2BackingInfo:
		return baseDiskPath(backing)
	case *vimtypes.VirtualDiskSparseVer2BackingInfo:
		return baseDiskPathSparse(backing)
	case *vimtypes.VirtualDiskSeSparseBackingInfo:
		if backing.FileName != "" {
			return backing.FileName
		}
	case *vimtypes.VirtualDiskRawDiskMappingVer1BackingInfo:
		return ""
	}
	return ""
}

func baseDiskPath(backing *vimtypes.VirtualDiskFlatVer2BackingInfo) string {
	if backing == nil {
		return ""
	}
	current := backing.FileName
	if backing.Parent == nil {
		return trimDeltaSuffix(current)
	}
	parent := backing.Parent
	for parent.Parent != nil {
		parent = parent.Parent
	}
	if parent.FileName != "" {
		return parent.FileName
	}
	return trimDeltaSuffix(current)
}

func baseDiskPathSparse(backing *vimtypes.VirtualDiskSparseVer2BackingInfo) string {
	if backing == nil {
		return ""
	}
	current := backing.FileName
	if backing.Parent == nil {
		return trimDeltaSuffix(current)
	}
	parent := backing.Parent
	for parent.Parent != nil {
		parent = parent.Parent
	}
	if parent.FileName != "" {
		return parent.FileName
	}
	return trimDeltaSuffix(current)
}

func trimDeltaSuffix(path string) string {
	const suffix = ".vmdk"
	if !strings.HasSuffix(path, suffix) {
		return path
	}
	prefix := path[:len(path)-len(suffix)]
	if len(prefix) >= 7 && prefix[len(prefix)-7] == '-' && isSixDigits(prefix[len(prefix)-6:]) {
		return prefix[:len(prefix)-7] + suffix
	}
	return path
}

func isSixDigits(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < 6; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
