package populator

import (
	"context"
	"fmt"
	"os"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

var settings = populatorSettings{
	VVolDisabled: os.Getenv("DISABLE_VVOL_METHOD") == "true",
	RDMDisabled:  os.Getenv("DISABLE_RDM_METHOD") == "true",
}

// SSHConfig holds SSH configuration for VMDK/Xcopy populator
type SSHConfig struct {
	UseSSH         bool
	PrivateKey     []byte
	PublicKey      []byte
	TimeoutSeconds int
}

// NewPopulator creates a new PopulatorSelector
func NewPopulator(
	storageApi StorageApi,
	vsphereHostname string,
	vsphereUsername string,
	vspherePassword string,
	vmId string,
	vmdkPath string,
	sshConfig *SSHConfig,
) (Populator, error) {
	// Create vSphere client for type detection
	vsphereClient, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vSphere client: %w", err)
	}

	ctx := context.Background()
	log := klog.Background().WithName("copy-offload").WithName("setup")

	diskType, err := detectDiskType(ctx, vsphereClient, vmId, vmdkPath)
	if err != nil {
		log.Info("disk type detection failed, using VMDK/Xcopy", "err", err)
		return createVMDKPopulator(storageApi, vsphereClient, sshConfig)
	}

	log.Info("disk type detected", "type", diskType)

	switch diskType {
	case DiskTypeVVol:
		if canUse(storageApi, DiskTypeVVol) {
			log.Info("using VVol populator")
			return createVVolPopulator(storageApi, vsphereClient)
		}

	case DiskTypeRDM:
		if canUse(storageApi, DiskTypeRDM) {
			log.Info("using RDM populator")
			return createRDMPopulator(storageApi, vsphereClient)
		}
	}

	log.Info("using VMDK/Xcopy populator")
	return createVMDKPopulator(storageApi, vsphereClient, sshConfig)
}

// createVVolPopulator creates VVol populator
func createVVolPopulator(storageApi StorageApi, vmwareClient vmware.Client) (Populator, error) {
	vvolApi, ok := storageApi.(VVolCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement VVolCapable")
	}

	return NewVvolPopulator(vvolApi, vmwareClient)
}

// createRDMPopulator creates RDM populator
func createRDMPopulator(storageApi StorageApi, vmwareClient vmware.Client) (Populator, error) {
	rdmApi, ok := storageApi.(RDMCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement RDMCapable")
	}

	return NewRDMPopulator(rdmApi, vmwareClient)
}

// createVMDKPopulator creates VMDK/Xcopy populator (default/fallback)
func createVMDKPopulator(storageApi StorageApi, vmwareClient vmware.Client, sshConfig *SSHConfig) (Populator, error) {
	vmdkApi, ok := storageApi.(VMDKCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement VMDKCapable (required)")
	}

	var pop Populator
	var err error

	if sshConfig != nil && sshConfig.UseSSH {
		timeout := sshConfig.TimeoutSeconds
		if timeout == 0 {
			timeout = 30
		}
		pop, err = NewWithRemoteEsxcliSSH(vmdkApi,
			vmwareClient,
			sshConfig.PrivateKey,
			sshConfig.PublicKey,
			timeout)
	} else {
		pop, err = NewWithRemoteEsxcli(vmdkApi, vmwareClient)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create VMDK/Xcopy populator: %w", err)
	}

	return pop, nil
}

// canUse checks if a disk type method is enabled and supported
func canUse(storageApi StorageApi, diskType DiskType) bool {
	switch diskType {
	case DiskTypeVVol:
		if settings.VVolDisabled {
			return false
		}
		_, ok := storageApi.(VVolCapable)
		return ok

	case DiskTypeRDM:
		if settings.RDMDisabled {
			return false
		}
		_, ok := storageApi.(RDMCapable)
		return ok

	default:
		return false
	}
}
