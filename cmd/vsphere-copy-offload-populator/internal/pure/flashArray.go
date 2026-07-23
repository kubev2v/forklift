package pure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/storage"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"k8s.io/klog/v2"
)

const FlashProviderID = "624a9370"

// Ensure FlashArrayClonner implements required interfaces
var _ populator.RDMCapable = &FlashArrayClonner{}
var _ populator.VVolCapable = &FlashArrayClonner{}
var _ populator.VMDKCapable = &FlashArrayClonner{}
var _ populator.StorageArrayInfoProvider = &FlashArrayClonner{}
var _ storage.ArrayIdentifier = &FlashArrayClonner{}

type FlashArrayClonner struct {
	restClient    *RestClient
	clusterPrefix string
	// TODO use this instead of mappingContext[hosts]
	initiatorHostOrGroup string
	arrayInfo            populator.StorageArrayInfo
	log                  klog.Logger
}

const ClusterPrefixEnv = "PURE_CLUSTER_PREFIX"
const helpMessage = `clusterPrefix is missing and PURE_CLUSTER_PREFIX is not set.
Use this to extract the value:
printf "px_%s" $(oc get storagecluster -A -o=jsonpath='{.items[0].status.clusterUid}'| head -c 8)
`

// NewFlashArrayClonner creates a new FlashArrayClonner
// Authentication is mutually exclusive:
// - If apiToken is provided (non-empty), it will be used for authentication (username/password ignored)
// - If apiToken is empty, username and password will be used for authentication
func NewFlashArrayClonner(hostname, username, password, apiToken string, skipSSLVerification bool, clusterPrefix string, httpTimeoutSeconds int) (FlashArrayClonner, error) {
	if clusterPrefix == "" {
		return FlashArrayClonner{}, errors.New(helpMessage)
	}

	// Create the REST client for all operations
	restClient, err := NewRestClient(hostname, username, password, apiToken, skipSSLVerification, httpTimeoutSeconds)
	if err != nil {
		return FlashArrayClonner{}, fmt.Errorf("failed to create REST client: %w", err)
	}

	log := logger.New("pure")
	clonner := FlashArrayClonner{
		restClient:    restClient,
		clusterPrefix: clusterPrefix,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Pure Storage",
			Product: "FlashArray",
		},
		log: log,
	}

	// Fetch model and version from the API
	info, err := restClient.GetArrayInfo()
	if err != nil {
		log.Info("failed to get Pure FlashArray info for metrics", "err", err)
	} else {
		clonner.arrayInfo.Model = info.Model
		clonner.arrayInfo.Version = info.Version
		log.V(2).Info("Pure FlashArray info", "vendor", clonner.arrayInfo.Vendor, "product", clonner.arrayInfo.Product, "model", clonner.arrayInfo.Model, "version", clonner.arrayInfo.Version)
	}

	return clonner, nil
}

// GetStorageArrayInfo returns metadata about the Pure FlashArray for metric labels.
func (f *FlashArrayClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return f.arrayInfo
}

// MatchesDevice returns true if the given device name belongs to this Pure FlashArray.
// It first checks the Pure vendor OUI prefix (naa.624a9370) for a fast reject, then
// queries the array API to confirm the volume serial exists on this specific array.
func (f *FlashArrayClonner) MatchesDevice(deviceName string) (bool, error) {
	prefix := "naa." + FlashProviderID
	if !strings.HasPrefix(strings.ToLower(deviceName), prefix) {
		f.log.V(1).Info("device does not match vendor prefix", "device", deviceName, "prefix", prefix)
		return false, nil
	}

	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		return false, fmt.Errorf("failed to extract serial from device name %s: %w", deviceName, err)
	}

	f.log.V(1).Info("querying array for volume ownership", "device", deviceName, "serial", serial)
	_, err = f.restClient.FindVolumeBySerial(serial)
	if err != nil {
		if strings.Contains(err.Error(), "volume not found") || strings.Contains(err.Error(), "Volume not found") {
			f.log.V(1).Info("volume not found on this array", "device", deviceName, "serial", serial)
			return false, nil
		}
		return false, fmt.Errorf("failed to query volume by serial %s: %w", serial, err)
	}

	f.log.V(1).Info("device confirmed on this array", "device", deviceName, "serial", serial)
	return true, nil
}

