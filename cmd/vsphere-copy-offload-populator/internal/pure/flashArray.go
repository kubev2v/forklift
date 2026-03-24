package pure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"k8s.io/klog/v2"
)

const FlashProviderID = "624a9370"

// Ensure FlashArrayClonner implements required interfaces
var _ populator.RDMCapable = &FlashArrayClonner{}
var _ populator.VVolCapable = &FlashArrayClonner{}
var _ populator.VMDKCapable = &FlashArrayClonner{}

type FlashArrayClonner struct {
	restClient    *RestClient
	clusterPrefix string
	// TODO use this instead of mappingContext[hosts]
	initiatorHostOrGroup string
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

	return FlashArrayClonner{
		restClient:    restClient,
		clusterPrefix: clusterPrefix,
	}, nil
}

// EnsureClonnerIgroup creates or updates an initiator group with the ESX adapters
// Named hgroup in flash terminology
func (f *FlashArrayClonner) EnsureClonnerIgroup(initiatorGroup string, esxAdapters []string) (populator.MappingContext, error) {
	// pure does not allow a single host to connect to 2 separae groups. Hence
	// we must connect map the volume to the host, and not to the group
	hosts, err := f.restClient.ListHosts()
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		klog.Infof("checking host %s, iqns: %v, wwns: %v", h.Name, h.Iqn, h.Wwn)
		for _, wwn := range h.Wwn {
			for _, hostAdapter := range esxAdapters {
				if !strings.HasPrefix(hostAdapter, "fc.") {
					continue
				}
				adapterWWPN, err := fcUIDToWWPN(hostAdapter)
				if err != nil {
					klog.Warningf("failed to extract WWPN from adapter %s: %s", hostAdapter, err)
					continue
				}

				// Compare WWNs using the utility function that normalizes formatting
				klog.Infof("comparing ESX adapter WWPN %s with Pure host WWN %s", adapterWWPN, wwn)
				if fcutil.CompareWWNs(adapterWWPN, wwn) {
					klog.Infof("match found. Adding host %s to mapping context.", h.Name)
					return populator.MappingContext{"hosts": []string{h.Name}}, nil
				}
			}
		}
		for _, iqn := range h.Iqn {
			if slices.Contains(esxAdapters, iqn) {
				klog.Infof("adding host to group %v", h.Name)
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
		klog.Infof("connecting host %s to volume %s", host, targetLUN.Name)
		err := f.restClient.ConnectHost(host, targetLUN.Name)
		if err != nil {
			if strings.Contains(err.Error(), "Connection already exists.") {
				continue
			}
			return populator.LUN{}, fmt.Errorf("connect host %q to volume %q: %w", host, targetLUN.Name, err)
		}

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
				klog.Infof("disconnecting host %s from volume %s", host, targetLUN.Name)
				err := f.restClient.DisconnectHost(host, targetLUN.Name)
				if err != nil {
					return err
				}

			}
		}
	}
	return nil
}

// CurrentMappedGroups returns the initiator groups the populator.LUN is mapped to
func (f *FlashArrayClonner) CurrentMappedGroups(targetLUN populator.LUN, context populator.MappingContext) ([]string, error) {
	// we don't use the host group feature, as a host in pure flasharray can not belong to two separate groups, and we
	// definitely don't want to break host from their current groups. insted we'll just map/unmap the volume to individual hosts
	return nil, nil
}

// ResolvePVToLUN resolves a PersistentVolume to Pure FlashArray LUN details
func (f *FlashArrayClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	pvVolumeHandle := pv.VolumeHandle
	v, err := f.restClient.GetVolumeById(pvVolumeHandle)
	if err != nil {
		if strings.Contains(err.Error(), "Volume does not exist.") {
			klog.Errorf("Volume with handle %s does not exist: %v. Trying with Volume Name", pvVolumeHandle, err)
			volumeName := fmt.Sprintf("%s-%s", f.clusterPrefix, pv.Name)
			v, err = f.restClient.GetVolume(volumeName)
			if err != nil {
				return populator.LUN{}, fmt.Errorf("failed to get volume by name %s: %w", volumeName, err)
			}
		}
	}

	klog.Infof("volume %+v\n", v)
	l := populator.LUN{Name: v.Name, SerialNumber: v.Serial, NAA: fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(v.Serial))}

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
	klog.Infof("Starting VVol copy operation for VM %s", vmId)

	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get VVol disk backing info: %w", err)
	}

	if backing.VVolId == "" {
		return fmt.Errorf("disk %s is not a VVol disk", sourceVMDKFile)
	}

	klog.Infof("Found VVol backing with ID %s", backing.VVolId)

	sourceVolume, err := f.restClient.FindVolumeByVVolID(backing.VVolId)
	if err != nil {
		return fmt.Errorf("failed to find source volume by VVol ID %s: %w", backing.VVolId, err)
	}

	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	klog.Infof("Copying from source volume %s to target volume %s", sourceVolume, targetLUN.Name)

	err = f.performVolumeCopy(sourceVolume, targetLUN.Name, progress)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	klog.Infof("VVol copy operation completed successfully")
	return nil
}

