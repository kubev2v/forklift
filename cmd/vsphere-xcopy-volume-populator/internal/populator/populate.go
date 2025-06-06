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
	Populate(sourceVMDKFile string, volumeHanle string, progress chan<- uint, quit chan error) error
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
	// Logical device ID of the volume
	LDeviceID string
	// Storage device Serial Number
	StorageSerialNumber string
	// Storage Protocol
	Protocol string
}

// VMDisk is the target VMDisk in vmware
type VMDisk struct {
	VMName    string
	Datastore string
	VmdkFile  string
	VmnameDir string
}

func (d *VMDisk) Path() string {
	return fmt.Sprintf("/vmfs/volumes/%s/%s/%s", d.Datastore, d.VmnameDir, d.VmdkFile)
}

func ParseVmdkPath(vmdkPath string) (VMDisk, error) {
	if vmdkPath == "" {
		return VMDisk{}, fmt.Errorf("vmdkPath cannot be empty")
	}

	parts := strings.SplitN(vmdkPath, "] ", 2)
	if len(parts) != 2 {
		return VMDisk{}, fmt.Errorf("invalid vmdkPath %q: missing closing bracket and space after datastore, expected '[datastore] vmname/vmname.vmdk'", vmdkPath)
	}

	datastore := strings.TrimPrefix(parts[0], "[")
	if datastore == "" {
		return VMDisk{}, fmt.Errorf("invalid vmdkPath %q: datastore name cannot be empty", vmdkPath)
	}

	pathAndFile := parts[1]
	pathParts := strings.SplitN(pathAndFile, "/", 2)

	if len(pathParts) != 2 {
		return VMDisk{}, fmt.Errorf("invalid vmdkPath %q: missing slash between vmname directory and vmdk file, expected '[datastore] vmname/vmname.vmdk'", vmdkPath)
	}

	vmnameDir := pathParts[0]
	if vmnameDir == "" {
		return VMDisk{}, fmt.Errorf("invalid vmdkPath %q: VM directory name cannot be empty", vmdkPath)
	}

	vmdkFile := pathParts[1]
	if vmdkFile == "" {
		return VMDisk{}, fmt.Errorf("invalid vmdkPath %q: VMDK file name cannot be empty", vmdkPath)
	}

	if !strings.HasSuffix(vmdkFile, ".vmdk") {
		return VMDisk{}, fmt.Errorf("invalid vmdkPath %q: vmdk file name must end with '.vmdk'", vmdkPath)
	}

	vmName := strings.TrimSuffix(vmdkFile, ".vmdk")
	vmName = strings.TrimSuffix(vmName, "-flat")

	return VMDisk{
		VMName:    vmName,
		Datastore: datastore,
		VmdkFile:  vmdkFile,
		VmnameDir: vmnameDir,
	}, nil
}
