package primera3par

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
)

const (
	PROVIDER_ID = "60002ac"
	loggerName  = "copy-offload"
)

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
	log := klog.Background().WithName(loggerName).WithName("setup")
	sysInfo, err := clon.GetSystemInfo()
	if err != nil {
		log.Info("failed to get Primera/3PAR system info for metrics", "err", err)
	} else {
		clonner.arrayInfo.Model = sysInfo.Model
		clonner.arrayInfo.Version = sysInfo.SystemVersion
		log.V(2).Info("Primera/3PAR array info", "vendor", clonner.arrayInfo.Vendor, "product", clonner.arrayInfo.Product, "model", clonner.arrayInfo.Model, "version", clonner.arrayInfo.Version)
	}

	return clonner, nil
}

// EnsureClonnerIgroup creates or update an initiator group with the clonnerIqn
func (c *Primera3ParClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	log := klog.Background().WithName(loggerName).WithName("map").WithName("ensure-igroup")
	log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", adapterIds)

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
		log.V(2).Info("adding host to host set", "host", hostName, "group", initiatorGroup)
		err = c.client.AddHostToHostSet(initiatorGroup, hostName)
		if err != nil {
			return nil, fmt.Errorf("failed to add host to host set: %w", err)
		}
	}
	log.Info("initiator group ready", "group", initiatorGroup)
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
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("mapping volume to group", "volume", targetLUN.Name, "group", initiatorGroup)
	lun, err := c.client.EnsureLunMapped(initiatorGroup, targetLUN)
	if err != nil {
		return populator.LUN{}, err
	}
	log.Info("volume mapped successfully", "volume", lun.Name, "group", initiatorGroup)
	return lun, nil
}

// UnMap is responsible to unmapping an initiator group from a LUN
func (c *Primera3ParClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("unmapping volume from group", "volume", targetLUN.Name, "group", initiatorGroup)
	err := c.client.LunUnmap(context.TODO(), initiatorGroup, targetLUN.Name)
	if err != nil {
		return err
	}
	log.Info("volume unmapped successfully", "volume", targetLUN.Name, "group", initiatorGroup)
	return nil
}

// Return initiatorGroups the LUN is mapped to
func (p *Primera3ParClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.V(2).Info("querying current mapped groups", "volume", targetLUN.Name)
	res, err := p.client.CurrentMappedGroups(targetLUN.Name, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to get current mapped groups: %w", err)
	}
	log.V(2).Info("found mapped groups", "volume", targetLUN.Name, "groups", res)
	return res, nil
}

func (c *Primera3ParClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("resolve")
	log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)
	lun := populator.LUN{VolumeHandle: pv.VolumeHandle}
	lun, err := c.client.GetLunDetailsByVolumeName(pv.VolumeHandle, lun)
	if err != nil {
		return populator.LUN{}, err
	}
	log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA)
	return lun, nil
}

// VvolCopy performs a direct copy operation using vSphere API to discover source volume
func (c *Primera3ParClonner) VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	log := klog.Background().WithName(loggerName).WithName("vvol")
	resolveSourceLog := log.WithName("resolve-source")
	resolveTargetLog := log.WithName("resolve-target")
	copyLog := log.WithName("copy")

	resolveSourceLog.Info("VVol copy started", "vm", vmId, "source", sourceVMDKFile)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get VVol disk backing info: %w", err)
	}

	if backing.VVolId == "" {
		return fmt.Errorf("disk %s is not a VVol disk", sourceVMDKFile)
	}

	resolveSourceLog.Info("found VVol backing", "vvol_id", backing.VVolId)

	sourceVolumeName, err := c.findVolumeByVVolID(backing.VVolId, resolveSourceLog)
	if err != nil {
		return fmt.Errorf("failed to find source volume by VVol ID %s: %w", backing.VVolId, err)
	}

	resolveTargetLog.Info("resolving target PV to LUN", "pv", persistentVolume.Name)
	targetLUN, err := c.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	copyLog.Info("copying volume", "source", sourceVolumeName, "target", targetLUN.Name)

	progress <- 10

	err = c.client.CopyVolume(sourceVolumeName, targetLUN.Name)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	progress <- 100
	log.Info("VVol copy completed successfully")
	return nil
}

