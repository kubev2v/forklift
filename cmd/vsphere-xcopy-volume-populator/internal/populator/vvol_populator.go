package populator

import (
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

type VvolPopulator struct {
	VSphereClient vmware.Client
	StorageApi    VvolStorageApi
}

func NewVvolPopulator(storageApi VvolStorageApi, vsphereHostname, vsphereUsername, vspherePassword string) (Populator, error) {
	c, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vmware client: %w", err)
	}
	return &VvolPopulator{
		VSphereClient: c,
		StorageApi:    storageApi,
	}, nil
}

func (p *VvolPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, progress chan<- uint, quit chan error) (errFinal error) {
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

	// Try using vSphere API to discover source volume first (preferred method)
	klog.Infof("VVol Populator: Attempting vSphere API discovery...")
	err := p.StorageApi.VvolCopy(p.VSphereClient, vmId, sourceVMDKFile, pv, progress)
	if err != nil {
		klog.Errorf("VVol Populator: discovery of source volume using vSphere API failed: %v", err)
		return fmt.Errorf("failed to copy VMDK using VVol storage API: %w", err)
	}

	klog.Infof("VVol Populator: Copy operation completed successfully")
	return nil
}
