package vantara

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
)

const decode = true

// Action types
const (
	GETLDEV        = "getLdev"
	ADDPATH        = "addPath"
	DELETEPATH     = "deletePath"
	GETPORTDETAILS = "getPortDetails"
	CLONELDEV      = "cloneLdev"
	GETCLONEPAIRS  = "getClonePairs"
)

type VantaraCloner struct {
	api VantaraStorageAPI
}

func NewVantaraClonner(hostname, username, password string) (VantaraCloner, error) {
	vantaraObj := make(VantaraObject)
	envStorage, _ := getStorageEnvVars()
	v := getNewVantaraStorageAPIfromEnv(envStorage, vantaraObj)

	return VantaraCloner{api: *v}, nil
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

	storageEnvVars := map[string]interface{}{
		"storageId":    os.Getenv("STORAGE_ID"),
		"restServerIP": os.Getenv("STORAGE_HOSTNAME"),
		"port":         os.Getenv("STORAGE_PORT"),
		"userID":       os.Getenv("STORAGE_USERNAME"),
		"password":     os.Getenv("STORAGE_PASSWORD"),
		"hostGroupIds": hgids,
	}
	klog.Info(
		"storageId: ", storageEnvVars["storageId"],
		"restServerIP: ", storageEnvVars["restServerIP"],
		"port: ", storageEnvVars["port"],
		"userID: ", "",
		"password: ", "",
		"hostGroupID: ", storageEnvVars["hostGroupIds"],
	)
	return storageEnvVars, nil
}

func getNewVantaraStorageAPIfromEnv(envVars map[string]interface{}, vantaraObj VantaraObject) *VantaraStorageAPI {
	vantaraObj["envHostGroupIds"] = envVars["hostGroupIds"].([]string)
	return NewVantaraStorageAPI(envVars["storageId"].(string), envVars["restServerIP"].(string), envVars["port"].(string), envVars["userID"].(string), envVars["password"].(string), vantaraObj)
}

func (v *VantaraCloner) CurrentMappedGroups(lun populator.LUN, context populator.MappingContext) ([]string, error) {
	LDEV := v.ShowLdev(lun)
	klog.Infof("LDEV: %+v", LDEV) // LDEV is a map[string]interface{}

	// Ensure LDEV["ports"] is of type []interface{}
	rawPorts, ok := LDEV["ports"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid type for LDEV['ports'], expected []interface{}")
	}

	hgids := []string{}
	for _, rawPort := range rawPorts {
		portMap, ok := rawPort.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid type for port, expected map[string]interface{}")
		}

		portID, _ := portMap["portId"].(string)

		var hostGroupNumber string
		if hgn, ok := portMap["hostGroupNumber"].(float64); ok {
			hostGroupNumber = fmt.Sprintf("%d", int(hgn))
		} else if hgnStr, ok := portMap["hostGroupNumber"].(string); ok {
			hostGroupNumber = hgnStr
		} else {
			return nil, fmt.Errorf("invalid type for port['hostGroupNumber']")
		}

		hgids = append(hgids, portID+","+hostGroupNumber)
		klog.Infof("portID: %s, hostGroupNumber: %s", portID, hostGroupNumber)
	}
	return hgids, nil
}

func (v *VantaraCloner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
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
	klog.Infof("Resolved LUN: %+v", lun)
	return lun, nil
}

func (v *VantaraCloner) GetNaaID(lun populator.LUN) populator.LUN {
	LDEV := v.ShowLdev(lun)
	ldevnaaid := LDEV["naaId"].(string)
	lun.ProviderID = ldevnaaid[:6]
	lun.SerialNumber = ldevnaaid[6:]
	lun.NAA = fmt.Sprintf("naa.%s", ldevnaaid)
	return lun
}

