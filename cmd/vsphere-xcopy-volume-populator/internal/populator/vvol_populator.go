package populator

import (
	"fmt"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
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
	log := klog.Background().WithName("copy-offload").WithName("vvol")
	defer func() {
		r := recover()
		if r != nil {
			log.Info("recovered from panic", "panic", r)
		}
		log.Info("VVol copy exiting", "err", errFinal)
		quit <- errFinal
	}()

	log.Info("VVol copy started", "vm", vmId, "source", sourceVMDKFile, "target", pv.Name)

	// VVol copy does not use xcopy
	xcopyUsed <- 0

	err := p.storageApi.VvolCopy(p.vSphereClient, vmId, sourceVMDKFile, pv, progress)
	if err != nil {
		log.Error(err, "VVol copy failed (source volume discovery)")
		return fmt.Errorf("failed to copy VMDK using VVol storage API: %w", err)
	}

	log.Info("VVol copy finished")
	return nil
}
