package vantara

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

const decode = true

// Action types
const (
	GETLDEV        = "getLdev"
	ADDPATH        = "addPath"
	DELETEPATH     = "deletePath"
	GETPORTDETAILS = "getPortDetails"
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
		"restServerIP": os.Getenv("STORAGE_URL"),
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
