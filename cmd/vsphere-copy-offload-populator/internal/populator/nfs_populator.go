package populator

import (
	"context"
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"k8s.io/klog/v2"
)

// NFSPopulator offloads the copy of an NFS-backed disk to the array's file API.
// Both source datastore and destination PV are exports of the same array, so the
// array copies the file itself.
type NFSPopulator struct {
	vSphereClient vmware.Client
	storageApi    NFSCapable
	copyCtx       CopyContext
}

func (p *NFSPopulator) GetCopyContext() CopyContext {
	return p.copyCtx
}

func NewNFSPopulator(storageApi NFSCapable, vmwareClient vmware.Client, copyCtx CopyContext) (Populator, error) {
	return &NFSPopulator{
		vSphereClient: vmwareClient,
		storageApi:    storageApi,
		copyCtx:       copyCtx,
	}, nil
}

func (p *NFSPopulator) Populate(ctx context.Context, vmId string, migrationHostId, sourceVMDKFile string, pv PersistentVolume, hostLocker Hostlocker, progress chan<- uint64, xcopyUsed chan<- int, quit chan error) (errFinal error) {
	log := klog.Background().WithName("copy-offload").WithName("nfs")
	defer func() {
		r := recover()
		if r != nil {
			log.Info("recovered from panic", "panic", r)
			quit <- fmt.Errorf("recovered failure: %v", r)
			return
		}
		log.Info("NFS copy exiting", "err", errFinal)
		quit <- errFinal
	}()

	log.Info("NFS copy started", "vm", vmId, "source", sourceVMDKFile, "target", pv.Name)

	// NFS copy does not use xcopy
	xcopyUsed <- 0

	err := p.storageApi.NFSCopy(p.vSphereClient, vmId, sourceVMDKFile, pv, progress)
	if err != nil {
		log.Error(err, "NFS copy failed")
		return fmt.Errorf("failed to copy VMDK using NFS storage API: %w", err)
	}

	log.Info("NFS copy finished")
	return nil
}