// RDMCopy performs a copy operation for RDM-backed disks using Pure FlashArray APIs
func (f *FlashArrayClonner) RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	klog.Infof("Pure RDM Copy: Starting RDM copy operation for VM %s", vmId)

	// Get disk backing info to find the RDM device
	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get RDM disk backing info: %w", err)
	}

	if !backing.IsRDM {
		return fmt.Errorf("disk %s is not an RDM disk", sourceVMDKFile)
	}

	klog.Infof("Pure RDM Copy: Found RDM device: %s", backing.DeviceName)

	// Resolve the source LUN from the RDM device name
	sourceLUN, err := f.resolveRDMToLUN(backing.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to resolve RDM device to source LUN: %w", err)
	}

	// Resolve the target PV to LUN
	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	klog.Infof("Pure RDM Copy: Copying from source LUN %s to target LUN %s", sourceLUN.Name, targetLUN.Name)

	// Report progress start
	progress <- 10

	// Perform the copy operation using Pure FlashArray API
	err = f.restClient.CopyVolume(sourceLUN.Name, targetLUN.Name)
	if err != nil {
		return fmt.Errorf("Pure FlashArray CopyVolume failed: %w", err)
	}

	// Report progress complete
	progress <- 100

	klog.Infof("Pure RDM Copy: Copy operation completed successfully")
	return nil
}

// resolveRDMToLUN resolves an RDM device name to a Pure FlashArray LUN
func (f *FlashArrayClonner) resolveRDMToLUN(deviceName string) (populator.LUN, error) {
	klog.Infof("Pure RDM Copy: Resolving RDM device %s to LUN", deviceName)

	// The device name from RDM typically contains the NAA identifier
	// For Pure FlashArray, format is "naa.624a9370<serial>" where 624a9370 is the FlashProviderID
	// We need to extract the serial number and find the corresponding LUN

	// Extract serial number from NAA identifier
	serial, err := extractSerialFromNAA(deviceName)
	if err != nil {
		// Try to find by listing all volumes and matching
		klog.Warningf("Could not extract serial from NAA %s: %v, trying to find by listing volumes", deviceName, err)
		return f.findVolumeByDeviceName(deviceName)
	}

	// Find volume by serial number
	volume, err := f.restClient.FindVolumeBySerial(serial)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to find volume by serial %s: %w", serial, err)
	}

	klog.Infof("Pure RDM Copy: Found matching volume %s for device %s", volume.Name, deviceName)
	return populator.LUN{
		Name:         volume.Name,
		SerialNumber: volume.Serial,
		NAA:          fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(volume.Serial)),
	}, nil
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
	// List all volumes and find the one matching the device name
	volumes, err := f.restClient.ListVolumes()
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to list volumes: %w", err)
	}

	deviceName = strings.ToLower(deviceName)

	for _, volume := range volumes {
		// Build the expected NAA for this volume
		naa := fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(volume.Serial))

		// Compare with the device name
		if strings.Contains(deviceName, strings.ToLower(volume.Serial)) ||
			strings.Contains(deviceName, naa) ||
			deviceName == naa {
			klog.Infof("Pure RDM Copy: Found matching volume %s for device %s", volume.Name, deviceName)
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
	// Perform the copy operation using Pure FlashArray API
	err := f.restClient.CopyVolume(sourceVolumeName, targetVolumeName)
	if err != nil {
		return fmt.Errorf("Pure FlashArray CopyVolume failed: %w", err)
	}

	progress <- 100
	return nil
}
