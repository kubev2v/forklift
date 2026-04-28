package vantara

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/vmware"
	"k8s.io/klog/v2"
)

const (
	VantaraProviderID = "60060e80" // Vantara's NAA prefix
	LengthNAAID       = 32
	loggerName        = "copy-offload"
)

type VantaraCloner struct {
	client          VantaraClient
	envHostGroupIds []string
	initiatorGroup  string
	copySpeed       string
	arrayInfo       populator.StorageArrayInfo
}

// Ensure VantaraCloner implements StorageArrayInfoProvider
var _ populator.StorageArrayInfoProvider = &VantaraCloner{}

// GetStorageArrayInfo returns metadata about the Vantara array for metric labels.
func (v *VantaraCloner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return v.arrayInfo
}

func NewVantaraClonner(hostname, username, password string) (VantaraCloner, error) {
	envStorage, err := getStorageEnvVars()
	if err != nil {
		return VantaraCloner{}, fmt.Errorf("failed to get storage env vars: %w", err)
	}

	// Extract IP from hostname
	decodedIP, err := extractIPAddress(envStorage["restServerIP"].(string))
	if err != nil {
		return VantaraCloner{}, fmt.Errorf("failed to extract IP address: %w", err)
	}

	client := NewBlockStorageAPI(
		decodedIP,
		envStorage["port"].(string),
		envStorage["storageId"].(string),
		envStorage["userID"].(string),
		envStorage["password"].(string),
	)

	// Establish initial connection
	if err := client.Connect(); err != nil {
		return VantaraCloner{}, fmt.Errorf("failed to connect to Vantara storage: %w", err)
	}

	cloner := VantaraCloner{
		client:          client,
		envHostGroupIds: envStorage["hostGroupIds"].([]string),
		copySpeed:       envStorage["copySpeed"].(string),
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Hitachi",
			Product: "Vantara",
		},
	}

	// Fetch model and version from the API
	log := klog.Background().WithName(loggerName).WithName("setup")
	storageInfo, err := client.GetStorageInfo()
	if err != nil {
		log.Info("failed to get Vantara storage info for metrics", "err", err)
	} else {
		cloner.arrayInfo.Model = storageInfo.Model
		cloner.arrayInfo.Version = storageInfo.DkcMicroVersion
		log.V(2).Info("Vantara array info", "vendor", cloner.arrayInfo.Vendor, "product", cloner.arrayInfo.Product, "model", cloner.arrayInfo.Model, "version", cloner.arrayInfo.Version)
	}

	return cloner, nil
}

func (v *VantaraCloner) MapTarget(targetLUN populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	return v.Map(v.initiatorGroup, targetLUN, context)
}

func (v *VantaraCloner) UnmapTarget(targetLUN populator.LUN, context populator.MappingContext) error {
	return v.UnMap(v.initiatorGroup, targetLUN, context)
}

func getStorageEnvVars() (map[string]interface{}, error) {

	envHGs := os.Getenv("HOSTGROUP_ID_LIST")
	hgids := []string{}
	if envHGs != "" {
		items := strings.Split(envHGs, ":")
		for _, item := range items {
			hg := strings.TrimSpace(item)
			if hg != "" {
				hgids = append(hgids, hg)
			}
		}
	}

	copySpeed, found := os.LookupEnv("COPY_SPEED")
	if !found {
		copySpeed = "slower" // default value
	}

	storageEnvVars := map[string]interface{}{
		"storageId":    os.Getenv("STORAGE_ID"),
		"restServerIP": os.Getenv("STORAGE_HOSTNAME"),
		"port":         os.Getenv("STORAGE_PORT"),
		"userID":       os.Getenv("STORAGE_USERNAME"),
		"password":     os.Getenv("STORAGE_PASSWORD"),
		"hostGroupIds": hgids,
		"copySpeed":    copySpeed,
	}
	klog.Info(
		"storageId: ", storageEnvVars["storageId"],
		"restServerIP: ", storageEnvVars["restServerIP"],
		"port: ", storageEnvVars["port"],
		"userID: ", "",
		"password: ", "",
		"hostGroupID: ", storageEnvVars["hostGroupIds"],
		"copySpeed: ", storageEnvVars["copySpeed"],
	)
	return storageEnvVars, nil
}