// EnsureClonnerIgroup creates or updates an initiator group with the ESX adapters
// Named hgroup in flash terminology
func (f *FlashArrayClonner) EnsureClonnerIgroup(initiatorGroup string, esxAdapters []string) (populator.MappingContext, error) {
	f.log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", esxAdapters)

	// pure does not allow a single host to connect to 2 separae groups. Hence
	// we must connect map the volume to the host, and not to the group
	hosts, err := f.restClient.ListHosts()
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		f.log.V(2).Info("checking host", "host", h.Name, "iqns", h.Iqn, "wwns", h.Wwn)
		for _, wwn := range h.Wwn {
			for _, hostAdapter := range esxAdapters {
				if !strings.HasPrefix(hostAdapter, "fc.") {
					continue
				}
				adapterWWPN, err := fcUIDToWWPN(hostAdapter)
				if err != nil {
					f.log.Info("failed to extract WWPN from adapter", "adapter", hostAdapter, "err", err)
					continue
				}

				// Compare WWNs using the utility function that normalizes formatting
				f.log.V(2).Info("comparing WWNs", "adapter_wwpn", adapterWWPN, "host_wwn", wwn)
				if fcutil.CompareWWNs(adapterWWPN, wwn) {
					f.log.Info("found matching host", "host", h.Name)
					f.log.Info("initiator group ready", "group", initiatorGroup, "host", h.Name)
					return populator.MappingContext{"hosts": []string{h.Name}}, nil
				}
			}
		}
		for _, iqn := range h.Iqn {
			if slices.Contains(esxAdapters, iqn) {
				f.log.Info("found matching host by IQN", "host", h.Name, "iqn", iqn)
				f.log.Info("initiator group ready", "group", initiatorGroup, "host", h.Name)
				return populator.MappingContext{"hosts": []string{h.Name}}, nil
			}
		}

	}
	return nil, fmt.Errorf("no hosts found matching any of the provided IQNs/FC adapters: %v", esxAdapters)
}

// Map is responsible to mapping an initiator group to a populator.LUN
func (f *FlashArrayClonner) Map(
	initatorGroup string,
	targetLUN populator.LUN,
	context populator.MappingContext) (populator.LUN, error) {

	hosts, ok := context["hosts"]
	if !ok {
		return populator.LUN{}, fmt.Errorf("hosts not found in context")
	}
	hs, ok := hosts.([]string)
	if !ok || len(hs) == 0 {
		return populator.LUN{}, errors.New("invalid or empty hosts list in mapping context")
	}
	for _, host := range hs {
		f.log.Info("mapping volume to host", "volume", targetLUN.Name, "host", host)
		err := f.restClient.ConnectHost(host, targetLUN.Name)
		if err != nil {
			if strings.Contains(err.Error(), "Connection already exists.") {
				f.log.V(2).Info("volume already mapped to host", "volume", targetLUN.Name, "host", host)
				continue
			}
			return populator.LUN{}, fmt.Errorf("connect host %q to volume %q: %w", host, targetLUN.Name, err)
		}

		f.log.Info("volume mapped successfully", "volume", targetLUN.Name, "host", host)
		return targetLUN, nil
	}
	return populator.LUN{}, fmt.Errorf("connection failed for all hosts in context")
}

func (f *FlashArrayClonner) MapTarget(targetLUN populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	return f.Map(f.initiatorHostOrGroup, targetLUN, context)
}

func (f *FlashArrayClonner) UnmapTarget(targetLUN populator.LUN, context populator.MappingContext) error {
	return f.UnMap(f.initiatorHostOrGroup, targetLUN, context)
}