func (v *VantaraCloner) EnsureClonnerIgroup(xcopyInitiatorGroup string, hbaUIDs []string) (populator.MappingContext, error) {
	if v.api.VantaraObj["envHostGroupIds"] != nil {
		hgids := v.api.VantaraObj["envHostGroupIds"].([]string)
		klog.Infof("HostGroupIDs used from environment variable: %s", hgids)
		return populator.MappingContext{"hostGroupIds": hgids}, nil
	}

	// Get the host group IDs from the storage
	klog.Infof("Fetching host group IDs from storage")
	var r map[string]interface{}

	r, _ = v.api.VantaraStorage(GETPORTDETAILS)

	jsonBytes, err := json.Marshal(r)
	if err != nil {
		klog.Errorf("Error marshalling map to JSON: %s", err)
		return nil, err
	}

	var jsonData JSONData
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		klog.Errorf("Error parsing JSON: %s", err)
		return nil, err
	}

	ret := FindHostGroupIDs(jsonData, hbaUIDs)

	jsonBytes, _ = json.MarshalIndent(ret, "", "  ")
	klog.Infof("HostGroupIDs: %s", string(jsonBytes))

	var hostGroupIds = make([]string, len(ret))
	for i, login := range ret {
		hostGroupIds[i] = login.HostGroupId
	}
	klog.Infof("HostGroupIDs: %s", hostGroupIds)
	return populator.MappingContext{"hostGroupIds": hostGroupIds}, nil
}

func (v *VantaraCloner) Map(xcopyInitiatorGroup string, lun populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	v.api.VantaraObj["ldevId"] = lun.LDeviceID
	v.api.VantaraObj["hostGroupIds"] = context["hostGroupIds"].([]string)
	_, _ = v.api.VantaraStorage(ADDPATH)
	lun = v.GetNaaID(lun)
	return lun, nil
}

func (v *VantaraCloner) UnMap(xcopyInitiatorGroup string, lun populator.LUN, context populator.MappingContext) error {
	v.api.VantaraObj["ldevId"] = lun.LDeviceID
	v.api.VantaraObj["hostGroupIds"] = context["hostGroupIds"].([]string)
	_, _ = v.api.VantaraStorage(DELETEPATH)
	return nil
}

func (v *VantaraCloner) ShowLdev(lun populator.LUN) map[string]interface{} {
	v.api.VantaraObj["ldevId"] = lun.LDeviceID
	r, _ := v.api.VantaraStorage(GETLDEV)
	return r
}

// VvolCopy performs a direct copy operation using vSphere API to discover source volume
func (v *VantaraCloner) VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	klog.Infof("Starting VVol copy operation for VM %s", vmId)

	// Parse the VMDK path
	vmDisk, err := populator.ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to parse VMDK path: %w", err)
	}

	// Resolve target volume details
	targetLUN, err := v.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	// Try to get source volume from vSphere API
	sourceVolumeID, err := v.getSourceVolume(vsphereClient, vmId, vmDisk)
	if err != nil {
		return fmt.Errorf("failed to get source volume from vSphere: %w", err)
	}

	klog.Infof("Copying from source volume %s to target volume %s", sourceVolumeID, targetLUN.Name)

	// Get target volume pool ID
	LDEV := v.ShowLdev(targetLUN)
	klog.Infof("Target LDEV: %v", LDEV)

	// Perform the copy operation
	err = v.performVolumeCopy(sourceVolumeID, LDEV, progress)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	klog.Infof("VVol copy operation completed successfully")
	return nil
}