func (c *Primera3ParClonner) findVolumeByVVolID(vvolID string, log klog.Logger) (string, error) {
	volumes, err := c.client.GetVolumes()
	if err != nil {
		return "", fmt.Errorf("failed to get volumes: %w", err)
	}

	searchID := strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(vvolID, "rfc4122.")), "-", "")
	log.V(2).Info("searching for volume by VVol ID", "vvol_id", vvolID, "search_id", searchID)

	for _, v := range volumes {
		if strings.Contains(strings.ToLower(v.Name), searchID) {
			log.Info("found volume by name match", "volume", v.Name, "vvol_id", vvolID)
			return v.Name, nil
		}
		if strings.ToLower(v.WWN) == searchID {
			log.Info("found volume by WWN match", "volume", v.Name, "wwn", v.WWN, "vvol_id", vvolID)
			return v.Name, nil
		}
	}

	return "", fmt.Errorf("could not find volume matching VVol ID %s", vvolID)
}

func (c *Primera3ParClonner) RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	log := klog.Background().WithName(loggerName).WithName("rdm")
	resolveSourceLog := log.WithName("resolve-source")
	resolveTargetLog := log.WithName("resolve-target")
	copyLog := log.WithName("copy")

	resolveSourceLog.Info("RDM copy started", "vm", vmId)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get RDM disk backing info: %w", err)
	}

	if !backing.IsRDM {
		return fmt.Errorf("disk %s is not an RDM disk", sourceVMDKFile)
	}

	resolveSourceLog.Info("found RDM device", "device", backing.DeviceName)

	sourceLUN, err := c.resolveRDMToLUN(backing.DeviceName, resolveSourceLog)
	if err != nil {
		return fmt.Errorf("failed to resolve RDM device to source LUN: %w", err)
	}

	resolveTargetLog.Info("resolving target PV to LUN", "pv", persistentVolume.Name)
	targetLUN, err := c.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	copyLog.Info("copying volume", "source", sourceLUN.Name, "target", targetLUN.Name)

	progress <- 10

	err = c.client.CopyVolume(sourceLUN.Name, targetLUN.Name)
	if err != nil {
		return fmt.Errorf("3PAR CopyVolume failed: %w", err)
	}

	progress <- 100

	log.Info("RDM copy completed successfully")
	return nil
}

func (c *Primera3ParClonner) resolveRDMToLUN(deviceName string, log klog.Logger) (populator.LUN, error) {
	log.V(2).Info("resolving RDM device to LUN", "device", deviceName)

	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		log.Info("could not extract serial from NAA, trying to find by listing volumes", "device", deviceName, "err", err)
		return c.findVolumeByDeviceName(deviceName, log)
	}

	volumes, err := c.client.GetVolumes()
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get volumes: %w", err)
	}

	log.V(2).Info("searching for volume by serial", "serial", serial)
	for _, v := range volumes {
		if strings.ToLower(v.WWN) == strings.ToLower(serial) {
			lun := populator.LUN{
				Name:         v.Name,
				SerialNumber: v.WWN,
				NAA:          fmt.Sprintf("naa.%s%s", PROVIDER_ID, strings.ToLower(v.WWN)),
			}
			log.Info("resolved source LUN", "lun", lun.Name, "serial", lun.SerialNumber, "naa", lun.NAA)
			return lun, nil
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

func (c *Primera3ParClonner) findVolumeByDeviceName(deviceName string, log klog.Logger) (populator.LUN, error) {
	volumes, err := c.client.GetVolumes()
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to list volumes: %w", err)
	}

	deviceName = strings.ToLower(deviceName)
	log.V(2).Info("searching for volume by device name", "device", deviceName)

	for _, volume := range volumes {
		naa := fmt.Sprintf("naa.%s%s", PROVIDER_ID, strings.ToLower(volume.WWN))

		if strings.Contains(deviceName, strings.ToLower(volume.WWN)) ||
			strings.Contains(deviceName, naa) ||
			deviceName == naa {
			log.Info("found matching volume", "volume", volume.Name, "device", deviceName)
			return populator.LUN{
				Name:         volume.Name,
				SerialNumber: volume.WWN,
				NAA:          fmt.Sprintf("naa.%s%s", PROVIDER_ID, strings.ToLower(volume.WWN)),
			}, nil
		}
	}

	return populator.LUN{}, fmt.Errorf("could not find volume matching RDM device %s", deviceName)
}