// UnMap is responsible to unmapping an initiator group from a populator.LUN
func (f *FlashArrayClonner) UnMap(initatorGroup string, targetLUN populator.LUN, context populator.MappingContext) error {
	hosts, ok := context["hosts"]

	if ok {
		hs, ok := hosts.([]string)
		if ok && len(hs) > 0 {
			for _, host := range hs {
				f.log.Info("unmapping volume from host", "volume", targetLUN.Name, "host", host)
				err := f.restClient.DisconnectHost(host, targetLUN.Name)
				if err != nil {
					return err
				}
				f.log.Info("volume unmapped successfully", "volume", targetLUN.Name, "host", host)
			}
		}
	}
	return nil
}

// CurrentMappedGroups returns the initiator groups the populator.LUN is mapped to
func (f *FlashArrayClonner) CurrentMappedGroups(targetLUN populator.LUN, context populator.MappingContext) ([]string, error) {
	f.log.V(2).Info("querying current mapped groups", "volume", targetLUN.Name)
	// we don't use the host group feature, as a host in pure flasharray can not belong to two separate groups, and we
	// definitely don't want to break host from their current groups. insted we'll just map/unmap the volume to individual hosts
	return nil, nil
}

// ResolvePVToLUN resolves a PersistentVolume to Pure FlashArray LUN details
func (f *FlashArrayClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	f.log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	pvVolumeHandle := pv.VolumeHandle
	v, err := f.restClient.GetVolumeById(pvVolumeHandle)
	if err != nil {
		if strings.Contains(err.Error(), "Volume does not exist.") {
			f.log.Info("volume not found by handle, trying by name", "volume_handle", pvVolumeHandle, "err", err)
			volumeName := fmt.Sprintf("%s-%s", f.clusterPrefix, pv.Name)
			v, err = f.restClient.GetVolume(volumeName)
			if err != nil {
				return populator.LUN{}, fmt.Errorf("failed to get volume by name %s: %w", volumeName, err)
			}
		}
	}

	f.log.V(2).Info("volume details", "volume", v)
	l := populator.LUN{Name: v.Name, SerialNumber: v.Serial, NAA: fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(v.Serial))}
	f.log.Info("LUN resolved", "lun", l.Name, "naa", l.NAA, "serial", l.SerialNumber)

	return l, nil
}

// fcUIDToWWPN extracts the WWPN (port name) from an ESXi fcUid string.
// The expected input is of the form: 'fc.WWNN:WWPN' where the WWNN and WWPN
// are not separated with columns every byte (2 hex chars) like 00:00:00:00:00:00:00:00
func fcUIDToWWPN(fcUid string) (string, error) {
	return fcutil.ExtractAndFormatWWPN(fcUid)
}

// VvolCopy performs a direct copy operation using vSphere API to discover source volume
func (f *FlashArrayClonner) VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	f.log.Info("VVol copy started", "vm", vmId, "source", sourceVMDKFile)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get VVol disk backing info: %w", err)
	}

	if backing.VVolID == "" {
		return fmt.Errorf("disk %s is not a VVol disk", sourceVMDKFile)
	}

	f.log.Info("found VVol backing", "vvol_id", backing.VVolID)

	sourceVolume, err := f.restClient.FindVolumeByVVolID(backing.VVolID)
	if err != nil {
		return fmt.Errorf("failed to find source volume by VVol ID %s: %w", backing.VVolID, err)
	}

	f.log.Info("resolving target PV to LUN", "pv", persistentVolume.Name)
	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	f.log.Info("copying volume", "source", sourceVolume, "target", targetLUN.Name)

	err = f.performVolumeCopy(sourceVolume, targetLUN.Name, progress)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	f.log.Info("VVol copy completed successfully")
	return nil
}

// RDMCopy performs a copy operation for RDM-backed disks using Pure FlashArray APIs
func (f *FlashArrayClonner) RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	f.log.Info("RDM copy started", "vm", vmId)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get RDM disk backing info: %w", err)
	}

	if !backing.IsRDM {
		return fmt.Errorf("disk %s is not an RDM disk", sourceVMDKFile)
	}

	f.log.Info("found RDM device", "device", backing.DeviceName)

	sourceLUN, err := f.resolveRDMToLUN(backing.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to resolve RDM device to source LUN: %w", err)
	}

	f.log.Info("resolving target PV to LUN", "pv", persistentVolume.Name)
	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	f.log.Info("copying volume", "source", sourceLUN.Name, "target", targetLUN.Name)

	progress <- 10

	err = f.restClient.CopyVolume(sourceLUN.Name, targetLUN.Name)
	if err != nil {
		return fmt.Errorf("Pure FlashArray CopyVolume failed: %w", err)
	}

	progress <- 100

	f.log.Info("RDM copy completed successfully")
	return nil
}

