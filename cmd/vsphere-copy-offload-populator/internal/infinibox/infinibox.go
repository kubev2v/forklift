package infinibox

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/infinidat/infinibox-csi-driver/iboxapi"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"k8s.io/klog/v2"
)

const (
	hostIDContextKey      string = "hostID"
	esxLogicalHostNameKey string = "esxLogicalHostName"
	esxRealHostNameKey    string = "esxRealHostName"
	ocpRealHostNameKey    string = "ocpRealHostName"
)

type InfiniboxClonner struct {
	api            iboxapi.Client
	initiatorGroup string
	arrayInfo      populator.StorageArrayInfo
	log            klog.Logger
}

// Ensure InfiniboxClonner implements StorageArrayInfoProvider
var _ populator.StorageArrayInfoProvider = &InfiniboxClonner{}

// GetStorageArrayInfo returns metadata about the InfiniBox array for metric labels.
func (c *InfiniboxClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return c.arrayInfo
}

func (c *InfiniboxClonner) MapTarget(targetLUN populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	return c.Map(c.initiatorGroup, targetLUN, context)
}

func (c *InfiniboxClonner) UnmapTarget(targetLUN populator.LUN, context populator.MappingContext) error {
	return c.UnMap(c.initiatorGroup, targetLUN, context)
}

func (c *InfiniboxClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	if mappingContext == nil {
		return targetLUN, fmt.Errorf("mapping context is required")
	}

	hostName := ""
	if initiatorGroup != mappingContext[esxLogicalHostNameKey] {
		hostName = mappingContext[ocpRealHostNameKey].(string)
	} else {
		hostName = mappingContext[esxRealHostNameKey].(string)
	}
	c.log.Info("mapping volume to group", "volume", targetLUN.Name, "group", hostName)

	host, err := c.api.GetHostByName(context.TODO(), hostName)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to find host for host name %s: %w", hostName, err)
	}

	volumeID, err := strconv.Atoi(targetLUN.LDeviceID)
	if err != nil {
		return targetLUN, fmt.Errorf("invalid volume ID %q, expected integer: %w", targetLUN.LDeviceID, err)
	}
	// Idempotency: check if already mapped
	existingMappings, err := c.api.GetLunsByVolume(context.TODO(), volumeID)
	if err == nil {
		for _, mapping := range existingMappings {
			if mapping.HostID == host.ID {
				c.log.V(2).Info("volume already mapped to group", "volume", targetLUN.Name, "group", hostName)
				return targetLUN, nil
			}
		}
	}

	c.log.V(2).Info("mapping volume to host", "volume_id", volumeID, "host_id", host.ID)
	_, err = c.api.MapVolumeToHost(context.TODO(), host.ID, volumeID, 0)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to map volume %s to host %s: %w", targetLUN.Name, hostName, err)
	}

	c.log.Info("volume mapped successfully", "volume", targetLUN.Name, "group", hostName)
	return targetLUN, nil
}

func (c *InfiniboxClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	if mappingContext == nil {
		return fmt.Errorf("mapping context is required")
	}

	hostName := ""
	if initiatorGroup != mappingContext[esxLogicalHostNameKey] {
		hostName = mappingContext[ocpRealHostNameKey].(string)
	} else {
		hostName = mappingContext[esxRealHostNameKey].(string)
	}
	c.log.Info("unmapping volume from group", "volume", targetLUN.Name, "group", hostName)

	host, err := c.api.GetHostByName(context.TODO(), hostName)
	if err != nil {
		return fmt.Errorf("failed to find host for host name %s: %w", hostName, err)
	}

	volumeID, err := strconv.Atoi(targetLUN.LDeviceID)
	if err != nil {
		return fmt.Errorf("failed to convert volume ID %s to integer: %w", targetLUN.LDeviceID, err)
	}

	c.log.V(2).Info("unmapping volume from host", "volume_id", volumeID, "host_id", host.ID)
	_, err = c.api.UnMapVolumeFromHost(context.TODO(), host.ID, volumeID)
	if err != nil {
		return fmt.Errorf("failed to unmap volume %s from host %s: %w", targetLUN.Name, hostName, err)
	}

	c.log.Info("volume unmapped successfully", "volume", targetLUN.Name, "group", hostName)
	return nil
}

