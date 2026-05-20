package primera3par

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
)

const PROVIDER_ID = "60002ac"

// Ensure Primera3ParClonner implements required interfaces
var _ populator.RDMCapable = &Primera3ParClonner{}
var _ populator.VVolCapable = &Primera3ParClonner{}
var _ populator.StorageArrayInfoProvider = &Primera3ParClonner{}

type Primera3ParClonner struct {
	client         Primera3ParClient
	initiatorGroup string
	arrayInfo      populator.StorageArrayInfo
}

// GetStorageArrayInfo returns metadata about the Primera/3PAR array for metric labels.
func (c *Primera3ParClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return c.arrayInfo
}

func NewPrimera3ParClonner(storageHostname, storageUsername, storagePassword string, sslSkipVerify bool) (Primera3ParClonner, error) {
	clon := NewPrimera3ParClientWsImpl(storageHostname, storageUsername, storagePassword, sslSkipVerify)
	clonner := Primera3ParClonner{
		client: &clon,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "HPE",
			Product: "Primera/3PAR",
		},
	}

	// Fetch model and version from the API
	sysInfo, err := clon.GetSystemInfo()
	if err != nil {
		klog.Warningf("Failed to get Primera/3PAR system info for metrics: %v", err)
	} else {
		clonner.arrayInfo.Model = sysInfo.Model
		clonner.arrayInfo.Version = sysInfo.SystemVersion
	}

	return clonner, nil
}

// EnsureClonnerIgroup creates or update an initiator group with the clonnerIqn
func (c *Primera3ParClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	c.initiatorGroup = initiatorGroup
	hostNames, err := c.client.EnsureHostsWithIds(adapterIds)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure host with IQN: %w", err)
	}

	err = c.client.EnsureHostSetExists(initiatorGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure host set: %w", err)
	}

	for _, hostName := range hostNames {
		klog.Infof("adding host %s, to initiatorGroup: %s", hostName, initiatorGroup)
		err = c.client.AddHostToHostSet(initiatorGroup, hostName)
		if err != nil {
			return nil, fmt.Errorf("failed to add host to host set: %w", err)
		}
	}
	return nil, nil
}

func (p *Primera3ParClonner) GetNaaID(lun populator.LUN) populator.LUN {
	return lun
}

func (c *Primera3ParClonner) MapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return c.Map(c.initiatorGroup, targetLUN, mappingContext)
}

func (c *Primera3ParClonner) UnmapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return c.UnMap(c.initiatorGroup, targetLUN, mappingContext)
}

// Map is responsible to mapping an initiator group to a LUN
func (c *Primera3ParClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return c.client.EnsureLunMapped(initiatorGroup, targetLUN)
}

// UnMap is responsible to unmapping an initiator group from a LUN
func (c *Primera3ParClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return c.client.LunUnmap(context.TODO(), initiatorGroup, targetLUN.Name)
}

// Return initiatorGroups the LUN is mapped to
func (p *Primera3ParClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	res, err := p.client.CurrentMappedGroups(targetLUN.Name, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to get current mapped groups: %w", err)
	}
	return res, nil
}

func (c *Primera3ParClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	lun := populator.LUN{VolumeHandle: pv.VolumeHandle}
	lun, err := c.client.GetLunDetailsByVolumeName(pv.VolumeHandle, lun)
	if err != nil {
		return populator.LUN{}, err
	}
	return lun, nil
}

// VvolCopy performs a direct copy operation using vSphere API to discover source volume
func (c *Primera3ParClonner) VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	klog.Infof("Starting VVol copy operation for VM %s", vmId)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get VVol disk backing info: %w", err)
	}

	if backing.VVolId == "" {
		return fmt.Errorf("disk %s is not a VVol disk", sourceVMDKFile)
	}

	klog.Infof("Found VVol backing with ID %s", backing.VVolId)

	sourceVolumeName, err := c.findVolumeByVVolID(backing.VVolId)
	if err != nil {
		return fmt.Errorf("failed to find source volume by VVol ID %s: %w", backing.VVolId, err)
	}

	targetLUN, err := c.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	klog.Infof("Copying from source volume %s to target volume %s", sourceVolumeName, targetLUN.Name)

	progress <- 10

	err = c.client.CopyVolume(sourceVolumeName, targetLUN.Name)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	progress <- 100
	klog.Infof("VVol copy operation completed successfully")
	return nil
}

