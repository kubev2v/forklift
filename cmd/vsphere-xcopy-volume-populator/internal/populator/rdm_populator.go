package populator

import (
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

// RDMPopulator handles population of RDM-backed disks
type RDMPopulator struct {
	VSphereClient vmware.Client
	StorageApi    RDMCapable
}

// NewRDMPopulator creates a new RDM populator
func NewRDMPopulator(storageApi RDMCapable, vmwareClient vmware.Client) (Populator, error) {
	return &RDMPopulator{
		VSphereClient: vmwareClient,
		StorageApi:    storageApi,
	}, nil
}

// Populate performs the RDM copy operation
func (p *RDMPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, hostLocker Hostlocker, progress chan<- uint64, xcopyUsed chan<- int, quit chan error) (errFinal error) {
	defer func() {
		r := recover()
		if r != nil {
			klog.Infof("RDM Populator: recovered from panic: %v", r)
		}
		klog.Infof("RDM Populator: exiting with final error: %v", errFinal)
		quit <- errFinal
	}()

	klog.Infof("RDM Populator: Starting copy operation")
	klog.Infof("RDM Populator: VM ID: %s, Source VMDK: %s, Target: %s", vmId, sourceVMDKFile, pv.Name)

	// RDM copy does not use xcopy
	xcopyUsed <- 0

	// Perform the RDM copy operation
	klog.Infof("RDM Populator: Starting RDM copy operation...")
	err := p.StorageApi.RDMCopy(p.VSphereClient, vmId, sourceVMDKFile, pv, progress)
	if err != nil {
		klog.Errorf("RDM Populator: RDM copy operation failed: %v", err)
		return fmt.Errorf("failed to copy RDM disk: %w", err)
	}

	klog.Infof("RDM Populator: Copy operation completed successfully")
	return nil
}