func (v *VantaraCloner) CurrentMappedGroups(lun populator.LUN, context populator.MappingContext) ([]string, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.V(2).Info("querying current mapped groups", "ldev_id", lun.LDeviceID)

	ldevResp, err := v.client.GetLdev(lun.LDeviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get LDEV: %w", err)
	}

	log.V(2).Info("LDEV response", "ldev", ldevResp)

	hgids := make([]string, 0, len(ldevResp.Ports))
	for _, port := range ldevResp.Ports {
		hostGroupNumber := fmt.Sprintf("%d", int(port.HostGroupNumber))
		hgid := fmt.Sprintf("%s,%s", port.PortId, hostGroupNumber)
		hgids = append(hgids, hgid)
		log.V(2).Info("found mapping", "port_id", port.PortId, "host_group_number", hostGroupNumber)
	}

	log.V(2).Info("found mapped groups", "ldev_id", lun.LDeviceID, "host_group_ids", hgids)
	return hgids, nil
}

func (v *VantaraCloner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("resolve")
	log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	parts := strings.Split(pv.VolumeHandle, "--")
	lun := populator.LUN{}
	if len(parts) != 5 || parts[0] != "01" {
		return lun, fmt.Errorf("invalid volume handle: %s", pv.VolumeHandle)
	}
	ioProtocol := parts[1]
	storageDeviceID := parts[2]
	ldevID := parts[3]
	ldevNickName := parts[4]
	//storageModelID := storageDeviceID[:6]
	//storageSerialNumber := storageDeviceID[6:]

	lun.LDeviceID = ldevID
	//	LDEV := ShowLdev(lun)
	//	ldevnaaid := LDEV["naaId"].(string)
	lun.StorageSerialNumber = storageDeviceID
	lun.Protocol = ioProtocol
	//	lun.ProviderID = ldevnaaid[:6]
	//	lun.SerialNumber = ldevnaaid[6:]
	lun.VolumeHandle = pv.VolumeHandle
	lun.Name = ldevNickName
	log.Info("LUN resolved", "lun", lun.Name, "ldev_id", lun.LDeviceID, "protocol", lun.Protocol)
	return lun, nil
}

func (v *VantaraCloner) GetNaaID(lun populator.LUN) populator.LUN {
	log := klog.Background().WithName(loggerName).WithName("resolve")
	ldevResp, err := v.client.GetLdev(lun.LDeviceID)
	if err != nil {
		log.Info("failed to get LDEV NAA ID", "ldev_id", lun.LDeviceID, "err", err)
		return lun
	}
	lun.ProviderID = ldevResp.NaaId[:6]
	lun.SerialNumber = ldevResp.NaaId[6:]
	lun.NAA = fmt.Sprintf("naa.%s", ldevResp.NaaId)
	log.V(2).Info("NAA ID retrieved", "ldev_id", lun.LDeviceID, "naa", lun.NAA)
	return lun
}

func (v *VantaraCloner) EnsureClonnerIgroup(xcopyInitiatorGroup string, hbaUIDs []string) (populator.MappingContext, error) {
	log := klog.Background().WithName(loggerName).WithName("map").WithName("ensure-igroup")
	log.Info("ensuring initiator group", "group", xcopyInitiatorGroup, "adapters", hbaUIDs)

	v.initiatorGroup = xcopyInitiatorGroup
	if len(v.envHostGroupIds) > 0 {
		log.Info("using host group IDs from environment", "host_group_ids", v.envHostGroupIds)
		log.Info("initiator group ready", "group", xcopyInitiatorGroup)
		return populator.MappingContext{"hostGroupIds": v.envHostGroupIds}, nil
	}

	// Get port details from storage
	log.V(2).Info("fetching host group IDs from storage")
	portDetails, err := v.client.GetPortDetails()
	if err != nil {
		return nil, fmt.Errorf("failed to get port details: %w", err)
	}

	// Convert to JSONData format for compatibility with existing FindHostGroupIDs function
	jsonData := JSONData{Data: portDetails.Data}
	logins := FindHostGroupIDs(jsonData, hbaUIDs)

	jsonBytes, _ := json.MarshalIndent(logins, "", "  ")
	log.V(2).Info("found logins", "logins", string(jsonBytes))

	hostGroupIds := make([]string, len(logins))
	for i, login := range logins {
		hostGroupIds[i] = login.HostGroupId
	}

	log.Info("initiator group ready", "group", xcopyInitiatorGroup, "host_group_ids", hostGroupIds)
	return populator.MappingContext{"hostGroupIds": hostGroupIds}, nil
}

func (v *VantaraCloner) Map(xcopyInitiatorGroup string, lun populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	hostGroupIds := context["hostGroupIds"].([]string)

	for _, hostGroupId := range hostGroupIds {
		parts := strings.SplitN(hostGroupId, ",", 2)
		if len(parts) != 2 {
			return populator.LUN{}, fmt.Errorf("invalid hostGroupId format: %s", hostGroupId)
		}
		portId := parts[0]
		hostGroupNumber := parts[1]

		log.Info("mapping volume to host group", "ldev_id", lun.LDeviceID, "port_id", portId, "host_group", hostGroupNumber)
		if err := v.client.AddPath(lun.LDeviceID, portId, hostGroupNumber); err != nil {
			return populator.LUN{}, fmt.Errorf("failed to add path %s: %w", hostGroupId, err)
		}
	}

	// Get NAA ID after mapping
	lun = v.GetNaaID(lun)
	log.Info("volume mapped successfully", "ldev_id", lun.LDeviceID, "naa", lun.NAA)
	return lun, nil
}

