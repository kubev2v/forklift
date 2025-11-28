package populator

import (
	"context"
	"fmt"
	"strings"
)

type Populator interface {
	// Populate will populate the volume identified by volumeHanle with the content of
	// the sourceVMDKFile.
	// persistentVolume is a slim version of k8s PersistentVolume created by the CSI driver,
	// to help identify its underlying LUN in the storage system.
	Populate(vmId string, sourceVMDKFile string, persistentVolume PersistentVolume, hostLocker Hostlocker, progress chan<- uint, quit chan error) error
}

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/hostlocker_mock.go -package=mocks . Hostlocker
type Hostlocker interface {
	// WithLock acquires a distributed lock and executes work. The work function receives a context
	// that will be cancelled if the lock is lost (e.g., due to lease renewal failure).
	// Work should check ctx.Err() periodically and abort gracefully if cancelled.
	WithLock(ctx context.Context, hostID string, work func(ctx context.Context) error) error
}

type PersistentVolume struct {
	Name             string
	VolumeHandle     string
	VolumeAttributes map[string]string
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
	Datastore string
	VmHomeDir string
	VmdkFile  string
}

func (d *VMDisk) Path() string {
	return fmt.Sprintf("/vmfs/volumes/%s/%s/%s", d.Datastore, d.VmHomeDir, d.VmdkFile)
}

func ParseVmdkPath(vmdkPath string) (VMDisk, error) {
	if vmdkPath == "" {
		return VMDisk{}, fmt.Errorf("vmdkPath cannot be empty")
	}

	parts := strings.SplitN(vmdkPath, "] ", 2)
	if len(parts) != 2 {
		return VMDisk{}, fmt.Errorf("Invalid vmdkPath %q, should be '[datastore] vmname/xyz.vmdk'", vmdkPath)
	}

	datastore := strings.TrimPrefix(parts[0], "[")
	pathParts := strings.Split(parts[1], "/")

	if len(pathParts) != 2 {
		return VMDisk{}, fmt.Errorf("Invalid vmdkPath %q, should be '[datastore] vmname/xyz.vmdk'", vmdkPath)
	}

	return VMDisk{
		Datastore: datastore,
		VmHomeDir: pathParts[0],
		VmdkFile:  pathParts[1],
	}, nil
}
