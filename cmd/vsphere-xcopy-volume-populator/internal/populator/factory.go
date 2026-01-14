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

	diskType, err := detectDiskType(ctx, vsphereClient, vmId, vmdkPath)
	if err != nil {
		klog.Warningf("Failed to detect disk type: %v, using VMDK/Xcopy", err)
		return createVMDKPopulator(storageApi, vsphereClient, sshConfig)
	}

	klog.Infof("Detected disk type: %s", diskType)

	// Step 2: Try to use optimized method for detected disk type
	switch diskType {
	case DiskTypeVVol:
		if canUse(storageApi, DiskTypeVVol) {
			klog.Infof("VVol method is available, using VVol populator")
			return createVVolPopulator(storageApi, vsphereClient)
		}

	case DiskTypeRDM:
		if canUse(storageApi, DiskTypeRDM) {
			klog.Infof("RDM method is available, using RDM populator")
			return createRDMPopulator(storageApi, vsphereClient)
		}
	}

	// Default: Use VMDK/Xcopy (always works)
	klog.Infof("Using VMDK/Xcopy populator")
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
	_, ok := storageApi.(VMDKCapable)
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
		pop, err = NewWithRemoteEsxcliSSH(storageApi,
			vmwareClient,
			sshConfig.PrivateKey,
			sshConfig.PublicKey,
			timeout)
	} else {
		pop, err = NewWithRemoteEsxcli(storageApi, vmwareClient)
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
