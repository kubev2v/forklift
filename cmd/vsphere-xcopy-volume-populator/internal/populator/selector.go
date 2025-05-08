package populator

import (
	"context"
	"fmt"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"k8s.io/klog/v2"
)

// SSHConfig holds SSH configuration for VMDK/Xcopy populator
type SSHConfig struct {
	UseSSH         bool
	PrivateKey     []byte
	PublicKey      []byte
	TimeoutSeconds int
}

// PopulatorSelector selects the appropriate populator based on disk type
type PopulatorSelector struct {
	storageApi      StorageApi
	settings        *PopulatorSettings
	typeDetector    DiskTypeDetector
	vsphereClient   vmware.Client
	vsphereHostname string
	vsphereUsername string
	vspherePassword string
}

// NewPopulatorSelector creates a new PopulatorSelector
func NewPopulatorSelector(
	storageApi StorageApi,
	settings *PopulatorSettings,
	vsphereHostname string,
	vsphereUsername string,
	vspherePassword string,
) (*PopulatorSelector, error) {
	// Create vSphere client for type detection
	vsphereClient, err := vmware.NewClient(vsphereHostname, vsphereUsername, vspherePassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create vSphere client: %w", err)
	}

	return &PopulatorSelector{
		storageApi:      storageApi,
		settings:        settings,
		typeDetector:    NewVSphereTypeDetector(),
		vsphereClient:   vsphereClient,
		vsphereHostname: vsphereHostname,
		vsphereUsername: vsphereUsername,
		vspherePassword: vspherePassword,
	}, nil
}

// SelectPopulator determines the appropriate populator based on disk type
// Falls back to VMDK/Xcopy if the detected type's method is not available
func (s *PopulatorSelector) SelectPopulator(
	ctx context.Context,
	vmId string,
	vmdkPath string,
	sshConfig *SSHConfig,
) (Populator, DiskType, error) {

	// Step 1: Detect disk type using vSphere API
	diskType, err := s.typeDetector.DetectDiskType(ctx, s.vsphereClient, vmId, vmdkPath)
	if err != nil {
		klog.Warningf("Failed to detect disk type: %v, using VMDK/Xcopy", err)
		return s.createVMDKPopulator(sshConfig)
	}

	klog.Infof("Detected disk type: %s", diskType)

	// Step 2: Try to use optimized method for detected disk type
	switch diskType {
	case DiskTypeVVol:
		if s.isMethodAvailable(DiskTypeVVol) {
			klog.Infof("VVol method is available, using VVol populator")
			if pop, err := s.createVVolPopulator(); err == nil {
				return pop, DiskTypeVVol, nil
			} else {
				klog.Warningf("Failed to create VVol populator: %v", err)
			}
		}

	case DiskTypeRDM:
		if s.isMethodAvailable(DiskTypeRDM) {
			klog.Infof("RDM method is available, using RDM populator")
			if pop, err := s.createRDMPopulator(); err == nil {
				return pop, DiskTypeRDM, nil
			} else {
				klog.Warningf("Failed to create RDM populator: %v", err)
			}
		}
	}

	// Default: Use VMDK/Xcopy (always works)
	klog.Infof("Using VMDK/Xcopy populator")
	return s.createVMDKPopulator(sshConfig)
}

// isMethodAvailable checks if a method is both enabled and supported
func (s *PopulatorSelector) isMethodAvailable(diskType DiskType) bool {
	// Check if enabled in settings
	if !s.isMethodEnabled(diskType) {
		klog.V(2).Infof("Method %s is disabled in settings", diskType)
		return false
	}

	// Check if storage supports it
	if !s.isMethodSupported(diskType) {
		klog.V(2).Infof("Storage does not support method %s", diskType)
		return false
	}

	return true
}

// isMethodEnabled checks settings - methods are enabled unless explicitly disabled
func (s *PopulatorSelector) isMethodEnabled(diskType DiskType) bool {
	switch diskType {
	case DiskTypeVVol:
		return !s.settings.VVolDisabled
	case DiskTypeRDM:
		return !s.settings.RDMDisabled
	case DiskTypeVMDK:
		return true // Cannot be disabled - always the fallback
	default:
		return false
	}
}

// isMethodSupported queries storage to see if it supports the disk type
func (s *PopulatorSelector) isMethodSupported(diskType DiskType) bool {
	// VMDK is always supported if storage implements VMDKCapable
	if diskType == DiskTypeVMDK {
		_, ok := s.storageApi.(VMDKCapable)
		return ok
	}

	// Check if storage implements DiskTypeCapable interface
	if capable, ok := s.storageApi.(DiskTypeCapable); ok {
		supported := capable.SupportsDiskType(diskType)
		klog.V(2).Infof("Storage DiskTypeCapable.SupportsDiskType(%s) = %v", diskType, supported)
		return supported
	}

	// If DiskTypeCapable not implemented, assume no support
	klog.V(2).Infof("Storage does not implement DiskTypeCapable, assuming no support for %s", diskType)
	return false
}

// createVVolPopulator creates VVol populator
func (s *PopulatorSelector) createVVolPopulator() (Populator, error) {
	vvolApi, ok := s.storageApi.(VVolCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement VVolCapable")
	}

	return NewVvolPopulator(vvolApi, s.vsphereHostname, s.vsphereUsername, s.vspherePassword)
}

// createRDMPopulator creates RDM populator
func (s *PopulatorSelector) createRDMPopulator() (Populator, error) {
	rdmApi, ok := s.storageApi.(RDMCapable)
	if !ok {
		return nil, fmt.Errorf("storage API does not implement RDMCapable")
	}

	return NewRDMPopulator(rdmApi, s.vsphereHostname, s.vsphereUsername, s.vspherePassword)
}

// createVMDKPopulator creates VMDK/Xcopy populator (default/fallback)
func (s *PopulatorSelector) createVMDKPopulator(sshConfig *SSHConfig) (Populator, DiskType, error) {
	vmdkApi, ok := s.storageApi.(VMDKCapable)
	if !ok {
		return nil, "", fmt.Errorf("storage API does not implement VMDKCapable (required)")
	}

	var pop Populator
	var err error

	if sshConfig != nil && sshConfig.UseSSH {
		timeout := sshConfig.TimeoutSeconds
		if timeout == 0 {
			timeout = 30
		}
		pop, err = NewWithRemoteEsxcliSSH(vmdkApi,
			s.vsphereHostname,
			s.vsphereUsername,
			s.vspherePassword,
			sshConfig.PrivateKey,
			sshConfig.PublicKey,
			timeout)
	} else {
		pop, err = NewWithRemoteEsxcli(vmdkApi,
			s.vsphereHostname,
			s.vsphereUsername,
			s.vspherePassword)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to create VMDK/Xcopy populator: %w", err)
	}

	return pop, DiskTypeVMDK, nil
}

// GetSSHTimeout returns the SSH timeout duration
func GetSSHTimeout(timeoutSeconds int) time.Duration {
	if timeoutSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(timeoutSeconds) * time.Second
}
