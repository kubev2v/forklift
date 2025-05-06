package populator

import (
	"fmt"
	"strings"
)

type Populator interface {
	// Populate will populate the volume identified by volumeHanle with the content of
	// the sourceVMDKFile.
	// volumeHanle is the the PVC.Spec.Csi.VolumeHandle field, which by the CSI spec, represents
	// the volume in the storage system and is set by the CSI driver
	Populate(sourceVMDKFile string, volumeHanle string, progress chan int, quit chan error) error
}

// LUN describes the object in the storage system
type LUN struct {
	//Name is the volume name or just name in the storage system
	Name string
	// naa
	NAA string
	// SerialNumber is a representation of the disk. With combination of the
	// vendor ID it should ve globally unique and can be identified by udev, usually
	// under /dev/disk/by-id/ with some prefix or postfix, depending on the udev rule
	// and can also be found by lsblk -o name,serial
	SerialNumber string
	// target's IQN
	IQN string
	// Storage provider ID in hex
	ProviderID string
	// the volume handle as set by the CSI driver field spec.volumeHandle
	VolumeHandle string
	//  Logical device ID of the volume
	LDeviceID string
	// Storage device Serial Number
	StorageSerialNumber string
	// Storage Protocol
	Protocol string
}

// VMDisk is the target VMDisk in vmware
type VMDisk struct {
	VMName     string
	Datacenter string
	VmdkFile   string
	VmnameDir  string
}

func (d *VMDisk) Path() string {
	return fmt.Sprintf("/vmfs/volumes/%s/%s/%s", d.Datacenter, d.VmnameDir, d.VmdkFile)
}

func ParseVmdkPath(vmdkPath string) (VMDisk, error) {
	parts := strings.SplitN(vmdkPath, "] ", 2)
	if len(parts) != 2 {
		return VMDisk{}, fmt.Errorf("Invalid vmdkPath %q, should be '[datastore] vmname/vmname.vmdk'", vmdkPath)
	}
	datastore := strings.TrimPrefix(parts[0], "[")
	pathParts := strings.SplitN(parts[1], "/", 2)

	if len(pathParts) != 2 {
		return VMDisk{}, fmt.Errorf("Invalid vmdkPath %q, should be '[datastore] vmname/vmname.vmdk'", vmdkPath)
	}

	vmname_dir := pathParts[0]
	vmdk := pathParts[1]
	vmdkParts := strings.SplitN(vmdk, ".", 2)
	vmname_sub := vmdkParts[0]
	return VMDisk{VMName: vmname_sub, Datacenter: datastore, VmdkFile: vmdk, VmnameDir: vmname_dir}, nil
}
