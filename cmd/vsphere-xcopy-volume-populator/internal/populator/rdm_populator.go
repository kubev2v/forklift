package populator

import (
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

// RDMPopulator handles population of RDM-backed disks
type RDMPopulator struct {
	vSphereClient vmware.Client
	storageApi    RDMCapable
}

// NewRDMPopulator creates a new RDM populator
func NewRDMPopulator(storageApi RDMCapable, vmwareClient vmware.Client) (Populator, error) {
	return &RDMPopulator{
		vSphereClient: vmwareClient,
		storageApi:    storageApi,
	}, nil
}

// Populate performs the RDM copy operation
func (p *RDMPopulator) Populate(vmId string, sourceVMDKFile string, pv PersistentVolume, hostLocker Hostlocker, progress chan<- uint64, xcopyUsed chan<- int, quit chan error) (errFinal error) {
	log := klog.Background().WithName("copy-offload").WithName("rdm")
	defer func() {
		r := recover()
		if r != nil {
			log.Info("recovered from panic", "panic", r)
		}
		log.Info("RDM copy exiting", "err", errFinal)
		quit <- errFinal
	}()

	log.Info("RDM copy started", "vm", vmId, "source", sourceVMDKFile, "target", pv.Name)

	// RDM copy does not use xcopy
	xcopyUsed <- 0

	err := p.storageApi.RDMCopy(p.vSphereClient, vmId, sourceVMDKFile, pv, progress)
	if err != nil {
		log.Error(err, "RDM copy failed")
		return fmt.Errorf("failed to copy RDM disk: %w", err)
	}

	log.Info("RDM copy finished")
	return nil
}