func (v *VantaraCloner) UnMap(xcopyInitiatorGroup string, lun populator.LUN, context populator.MappingContext) error {
	log := klog.Background().WithName(loggerName).WithName("map")
	hostGroupIds := context["hostGroupIds"].([]string)

	// First get the LDEV to find LUN IDs
	ldevResp, err := v.client.GetLdev(lun.LDeviceID)
	if err != nil {
		return fmt.Errorf("failed to get LDEV info: %w", err)
	}

	for _, hostGroupId := range hostGroupIds {
		parts := strings.SplitN(hostGroupId, ",", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid hostGroupId format: %s", hostGroupId)
		}
		portId := parts[0]
		hostGroupNumber := parts[1]

		// Find the LUN ID for this port/hostgroup combination
		lunId, err := getLunIdFromPorts(ldevResp.Ports, portId, hostGroupNumber)
		if err != nil {
			return fmt.Errorf("failed to get LUN ID for %s: %w", hostGroupId, err)
		}

		log.Info("unmapping volume from host group", "ldev_id", lun.LDeviceID, "port_id", portId, "host_group", hostGroupNumber, "lun_id", lunId)
		if err := v.client.DeletePath(lun.LDeviceID, portId, hostGroupNumber, lunId); err != nil {
			return fmt.Errorf("failed to delete path %s: %w", hostGroupId, err)
		}
	}

	log.Info("volume unmapped successfully", "ldev_id", lun.LDeviceID)
	return nil
}

// getLunIdFromPorts finds the LUN ID for a specific port and host group combination
func getLunIdFromPorts(ports []PortMapping, portId string, hostGroupNumber string) (string, error) {
	for _, port := range ports {
		if port.PortId == portId && fmt.Sprintf("%d", int(port.HostGroupNumber)) == hostGroupNumber {
			return fmt.Sprintf("%d", int(port.Lun)), nil
		}
	}
	return "", fmt.Errorf("LUN not found for port %s, hostGroup %s", portId, hostGroupNumber)
}

// VvolCopy performs a direct copy operation using vSphere API to discover source volume
func (v *VantaraCloner) VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
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

	sourceVolumeID, err := v.findVolumeByVVolID(backing.VVolId, resolveSourceLog)
	if err != nil {
		return fmt.Errorf("failed to find source volume by VVol ID %s: %w", backing.VVolId, err)
	}

	resolveTargetLog.Info("resolving target PV to LUN", "pv", persistentVolume.Name)
	targetLUN, err := v.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	copyLog.Info("copying volume", "source", sourceVolumeID, "target", targetLUN.Name)

	// Get target volume pool ID
	ldevResp, err := v.client.GetLdev(targetLUN.LDeviceID)
	copyLog.V(2).Info("target LDEV details", "ldev", ldevResp)

	// Perform the copy operation
	err = v.performVolumeCopy(sourceVolumeID, ldevResp, progress, copyLog)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	log.Info("VVol copy completed successfully")
	return nil
}

// performVolumeCopy executes the volume copy operation on Vantara
func (v *VantaraCloner) performVolumeCopy(sourceLdevId string, ldevResp *LdevResponse, progress chan<- uint64, log klog.Logger) error {

	targetLdevId := fmt.Sprintf("%d", int(ldevResp.LdevId))
	poolID := fmt.Sprintf("%d", int(ldevResp.PoolId))

	// Perform the copy operation using Vantara API
	snapshotGroupName := "mtv-ss-copy-" + sourceLdevId + "-to-" + targetLdevId

	log.V(2).Info("creating clone LDEV", "snapshot_group", snapshotGroupName, "pool_id", poolID, "source_ldev", sourceLdevId, "target_ldev", targetLdevId)
	err := v.client.CreateCloneLdev(snapshotGroupName, poolID, sourceLdevId, targetLdevId, v.copySpeed)
	if err != nil {
		return fmt.Errorf("Vantara CopyVolume failed: %w", err)
	}
	// wait for creation of clone pair
	waittime := 5 // seconds
	maxcount := 30
	count := 0
	found := false
	for {
		if count >= maxcount {
			return fmt.Errorf("timeout waiting for clone pair to be created")
		}
		if count > 0 {
			time.Sleep(time.Duration(waittime) * time.Second)
		}
		count++
		respPairs, err := v.client.GetClonePairs(snapshotGroupName, sourceLdevId)
		if err != nil || len(respPairs.Data) == 0 {
			log.V(2).Info("waiting for clone pair", "attempt", count, "max", maxcount)
			continue
		}
		for _, cp := range respPairs.Data {
			if cp.Status == "PSUP" {
				log.V(2).Info("clone pair created", "pair", cp)
				found = true
				break
			}
		}
		if found {
			break
		}
		log.V(2).Info("waiting for clone pair to be created", "attempt", count, "max", maxcount)
	}
	progress <- 100
	return nil
}

