package populator

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/lib/vsphere_offload/vmware"
	"k8s.io/klog/v2"
)

type VvolPopulator struct {
	vSphereClient vmware.Client
	storageApi    VVolCapable
}

func NewVvolPopulator(storageApi VVolCapable, vmwareClient vmware.Client) (Populator, error) {
	return &VvolPopulator{
		vSphereClient: vmwareClient,
		storageApi:    storageApi,
	}, nil
}

func (p *VvolPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, hostLocker Hostlocker, progress chan<- uint64, xcopyUsed chan<- int, quit chan error) (errFinal error) {
	defer func() {
		r := recover()
		if r != nil {
			klog.Infof("VVol Populator: recovered from panic: %v", r)
		}
		klog.Infof("VVol Populator: exiting with final error: %v", errFinal)
		quit <- errFinal
	}()

	klog.Infof("VVol Populator: Starting copy operation")
	klog.Infof("VVol Populator: VM ID: %s, Source VMDK: %s, Target: %s", vmId, sourceVMDKFile, pv.Name)

	// VVol copy does not use xcopy
	xcopyUsed <- 0

	// Try using vSphere API to discover source volume first (preferred method)
	klog.Infof("VVol Populator: Starting VVol copy operation...")
	err := p.storageApi.VvolCopy(p.vSphereClient, vmId, sourceVMDKFile, pv, progress)
	if err != nil {
		klog.Errorf("VVol Populator: discovery of source volume using vSphere API failed: %v", err)
		return fmt.Errorf("failed to copy VMDK using VVol storage API: %w", err)
	}

	klog.Infof("VVol Populator: Copy operation completed successfully")
	return nil
}
