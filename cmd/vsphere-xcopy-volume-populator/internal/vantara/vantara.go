package vantara

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

type VantaraCloner struct {
	client          VantaraClient
	envHostGroupIds []string
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

	return VantaraCloner{
		client:          client,
		envHostGroupIds: envStorage["hostGroupIds"].([]string),
	}, nil
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

func (v *VantaraCloner) CurrentMappedGroups(lun populator.LUN, context populator.MappingContext) ([]string, error) {
	ldevResp, err := v.client.GetLdev(lun.LDeviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get LDEV: %w", err)
	}

	klog.Infof("LDEV: %+v", ldevResp)

	hgids := make([]string, 0, len(ldevResp.Ports))
	for _, port := range ldevResp.Ports {
		hostGroupNumber := fmt.Sprintf("%d", int(port.HostGroupNumber))
		hgid := fmt.Sprintf("%s,%s", port.PortId, hostGroupNumber)
		hgids = append(hgids, hgid)
		klog.V(2).Infof("Found mapping: portID=%s, hostGroupNumber=%s", port.PortId, hostGroupNumber)
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
	ldevResp, err := v.client.GetLdev(lun.LDeviceID)
	if err != nil {
		klog.Errorf("Failed to get LDEV NAA ID: %v", err)
		return lun
	}
	lun.ProviderID = ldevResp.NaaId[:6]
	lun.SerialNumber = ldevResp.NaaId[6:]
	lun.NAA = fmt.Sprintf("naa.%s", ldevResp.NaaId)
	return lun
}

func (v *VantaraCloner) EnsureClonnerIgroup(xcopyInitiatorGroup string, hbaUIDs []string) (populator.MappingContext, error) {
	if len(v.envHostGroupIds) > 0 {
		klog.Infof("Using host group IDs from environment: %v", v.envHostGroupIds)
		return populator.MappingContext{"hostGroupIds": v.envHostGroupIds}, nil
	}

	// Get port details from storage
	klog.Info("Fetching host group IDs from storage")
	portDetails, err := v.client.GetPortDetails()
	if err != nil {
		return nil, fmt.Errorf("failed to get port details: %w", err)
	}

	// Convert to JSONData format for compatibility with existing FindHostGroupIDs function
	jsonData := JSONData{Data: portDetails.Data}
	logins := FindHostGroupIDs(jsonData, hbaUIDs)

	jsonBytes, _ := json.MarshalIndent(logins, "", "  ")
	klog.Infof("Found logins: %s", string(jsonBytes))

	hostGroupIds := make([]string, len(logins))
	for i, login := range logins {
		hostGroupIds[i] = login.HostGroupId
	}

	klog.Infof("Host group IDs: %v", hostGroupIds)
	return populator.MappingContext{"hostGroupIds": hostGroupIds}, nil
}

func (v *VantaraCloner) Map(xcopyInitiatorGroup string, lun populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	hostGroupIds := context["hostGroupIds"].([]string)

	for _, hostGroupId := range hostGroupIds {
		parts := strings.SplitN(hostGroupId, ",", 2)
		if len(parts) != 2 {
			return populator.LUN{}, fmt.Errorf("invalid hostGroupId format: %s", hostGroupId)
		}
		portId := parts[0]
		hostGroupNumber := parts[1]

		if err := v.client.AddPath(lun.LDeviceID, portId, hostGroupNumber); err != nil {
			return populator.LUN{}, fmt.Errorf("failed to add path %s: %w", hostGroupId, err)
		}
	}

	// Get NAA ID after mapping
	lun = v.GetNaaID(lun)
	return lun, nil
}

func (v *VantaraCloner) UnMap(xcopyInitiatorGroup string, lun populator.LUN, context populator.MappingContext) error {
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

		if err := v.client.DeletePath(lun.LDeviceID, portId, hostGroupNumber, lunId); err != nil {
			return fmt.Errorf("failed to delete path %s: %w", hostGroupId, err)
		}
	}

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
