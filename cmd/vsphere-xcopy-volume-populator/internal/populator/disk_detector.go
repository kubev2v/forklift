package populator

import (
	"context"
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

// DiskTypeDetector detects disk types using vSphere API
// This is completely separate from storage implementations
type DiskTypeDetector interface {
	// DetectDiskType determines the backing type of a VM disk
	// Returns DiskTypeVVol, DiskTypeRDM, or DiskTypeVMDK
	DetectDiskType(ctx context.Context, vsphereClient vmware.Client, vmId string, vmdkPath string) (DiskType, error)
}

// VSphereTypeDetector uses vSphere API to detect disk backing type
type VSphereTypeDetector struct{}

// NewVSphereTypeDetector creates a new VSphereTypeDetector
func NewVSphereTypeDetector() *VSphereTypeDetector {
	return &VSphereTypeDetector{}
}

// DetectDiskType inspects the VM disk backing info to determine the type
func (d *VSphereTypeDetector) DetectDiskType(ctx context.Context, client vmware.Client, vmId string, vmdkPath string) (DiskType, error) {
	klog.V(2).Infof("Detecting disk type for VM %s, disk %s", vmId, vmdkPath)

	// Get disk backing info from vSphere
	backing, err := client.GetVMDiskBacking(ctx, vmId, vmdkPath)
	if err != nil {
		return "", fmt.Errorf("failed to get disk backing info from vSphere: %w", err)
	}

	// Determine disk type based on backing info
	switch {
	case backing.VVolId != "":
		// Virtual Volume (VVol) - has VVolId in backing
		klog.Infof("Detected VVol disk (VVolId: %s)", backing.VVolId)
		return DiskTypeVVol, nil

	case backing.IsRDM:
		// Raw Device Mapping
		klog.Infof("Detected RDM disk (DeviceName: %s)", backing.DeviceName)
		return DiskTypeRDM, nil

	default:
		// Traditional VMDK on datastore
		klog.Infof("Detected VMDK disk")
		return DiskTypeVMDK, nil
	}
}
