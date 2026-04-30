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
}

// NewPopulator creates a Populator with an embedded CopyContext describing how the
// copy will be performed (clone method, source disk sizes). StorageProtocol is
// detected by the populator during Populate() and available via GetCopyContext().
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

	// Single vSphere roundtrip: fetch source disk sizes and backing info.
	sourceDiskCap, sourceDatastoreAlloc, backing := fetchDiskInfo(ctx, log, vsphereClient, vmId, vmdkPath)

	diskType := detectDiskType(backing)
	log.Info("disk type detected", "type", diskType)

	switch diskType {
	case DiskTypeVVol:
		if canUse(storageApi, DiskTypeVVol) {
			log.Info("using VVol populator")
			return createVVolPopulator(storageApi, vsphereClient, sourceDiskCap, sourceDatastoreAlloc)
		}

	case DiskTypeRDM:
		if canUse(storageApi, DiskTypeRDM) {
			log.Info("using RDM populator")
			return createRDMPopulator(storageApi, vsphereClient, sourceDiskCap, sourceDatastoreAlloc)
		}
	}

	log.Info("using VMDK/Xcopy populator")
	return createVMDKPopulator(storageApi, vsphereClient, sshConfig, sourceDiskCap, sourceDatastoreAlloc)
}

// fetchDiskInfo retrieves provisioned capacity, datastore-allocated bytes, and disk
// backing info from vSphere in a single VM lookup. Best-effort: returns zeros and
// an empty DiskBacking on failure.
func fetchDiskInfo(ctx context.Context, log klog.Logger, vsphereClient vmware.Client, vmId, vmdkPath string) (int64, int64, *vmware.DiskBacking) {
	sourceDiskCap, sourceDatastoreAlloc, backing, err := vsphereClient.GetVirtualDiskSizes(ctx, vmId, vmdkPath)
	if err != nil {
		log.V(1).Info("could not read source disk info for metrics", "err", err)
		return 0, 0, &vmware.DiskBacking{}
	}
	if sourceDiskCap > 0 {
		log.V(1).Info("source virtual disk provisioned size for metrics", "bytes", sourceDiskCap)
	}
	if sourceDatastoreAlloc > 0 {
		if sourceDiskCap > 0 && sourceDatastoreAlloc > sourceDiskCap {
			sourceDatastoreAlloc = sourceDiskCap
		}
		log.V(1).Info("source VMDK datastore allocated bytes for metrics", "bytes", sourceDatastoreAlloc)
	}
	if backing == nil {
		backing = &vmware.DiskBacking{}
	}
	return sourceDiskCap, sourceDatastoreAlloc, backing
}

// createVVolPopulator creates VVol populator
func createVVolPopulator(storageApi StorageApi, vmwareClient vmware.Client, sourceDiskCap, sourceDatastoreAlloc int64) (Populator, error) {
	vvolApi, ok := storageApi.(VVolCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement VVolCapable")
	}

	copyCtx := CopyContext{CloneMethod: "vvol", SourceDiskCapacityBytes: sourceDiskCap, SourceDiskDatastoreAllocatedBytes: sourceDatastoreAlloc}
	return NewVvolPopulator(vvolApi, vmwareClient, copyCtx)
}

// createRDMPopulator creates RDM populator
func createRDMPopulator(storageApi StorageApi, vmwareClient vmware.Client, sourceDiskCap, sourceDatastoreAlloc int64) (Populator, error) {
	rdmApi, ok := storageApi.(RDMCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement RDMCapable")
	}

	copyCtx := CopyContext{CloneMethod: "rdm", SourceDiskCapacityBytes: sourceDiskCap, SourceDiskDatastoreAllocatedBytes: sourceDatastoreAlloc}
	return NewRDMPopulator(rdmApi, vmwareClient, copyCtx)
}

// createVMDKPopulator creates VMDK/Xcopy populator (default/fallback)
func createVMDKPopulator(storageApi StorageApi, vmwareClient vmware.Client, sshConfig *SSHConfig, sourceDiskCap, sourceDatastoreAlloc int64) (Populator, error) {
	vmdkApi, ok := storageApi.(VMDKCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement VMDKCapable (required)")
	}

	cloneMethod := "vib"
	if sshConfig != nil && sshConfig.UseSSH {
		cloneMethod = "ssh"
	}
	copyCtx := CopyContext{CloneMethod: cloneMethod, SourceDiskCapacityBytes: sourceDiskCap, SourceDiskDatastoreAllocatedBytes: sourceDatastoreAlloc}

	var pop Populator
	var err error
	if sshConfig != nil && sshConfig.UseSSH {
		pop, err = NewWithRemoteEsxcliSSH(vmdkApi,
			vmwareClient,
			copyCtx,
			sshConfig.PrivateKey,
			sshConfig.PublicKey)
	} else {
		pop, err = NewWithRemoteEsxcli(vmdkApi, vmwareClient, copyCtx)
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