// resolveRDMToLUN resolves an RDM device name to a Pure FlashArray LUN
func (f *FlashArrayClonner) resolveRDMToLUN(deviceName string) (populator.LUN, error) {
	f.log.V(2).Info("resolving RDM device to LUN", "device", deviceName)

	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		f.log.Info("could not extract serial from NAA, trying to find by listing volumes", "device", deviceName, "err", err)
		return f.findVolumeByDeviceName(deviceName)
	}

	// Find volume by serial number
	f.log.V(2).Info("finding volume by serial", "serial", serial)
	volume, err := f.restClient.FindVolumeBySerial(serial)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to find volume by serial %s: %w", serial, err)
	}

	lun := populator.LUN{
		Name:         volume.Name,
		SerialNumber: volume.Serial,
		NAA:          fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(volume.Serial)),
	}
	f.log.Info("resolved source LUN", "lun", lun.Name, "serial", lun.SerialNumber, "naa", lun.NAA)
	return lun, nil
}

// extractSerialFromNAA extracts the serial number from a NAA identifier
// NAA format for Pure: naa.624a9370<serial> where serial is the volume serial
func extractSerialFromNAA(naa string) (string, error) {
	naa = strings.ToLower(naa)

	// Remove "naa." prefix if present
	naa = strings.TrimPrefix(naa, "naa.")

	// Check if it starts with Pure's provider ID
	providerIDLower := strings.ToLower(FlashProviderID)
	if !strings.HasPrefix(naa, providerIDLower) {
		return "", fmt.Errorf("NAA %s does not appear to be a Pure FlashArray device (expected prefix %s)", naa, FlashProviderID)
	}

	// Extract serial (everything after the provider ID)
	serial := strings.TrimPrefix(naa, providerIDLower)
	if serial == "" {
		return "", fmt.Errorf("could not extract serial from NAA %s", naa)
	}

	return strings.ToUpper(serial), nil
}

// findVolumeByDeviceName finds a volume by searching through all volumes
func (f *FlashArrayClonner) findVolumeByDeviceName(deviceName string) (populator.LUN, error) {
	volumes, err := f.restClient.ListVolumes()
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to list volumes: %w", err)
	}

	deviceName = strings.ToLower(deviceName)
	f.log.V(2).Info("searching for volume by device name", "device", deviceName)

	for _, volume := range volumes {
		// Build the expected NAA for this volume
		naa := fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(volume.Serial))

		// Compare with the device name
		if strings.Contains(deviceName, strings.ToLower(volume.Serial)) ||
			strings.Contains(deviceName, naa) ||
			deviceName == naa {
			f.log.Info("found matching volume", "volume", volume.Name, "device", deviceName)
			return populator.LUN{
				Name:         volume.Name,
				SerialNumber: volume.Serial,
				NAA:          fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(volume.Serial)),
			}, nil
		}
	}

	return populator.LUN{}, fmt.Errorf("could not find volume matching RDM device %s", deviceName)
}

// performVolumeCopy executes the volume copy operation on Pure FlashArray
func (f *FlashArrayClonner) performVolumeCopy(sourceVolumeName, targetVolumeName string, progress chan<- uint64) error {
	f.log.V(2).Info("performing FlashArray copy", "source", sourceVolumeName, "target", targetVolumeName)
	// Perform the copy operation using Pure FlashArray API
	err := f.restClient.CopyVolume(sourceVolumeName, targetVolumeName)
	if err != nil {
		return fmt.Errorf("Pure FlashArray CopyVolume failed: %w", err)
	}

	progress <- 100
	return nil
}