func (c *InfiniboxClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	c.log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", adapterIds)

	c.initiatorGroup = initiatorGroup
	hosts, err := c.api.GetAllHosts(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts: %w", err)
	}

	for _, host := range hosts {
		for _, port := range host.Ports {
			for _, adapterId := range adapterIds {
				if strings.HasPrefix(adapterId, "fc.") {
					wwpn, err := fcutil.ExtractWWPN(adapterId)
					if err != nil {
						c.log.Info("failed to extract WWPN from adapter ID", "adapter", adapterId, "err", err)
						continue
					}
					if fcutil.CompareWWNs(wwpn, port.Address) {
						c.log.Info("found host with matching adapter", "host", host.Name, "adapter", adapterId, "port_address", port.Address)
						c.log.Info("initiator group ready", "group", initiatorGroup, "host", host.Name)
						return createMappingContext(&host, initiatorGroup), nil
					}
				} else {
					if port.Address == adapterId {
						c.log.Info("found host with matching adapter", "host", host.Name, "adapter", adapterId, "port_address", port.Address)
						c.log.Info("initiator group ready", "group", initiatorGroup, "host", host.Name)
						return createMappingContext(&host, initiatorGroup), nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("no host found with adapter IDs %v", adapterIds)
}

func createMappingContext(host *iboxapi.Host, initiatorGroup string) populator.MappingContext {
	return populator.MappingContext{
		hostIDContextKey:      host.ID,
		esxLogicalHostNameKey: initiatorGroup,
		esxRealHostNameKey:    host.Name,
	}
}

func NewInfiniboxClonner(hostname, username, password string, insecure bool) (InfiniboxClonner, error) {
	log := logger.New("infinibox")
	log.V(2).Info("creating InfiniBox client", "hostname", hostname)

	// Create credentials
	creds := iboxapi.Credentials{
		Username: username,
		Password: password,
		URL:      hostname,
	}

	// Create InfiniBox client
	client := iboxapi.NewIboxClient(creds)

	clonner := InfiniboxClonner{
		api: client,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Infinidat",
			Product: "InfiniBox",
		},
		log: log,
	}

	// Fetch model and version from the API
	sysInfo, err := client.GetSystem(context.TODO())
	if err != nil {
		log.Info("failed to get InfiniBox system info for metrics", "err", err)
	} else {
		clonner.arrayInfo.Model = sysInfo.Model
		clonner.arrayInfo.Version = sysInfo.Version
		log.V(2).Info("InfiniBox array info", "vendor", clonner.arrayInfo.Vendor, "product", clonner.arrayInfo.Product, "model", clonner.arrayInfo.Model, "version", clonner.arrayInfo.Version)
	}

	return clonner, nil
}

func (c *InfiniboxClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	c.log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	volumeAttributes := pv.VolumeAttributes
	volumeName := volumeAttributes["Name"]
	c.log.V(2).Info("looking up volume by name", "name", volumeName)

	volume, err := c.api.GetVolumeByName(context.TODO(), volumeName)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get volume by name %s: %w", volumeName, err)
	}

	serial := volume.Serial
	protocol := volumeAttributes["storage_protocol"]
	protocolPrefix := ""
	switch protocol {
	case "iscsi":
		protocolPrefix = "iqn"
	default:
		protocolPrefix = "naa"
	}
	IQN := fmt.Sprintf("%s.%s", protocolPrefix, serial)
	NAA := fmt.Sprintf("naa.6%s", serial)

	lun := populator.LUN{
		Name:         volumeName,
		LDeviceID:    strconv.Itoa(volume.ID),
		VolumeHandle: pv.VolumeHandle,
		SerialNumber: serial,
		IQN:          IQN,
		NAA:          NAA,
	}
	c.log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA, "serial", lun.SerialNumber, "protocol", protocol)
	return lun, nil
}

func (c *InfiniboxClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	c.log.V(2).Info("querying current mapped groups", "volume", targetLUN.LDeviceID)

	volumeID := targetLUN.LDeviceID

	volumeIDInt, err := strconv.Atoi(volumeID)
	if err != nil {
		return nil, fmt.Errorf("invalid volume ID '%s', expected integer volume ID: %w", volumeID, err)
	}

	c.log.V(2).Info("checking mappings for volume", "volume_id", volumeIDInt)
	lunInfos, err := c.api.GetLunsByVolume(context.TODO(), volumeIDInt)
	if err != nil {
		return nil, fmt.Errorf("failed to get LUN mappings for volume %s: %w", volumeID, err)
	}

	c.log.V(2).Info("found LUN mappings", "volume", volumeID, "mapping_count", len(lunInfos))

	if len(lunInfos) == 0 {
		c.log.V(2).Info("volume not mapped to any hosts", "volume", volumeID)
		return []string{}, nil
	}

	allHosts, err := c.api.GetAllHosts(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to get all hosts: %w", err)
	}

	hostByID := make(map[int]*iboxapi.Host)
	for i := range allHosts {
		hostByID[allHosts[i].ID] = &allHosts[i]
	}

	mappedHosts := make([]string, 0, len(lunInfos))
	hostIDsProcessed := make(map[int]bool)

	for _, lunInfo := range lunInfos {
		if hostIDsProcessed[lunInfo.HostID] {
			continue
		}

		if lunInfo.CLustered {
			c.log.Error(nil, "volume mapped to host cluster (cluster mappings not fully supported)", "volume", volumeID, "host_cluster_id", lunInfo.HostClusterID)
			continue
		}

		host, exists := hostByID[lunInfo.HostID]
		if !exists {
			c.log.Info("failed to find host info", "host_id", lunInfo.HostID)
			continue
		}

		mappedHosts = append(mappedHosts, host.Name)
		hostIDsProcessed[lunInfo.HostID] = true

		if mappingContext != nil {
			if _, ok := mappingContext[ocpRealHostNameKey]; !ok {
				mappingContext[ocpRealHostNameKey] = host.Name
				c.log.V(2).Info("volume currently mapped to host", "volume", volumeID, "host", host.Name)
				return mappedHosts, nil
			}
		}

		c.log.V(2).Info("volume mapped to host", "volume", volumeID, "host", host.Name, "host_id", lunInfo.HostID, "lun", lunInfo.Lun)
	}

	if len(mappedHosts) == 0 {
		return nil, fmt.Errorf("volume %s is not mapped to any host", volumeID)
	}

	c.log.V(2).Info("found mapped groups", "volume", volumeID, "hosts", mappedHosts)
	return mappedHosts, nil
}