func (c *Primera3ParClonner) findVolumeByVVolID(vvolID string) (string, error) {
	volumes, err := c.client.GetVolumes()
	if err != nil {
		return "", fmt.Errorf("failed to get volumes: %w", err)
	}

	searchID := strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(vvolID, "rfc4122.")), "-", "")

	for _, v := range volumes {
		if strings.Contains(strings.ToLower(v.Name), searchID) {
			return v.Name, nil
		}
		if strings.ToLower(v.WWN) == searchID {
			return v.Name, nil
		}
	}

	return "", fmt.Errorf("could not find volume matching VVol ID %s", vvolID)
}

func (c *Primera3ParClonner) RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	klog.Infof("3PAR RDM Copy: Starting RDM copy operation for VM %s", vmId)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get RDM disk backing info: %w", err)
	}

	if !backing.IsRDM {
		return fmt.Errorf("disk %s is not an RDM disk", sourceVMDKFile)
	}

	klog.Infof("3PAR RDM Copy: Found RDM device: %s", backing.DeviceName)

	sourceLUN, err := c.resolveRDMToLUN(backing.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to resolve RDM device to source LUN: %w", err)
	}

	targetLUN, err := c.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	klog.Infof("3PAR RDM Copy: Copying from source LUN %s to target LUN %s", sourceLUN.Name, targetLUN.Name)

	progress <- 10

	err = c.client.CopyVolume(sourceLUN.Name, targetLUN.Name)
	if err != nil {
		return fmt.Errorf("3PAR CopyVolume failed: %w", err)
	}

	progress <- 100

	klog.Infof("3PAR RDM Copy: Copy operation completed successfully")
	return nil
}

func (c *Primera3ParClonner) resolveRDMToLUN(deviceName string) (populator.LUN, error) {
	klog.Infof("3PAR RDM Copy: Resolving RDM device %s to LUN", deviceName)

	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		klog.Warningf("Could not extract serial from NAA %s: %v, trying to find by listing volumes", deviceName, err)
		return c.findVolumeByDeviceName(deviceName)
	}

	volumes, err := c.client.GetVolumes()
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get volumes: %w", err)
	}

	for _, v := range volumes {
		if strings.ToLower(v.WWN) == strings.ToLower(serial) {
			return populator.LUN{
				Name:         v.Name,
				SerialNumber: v.WWN,
				NAA:          fmt.Sprintf("naa.%s%s", PROVIDER_ID, strings.ToLower(v.WWN)),
			}, nil
		}
	}

	return populator.LUN{}, fmt.Errorf("failed to find volume by serial %s", serial)
}

func extractSerialFromNAA(naa string) (string, error) {
	naa = strings.ToLower(naa)
	naa = strings.TrimPrefix(naa, "naa.")

	providerIDLower := strings.ToLower(PROVIDER_ID)
	if !strings.HasPrefix(naa, providerIDLower) {
		return "", fmt.Errorf("NAA %s does not appear to be a 3PAR device (expected prefix %s)", naa, PROVIDER_ID)
	}

	serial := strings.TrimPrefix(naa, providerIDLower)
	if serial == "" {
		return "", fmt.Errorf("could not extract serial from NAA %s", naa)
	}

	return strings.ToUpper(serial), nil
}

func (c *Primera3ParClonner) findVolumeByDeviceName(deviceName string) (populator.LUN, error) {
	volumes, err := c.client.GetVolumes()
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to list volumes: %w", err)
	}

	deviceName = strings.ToLower(deviceName)

	for _, volume := range volumes {
		naa := fmt.Sprintf("naa.%s%s", PROVIDER_ID, strings.ToLower(volume.WWN))

		if strings.Contains(deviceName, strings.ToLower(volume.WWN)) ||
			strings.Contains(deviceName, naa) ||
			deviceName == naa {
			klog.Infof("3PAR RDM Copy: Found matching volume %s for device %s", volume.Name, deviceName)
			return populator.LUN{
				Name:         volume.Name,
				SerialNumber: volume.WWN,
				NAA:          fmt.Sprintf("naa.%s%s", PROVIDER_ID, strings.ToLower(volume.WWN)),
			}, nil
		}
	}

	return populator.LUN{}, fmt.Errorf("could not find volume matching RDM device %s", deviceName)
}
