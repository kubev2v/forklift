package infinibox

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/amitosw15/infinibox-csi-driver/iboxapi"
	"github.com/go-logr/logr"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

type InfiniboxClonner struct {
	api          iboxapi.Client
}

// Map the targetLUN to the initiator group.
func (c *InfiniboxClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	host, err := c.api.GetHostByName(initiatorGroup)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to find host %s: %w", initiatorGroup, err)
	}

	volume, err := c.api.GetVolumeByName(targetLUN.Name)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to find volume %s: %w", targetLUN.Name, err)
	}

	// Step 3: Map the volume to the host
	_, err = c.api.MapVolumeToHost(volume.ID, host.ID, 0) // This grants the host access to the volume
	if err != nil {
		return targetLUN, fmt.Errorf("failed to map volume %s to host %s: %w", volume.Name, host.Name, err)
	}

	klog.Infof("Successfully mapped volume %s to host %s", volume.Name, host.Name)
	return targetLUN, nil
}

func (c *InfiniboxClonner) UnMap(initatorGroup string, targetLUN populator.LUN, _ populator.MappingContext) error {
	// Unmap the volume from the host (initiator group)
	host, err := c.api.GetHostByName(initatorGroup)
	if err != nil {
		return fmt.Errorf("failed to find host %s: %w", initatorGroup, err)
	}

	volume, err := c.api.GetVolumeByName(targetLUN.Name)
	if err != nil {
		return fmt.Errorf("failed to find volume %s: %w", targetLUN.Name, err)
	}

	_, err = c.api.UnMapVolumeFromHost(host.ID, volume.ID)
	if err != nil {
		return fmt.Errorf("failed to unmap volume %s from host %s: %w", volume.Name, host.Name, err)
	}

	klog.Infof("Successfully unmapped volume %s from host %s", volume.Name, host.Name)
	return nil
}

func (c *InfiniboxClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	host, err := c.api.GetHostByName(initiatorGroup)
	if err != nil {
		klog.Infof("Host %s not found, creating new host", initiatorGroup)
		host, err = c.api.CreateHost(initiatorGroup)
		if err != nil {
			return nil, fmt.Errorf("failed creating host %s: %w", initiatorGroup, err)
		}
		klog.Infof("Successfully created host %s with ID %d", initiatorGroup, host.ID)
	} else {
		klog.Infof("Found existing host %s with ID %d", initiatorGroup, host.ID)
	}

	atLeastOneAdded := false

	for _, adapterId := range adapterIds {
		// Step 2a: Determine port type using heuristics
		portType := detectPortType(adapterId)
		klog.Infof("Detected port type %s for adapter %s", portType, adapterId)

		// Step 2b: Check if port already exists on this host
		existingPort, err := c.api.GetHostPort(host.ID, adapterId)
		if err == nil && existingPort != nil {
			klog.Infof("Port %s already exists on host %s", adapterId, initiatorGroup)
			atLeastOneAdded = true
			continue
		}

		// Step 2c: Add the port to the host
		_, err = c.api.AddHostPort(portType, adapterId, host.ID)
		if err != nil {
			klog.Warningf("failed adding port %s to host %s: %s", adapterId, initiatorGroup, err)
			continue
		}
		klog.Infof("Successfully added %s port %s to host %s", portType, adapterId, initiatorGroup)
		atLeastOneAdded = true
	}

	if !atLeastOneAdded {
		return nil, fmt.Errorf("failed adding any adapter to host %s", initiatorGroup)
	}

	klog.Infof("Successfully ensured host %s with %d adapters", initiatorGroup, len(adapterIds))
	return nil, nil
}