func (v *VantaraCloner) findVolumeByVVolID(vvolID string, log klog.Logger) (string, error) {
	if len(vvolID) < 4 {
		return "", errors.New("VVol ID is too short")
	}

	// Extract the last 4 characters
	last4 := vvolID[len(vvolID)-4:]

	// Parse as hexadecimal to uint64
	value, err := strconv.ParseUint(last4, 16, 64)
	if err != nil {
		return "", err
	}

	// Convert to decimal string and return
	ldevId := strconv.FormatUint(value, 10)
	log.Info("found source volume by VVol ID", "vvol_id", vvolID, "ldev_id", ldevId)
	return ldevId, nil
}

// RDMCopy performs a copy operation for RDM-backed disks using Vantara APIs
func (v *VantaraCloner) RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	log := klog.Background().WithName(loggerName).WithName("rdm")
	resolveSourceLog := log.WithName("resolve-source")
	resolveTargetLog := log.WithName("resolve-target")
	copyLog := log.WithName("copy")

	resolveSourceLog.Info("RDM copy started", "vm", vmId)

	// Get disk backing info to find the RDM device
	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get RDM disk backing info: %w", err)
	}

	if !backing.IsRDM {
		return fmt.Errorf("disk %s is not an RDM disk", sourceVMDKFile)
	}

	resolveSourceLog.Info("found RDM device", "device", backing.DeviceName)

	// Resolve the source LUN from the RDM device name
	sourceLUN, err := v.resolveRDMToLUN(backing.DeviceName, resolveSourceLog)
	if err != nil {
		return fmt.Errorf("failed to resolve RDM device to source LUN: %w", err)
	}

	// Resolve the target PV to LUN
	resolveTargetLog.Info("resolving target PV to LUN", "pv", persistentVolume.Name)
	targetLUN, err := v.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	copyLog.Info("copying volume", "source", sourceLUN.LDeviceID, "target", targetLUN.LDeviceID)

	// Report progress start
	progress <- 10

	// Perform the copy operation using Vantara API
	// Get target volume pool ID
	ldevResp, err := v.client.GetLdev(targetLUN.LDeviceID)
	copyLog.V(2).Info("target LDEV details", "ldev", ldevResp)

	// Perform the copy operation
	err = v.performVolumeCopy(sourceLUN.LDeviceID, ldevResp, progress, copyLog)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	// Report progress complete
	progress <- 100

	log.Info("RDM copy completed successfully")
	return nil
}

// resolveRDMToLUN resolves an RDM device name to a Vantara LDEV ID
func (v *VantaraCloner) resolveRDMToLUN(deviceName string, log klog.Logger) (populator.LUN, error) {
	log.V(2).Info("resolving RDM device to LUN", "device", deviceName)

	deviceName = strings.ToLower(deviceName)
	start := strings.Index(deviceName, VantaraProviderID)
	if start == -1 {
		return populator.LUN{}, fmt.Errorf("target not found")
	}

	if start+LengthNAAID > len(deviceName) {
		return populator.LUN{}, fmt.Errorf("device name too short")
	}

	naaDevice := deviceName[start : start+LengthNAAID]

	ldevIdHex := naaDevice[len(naaDevice)-4:]           // Get last 4 characters for hex ID
	ldevId, err := strconv.ParseUint(ldevIdHex, 16, 64) // Convert hex ID to decimal string
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to parse LDEV ID from device name %s: %w", deviceName, err)
	}

	ldevIds := strconv.FormatUint(ldevId, 10)
	log.V(2).Info("parsing LDEV ID from device", "ldev_id", ldevIds, "hex", ldevIdHex)

	ldevResp, err := v.client.GetLdev(ldevIds)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get LDEV info for device name %s: %w", deviceName, err)
	}

	naa := strings.ToLower(ldevResp.NaaId)

	if naaDevice != naa {
		return populator.LUN{}, fmt.Errorf("device name %s does not match LDEV NAA ID %s", naaDevice, naa)
	}

	lun := populator.LUN{
		LDeviceID: ldevIds,
		NAA:       naa,
	}
	log.Info("resolved source LUN", "ldev_id", lun.LDeviceID, "naa", lun.NAA)
	return lun, nil

}