// getSourceVolume find the vantara volume name for a VMDK
func (v *VantaraCloner) getSourceVolume(vsphereClient vmware.Client, vmId string, vmDisk populator.VMDisk) (string, error) {
	ctx := context.Background()

	// Get VM object from vSphere
	finder := find.NewFinder(vsphereClient.(*vmware.VSphereClient).Client.Client, true)
	vm, err := finder.VirtualMachine(ctx, vmId)
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %w", err)
	}

	// Get VM hardware configuration
	var vmObject mo.VirtualMachine
	pc := property.DefaultCollector(vsphereClient.(*vmware.VSphereClient).Client.Client)
	err = pc.RetrieveOne(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmObject)
	if err != nil {
		return "", fmt.Errorf("failed to get VM hardware config: %w", err)
	}

	// Look through VM's virtual disks to find VVol backing
	if vmObject.Config == nil || vmObject.Config.Hardware.Device == nil {
		return "", fmt.Errorf("VM config or hardware devices not found")
	}

	for _, device := range vmObject.Config.Hardware.Device {
		if disk, ok := device.(*types.VirtualDisk); ok {
			if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
				// Check if this is a VVol backing and matches our target VMDK
				if backing.BackingObjectId != "" && v.matchesVMDKPath(backing.FileName, vmDisk) {
					klog.Infof("Found VVol backing for VMDK %s with ID %s", vmDisk.VmdkFile, backing.BackingObjectId)

					// Use REST client to find the volume by VVol ID
					volumeID, err := v.api.FindVolumeByVVolID(backing.BackingObjectId)
					if err != nil {
						klog.Warningf("Failed to find volume by VVol ID %s: %v", backing.BackingObjectId, err)
						continue
					}

					return volumeID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("VVol backing for VMDK %s not found", vmDisk.VmdkFile)
}

// matchesVMDKPath checks if a vSphere VVol filename matches the target VMDK
func (f *VantaraCloner) matchesVMDKPath(fileName string, vmDisk populator.VMDisk) bool {
	fileBase := filepath.Base(fileName)
	targetBase := filepath.Base(vmDisk.VmdkFile)
	return fileBase == targetBase
}

// performVolumeCopy executes the volume copy operation on Vantara
func (v *VantaraCloner) performVolumeCopy(sourceVolumeId string, ldev map[string]interface{}, progress chan<- uint64) error {
	ldevID, err := extractStringField(ldev, "ldevId")
	if err != nil {
		return fmt.Errorf("invalid target LDEV id: %w", err)
	}

	poolID, err := extractStringField(ldev, "poolId")
	if err != nil {
		return fmt.Errorf("invalid target LDEV pool id: %w", err)
	}

	// Perform the copy operation using Vantara API
	v.api.VantaraObj["snapshotGroupName"] = "mtv-ss-copy-" + sourceVolumeId + "-to-" + ldevID
	v.api.VantaraObj["snapshotPoolId"] = poolID
	v.api.VantaraObj["sourceLdevId"] = sourceVolumeId
	v.api.VantaraObj["targetLdevId"] = ldevID
	_, err = v.api.VantaraStorage(CLONELDEV)
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
		r, err := v.api.VantaraStorage(GETCLONEPAIRS)
		if err != nil {
			return fmt.Errorf("Vantara GetClonePairs failed: %w", err)
		}
		dataAny, ok := r["data"]
		if !ok || dataAny == nil {
			klog.Infof("Waiting... (no data field yet)")
			continue
		}
		for _, item := range dataAny.([]interface{}) {
			clonePair, ok := item.(map[string]interface{})
			if !ok {
				klog.Infof("Waiting... (invalid clone pair data)")
				continue
			}
			if fmt.Sprint(clonePair["status"]) == "PSUP" {
				klog.Infof("Clone pair created: %v", clonePair)
				found = true
				break
			}
		}
		if found {
			break
		}
		klog.Infof("Waiting for clone pair to be created...")
	}
	progress <- 100
	return nil
}

func extractStringField(data map[string]interface{}, key string) (string, error) {
	val, ok := data[key]
	if !ok {
		return "", fmt.Errorf("field %q not found in LDEV data", key)
	}

	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("field %q is empty", key)
		}
		return v, nil
	case json.Number:
		return v.String(), nil
	case fmt.Stringer:
		s := v.String()
		if strings.TrimSpace(s) == "" {
			return "", fmt.Errorf("field %q is empty", key)
		}
		return s, nil
	default:
		s := fmt.Sprintf("%v", v)
		if strings.TrimSpace(s) == "" || s == "<nil>" {
			return "", fmt.Errorf("field %q could not be converted to string", key)
		}
		return s, nil
	}
}