func detectPortType(adapterId string) string {
	adapterId = strings.ToLower(strings.TrimSpace(adapterId))

	// iSCSI IQN patterns
	if strings.HasPrefix(adapterId, "iqn.") {
		return "iscsi"
	}

	// Fibre Channel patterns
	if strings.HasPrefix(adapterId, "fc.") {
		return "fc"
	}

	// NVMe-oF patterns
	if strings.HasPrefix(adapterId, "nqn.") {
		return "nvme"
	}

	// Legacy FC without prefix (just in case)
	cleanId := strings.ReplaceAll(strings.ReplaceAll(adapterId, ":", ""), "-", "")
	if len(cleanId) == 16 && isHexString(cleanId) {
		return "fc"
	}

	// Default to iSCSI if we can't determine
	klog.Warningf("Could not determine port type for %s, defaulting to iscsi", adapterId)
	return "iscsi"
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, char := range s {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}

func NewInfiniboxClonner(hostname, username, password string) (InfiniboxClonner, error) {
	// Get network space from environment variable, similar to ONTAP SVM
	networkSpace := os.Getenv("INFINIBOX_NETWORK_SPACE")
	if networkSpace == "" {
		networkSpace = "default" // fallback to default
	}

	// Create credentials
	creds := iboxapi.Credentials{
		Username: username,
		Password: password,
		Url:      hostname,
	}

	// Create logger (using klog adapter)
	logger := logr.Discard() // You can replace with proper logger if needed

	// Create InfiniBox client
	client := iboxapi.NewIboxClient(logger, creds)

	return InfiniboxClonner{
		api:          client,
		networkSpace: networkSpace,
	}, nil
}

func (c *InfiniboxClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	parts := strings.SplitN(pv.VolumeHandle, "$$", 2)
	if len(parts) != 2 {
		return populator.LUN{}, fmt.Errorf("invalid VolumeHandle format: %s, expected 'string$$string'", pv.VolumeHandle)
	}
	volumeName := parts[0]
	volumeIDStr := parts[1]
	volumeID, err := strconv.Atoi(volumeIDStr)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("invalid volume ID in VolumeHandle: %s, error: %w", pv.VolumeHandle, err)
	}

	// Get the volume from InfiniBox
	vol, err := c.api.GetVolume(volumeID)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get volume by ID %d: %w", volumeID, err)
	}

	// Use the serial number from the volume as the LUN serial
	serial := vol.Serial

	// Compose the LUN name (InfiniBox doesn't have a "LUN name" per se, so use volume name)
	l := struct {
		Name         string
		SerialNumber string
	}{
		Name:         vol.Name,
		SerialNumber: serial,
	}
	lun := populator.LUN{Name: l.Name, VolumeHandle: pv.VolumeHandle, SerialNumber: l.SerialNumber}
	return lun, nil
}

func (c *InfiniboxClonner) CurrentMappedGroups(targetLUN populator.LUN, _ populator.MappingContext) ([]string, error) {
	// Convert volume handle to volume ID
	volumeID, err := strconv.Atoi(targetLUN.VolumeHandle)
	if err != nil {
		return nil, fmt.Errorf("invalid volume handle %s, expected integer volume ID: %w", targetLUN.VolumeHandle, err)
	}

	// Get all LUN mappings for this volume
	lunInfos, err := c.api.GetLunsByVolume(volumeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get LUN mappings for volume %d: %w", volumeID, err)
	}

	if len(lunInfos) == 0 {
		klog.Infof("Volume %d (%s) is not mapped to any hosts", volumeID, targetLUN.Name)
		return []string{}, nil
	}

	hostNames := []string{}
	hostIDsProcessed := make(map[int]bool)

	for _, lunInfo := range lunInfos {
		if hostIDsProcessed[lunInfo.HostID] {
			continue
		}

		host, err := c.getHostByID(lunInfo.HostID)
		if err != nil {
			klog.Warningf("Failed to get host info for host ID %d: %s", lunInfo.HostID, err)
			continue
		}

		hostNames = append(hostNames, host.Name)
		hostIDsProcessed[lunInfo.HostID] = true

		klog.Infof("Volume %d is mapped to host %s (ID: %d) as LUN %d",
			volumeID, host.Name, lunInfo.HostID, lunInfo.Lun)
	}

	klog.Infof("Volume %d (%s) is currently mapped to %d host(s): %v",
		volumeID, targetLUN.Name, len(hostNames), hostNames)

	return hostNames, nil
}

// getHostByID is a helper function to get host by ID since the API only provides GetHostByName and GetAllHosts
func (c *InfiniboxClonner) getHostByID(hostID int) (*iboxapi.Host, error) {
	allHosts, err := c.api.GetAllHosts()
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts: %w", err)
	}

	for _, host := range allHosts {
		if host.ID == hostID {
			return &host, nil
		}
	}

	return nil, fmt.Errorf("host with ID %d not found", hostID)
}
