package powerstore

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/dell/gopowerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"k8s.io/klog/v2"
)

const (
	hostIDContextKey      string = "hostID"
	esxLogicalHostNameKey string = "esxLogicalHostName"
	esxRealHostNameKey    string = "esxRealHostName"
	loggerName                   = "copy-offload"
)

type PowerstoreClonner struct {
	Client         gopowerstore.Client
	initiatorGroup string
	arrayInfo      populator.StorageArrayInfo
}

// Ensure PowerstoreClonner implements StorageArrayInfoProvider
var _ populator.StorageArrayInfoProvider = &PowerstoreClonner{}

// GetStorageArrayInfo returns metadata about the PowerStore array for metric labels.
func (p *PowerstoreClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return p.arrayInfo
}

func (p *PowerstoreClonner) MapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return p.Map(p.initiatorGroup, targetLUN, mappingContext)
}

func (p *PowerstoreClonner) UnmapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return p.UnMap(p.initiatorGroup, targetLUN, mappingContext)
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowerstoreClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.V(2).Info("querying current mapped groups", "volume", targetLUN.Name)

	if targetLUN.IQN == "" {
		return nil, fmt.Errorf("target LUN IQN is required")
	}

	ctx := context.Background()
	mappings, err := p.Client.GetHostVolumeMappingByVolumeID(ctx, targetLUN.IQN)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume mappings for %s: %w", targetLUN.Name, err)
	}

	mappedHosts := make([]string, 0, len(mappings))

	for _, mapping := range mappings {
		host, err := p.Client.GetHost(ctx, mapping.HostID)
		if err != nil {
			log.Info("failed to get host info", "host_id", mapping.HostID, "err", err)
			continue
		}
		mappedHosts = append(mappedHosts, host.Name)
	}
	if len(mappedHosts) == 0 {
		return nil, fmt.Errorf("volume %s is not mapped to any host", targetLUN.Name)
	}

	log.V(2).Info("found mapped groups", "volume", targetLUN.Name, "hosts", mappedHosts)
	return mappedHosts, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowerstoreClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	log := klog.Background().WithName(loggerName).WithName("map").WithName("ensure-igroup")
	log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", adapterIds)

	p.initiatorGroup = initiatorGroup

	ctx := context.Background()
	mappingContext := make(map[string]any)

	hosts, err := p.Client.GetHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initiator groups: %w", err)
	}
	found, mappingContext, err := getHostByInitiator(adapterIds, &hosts, initiatorGroup, log)
	if err != nil {
		return nil, fmt.Errorf("failed to get host by initiator: %w", err)
	}
	if found {
		log.Info("initiator group ready", "group", initiatorGroup)
		return mappingContext, nil
	}
	hostGroups, err := p.Client.GetHostGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host groups: %w", err)
	}
	for _, hostGroup := range hostGroups {
		found, mappingContext, err = getHostByInitiator(adapterIds, &hostGroup.Hosts, initiatorGroup, log)
		if err != nil {
			return nil, fmt.Errorf("failed to get host by initiator: %w", err)
		}
		if found {
			log.Info("initiator group ready", "group", initiatorGroup)
			return mappingContext, nil
		}
	}
	// if no host group found or host, create new host group

	host, err := p.Client.GetHostByName(ctx, initiatorGroup)
	if err != nil {
		log.V(2).Info("initiator group not found, creating new", "group", initiatorGroup)
		osType := gopowerstore.OSTypeEnumESXi
		inits := make([]gopowerstore.InitiatorCreateModify, 0, len(adapterIds))
		for _, a := range adapterIds {
			pt, err := detectPortType(a)
			if err != nil {
				return nil, fmt.Errorf("failed to detect port type for adapter %s: %w", a, err)
			}
			portName, err := extractAdapterIdByPortType(a, pt)
			if err != nil {
				return nil, fmt.Errorf("failed to modify WWN by type for adapter %s: %w", a, err)
			}
			inits = append(inits, gopowerstore.InitiatorCreateModify{
				PortName: &portName,
				PortType: &pt,
			})
		}
		createParams := &gopowerstore.HostCreate{
			Name:       &initiatorGroup,
			OsType:     &osType,
			Initiators: &inits,
		}

		createResp, err := p.Client.CreateHost(ctx, createParams)
		if err != nil {
			return nil, fmt.Errorf("failed to create initiator group %s: %w", initiatorGroup, err)
		}

		host, err = p.Client.GetHost(ctx, createResp.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get created initiator group %s: %w", createResp.ID, err)
		}

		log.Info("created initiator group", "group", initiatorGroup, "host_id", host.ID)
	} else {
		log.V(2).Info("found existing initiator group", "group", initiatorGroup, "host_id", host.ID)
	}

	mappingContext = createMappingContext(&host, initiatorGroup)

	log.Info("initiator group ready", "group", initiatorGroup, "adapter_count", len(adapterIds))
	return mappingContext, nil
}

func extractAdapterIdByPortType(adapterId string, portType gopowerstore.InitiatorProtocolTypeEnum) (string, error) {
	switch portType {
	case gopowerstore.InitiatorProtocolTypeEnumISCSI:
		return adapterId, nil
	case gopowerstore.InitiatorProtocolTypeEnumFC:
		wwpn, err := fcutil.ExtractAndFormatWWPN(adapterId)
		if err != nil {
			return "", fmt.Errorf("failed to extract and format WWPN for adapter %s: %w", adapterId, err)
		}
		wwpn = strings.ToLower(wwpn)
		return wwpn, nil
	case gopowerstore.InitiatorProtocolTypeEnumNVME:
		return adapterId, nil
	}
	return "", fmt.Errorf("invalid port type: %s", portType)
}

func getHostByInitiator(adapterIds []string, hosts *[]gopowerstore.Host, initiatorGroup string, log klog.Logger) (bool, populator.MappingContext, error) {
	for _, host := range *hosts {
		for _, initiator := range host.Initiators {
			for _, adapterId := range adapterIds {
				portType, err := detectPortType(adapterId)
				if err != nil {
					return false, populator.MappingContext{}, fmt.Errorf("failed to detect port type for adapter %s: %w", adapterId, err)
				}
				formattedAdapterId, err := extractAdapterIdByPortType(adapterId, portType)
				if err != nil {
					return false, populator.MappingContext{}, fmt.Errorf("failed to extract adapter ID by port type for adapter %s: %w", adapterId, err)
				}
				if initiator.PortName == formattedAdapterId {
					log.V(2).Info("found existing host with matching initiator", "group", initiatorGroup, "host_id", host.ID, "host_name", host.Name, "port_name", initiator.PortName)
					mappingContext := createMappingContext(&host, initiatorGroup)
					return true, mappingContext, nil
				}
			}
		}
	}
	return false, populator.MappingContext{}, nil
}

func createMappingContext(host *gopowerstore.Host, initiatorGroup string) populator.MappingContext {
	mappingContext := populator.MappingContext{
		hostIDContextKey:      host.ID,
		esxLogicalHostNameKey: initiatorGroup,
		esxRealHostNameKey:    host.Name,
	}
	return mappingContext
}

func detectPortType(adapterId string) (gopowerstore.InitiatorProtocolTypeEnum, error) {
	switch {
	case strings.HasPrefix(adapterId, "iqn."):
		return gopowerstore.InitiatorProtocolTypeEnumISCSI, nil
	case strings.HasPrefix(adapterId, "fc."):
		return gopowerstore.InitiatorProtocolTypeEnumFC, nil
	case strings.HasPrefix(adapterId, "nqn."):
		return gopowerstore.InitiatorProtocolTypeEnumNVME, nil
	default:
		return gopowerstore.InitiatorProtocolTypeEnumISCSI, fmt.Errorf("Could not determine port type for adapter ID: %s", adapterId)
	}
}

func (p *PowerstoreClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("mapping volume to group", "volume", targetLUN.Name, "group", initiatorGroup)

	if targetLUN.IQN == "" {
		return targetLUN, fmt.Errorf("target LUN IQN is required")
	}
	if mappingContext == nil {
		return targetLUN, fmt.Errorf("mapping context is required")
	}

	ctx := context.Background()
	hostName := initiatorGroup
	if initiatorGroup == mappingContext[esxLogicalHostNameKey] {
		hostName = mappingContext[esxRealHostNameKey].(string)
	}

	// Get the host by the real PowerStore host name
	host, err := p.Client.GetHostByName(ctx, hostName)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to find host for host name %s: %w", hostName, err)
	}

	hostID := host.ID

	// idempotency: skip attach if already mapped
	existing, err := p.Client.GetHostVolumeMappingByVolumeID(ctx, targetLUN.IQN)
	if err == nil {
		for _, m := range existing {
			if m.HostID == hostID {
				log.V(2).Info("volume already mapped to group", "volume", targetLUN.Name, "host", hostName)
				return targetLUN, nil
			}
		}
	}

	attachParams := &gopowerstore.HostVolumeAttach{
		VolumeID: &targetLUN.IQN,
	}

	log.V(2).Info("attaching volume to host", "volume_id", targetLUN.IQN, "host_id", hostID)
	_, err = p.Client.AttachVolumeToHost(ctx, hostID, attachParams)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to attach volume %s to initiatior group %s: %w", targetLUN.Name, hostID, err)
	}

	log.Info("volume mapped successfully", "volume", targetLUN.Name, "host", hostName)
	return targetLUN, nil
}

// ResolveVolumeHandleToLUN implements populator.StorageApi.
func (p *PowerstoreClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("resolve")
	log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	if pv.VolumeAttributes == nil {
		return populator.LUN{}, fmt.Errorf("PersistentVolume attributes are required")
	}

	name := pv.VolumeAttributes["Name"]
	if name == "" {
		return populator.LUN{}, fmt.Errorf("PersistentVolume 'Name' attribute is required to locate the volume in PowerStore")
	}

	log.V(2).Info("looking up volume by name", "name", name)
	ctx := context.Background()
	volume, err := p.Client.GetVolumeByName(ctx, name)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get volume %s: %w", name, err)
	}

	lun := populator.LUN{
		Name:         name,
		VolumeHandle: pv.VolumeHandle,
		Protocol:     pv.VolumeAttributes["Protocol"],
		NAA:          volume.Wwn, // volume.Wwn contains naa. prefix
		ProviderID:   volume.ID,
		IQN:          volume.ID,
	}
	log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA, "provider_id", lun.ProviderID)
	return lun, nil
}

// UnMap implements populator.StorageApi.
func (p *PowerstoreClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("unmapping volume from group", "volume", targetLUN.Name, "group", initiatorGroup)

	if targetLUN.IQN == "" {
		return fmt.Errorf("target LUN IQN is required")
	}
	if mappingContext == nil {
		return fmt.Errorf("mapping context is required")
	}

	hostName := initiatorGroup
	if initiatorGroup == mappingContext[esxLogicalHostNameKey] {
		hostName = mappingContext[esxRealHostNameKey].(string)
	}
	ctx := context.Background()
	hostID := mappingContext[hostIDContextKey].(string)

	// Detach volume from host
	detachParams := &gopowerstore.HostVolumeDetach{
		VolumeID: &targetLUN.IQN,
	}
	log.V(2).Info("detaching volume from host", "volume_id", targetLUN.IQN, "host_id", hostID)
	_, err := p.Client.DetachVolumeFromHost(ctx, hostID, detachParams)
	if err != nil {
		return fmt.Errorf("failed to detach volume %s from initiator group %s: %w", targetLUN.Name, hostID, err)
	}

	log.Info("volume unmapped successfully", "volume", targetLUN.Name, "host", hostName)
	return nil
}

func NewPowerstoreClonner(hostname, username, password string, sslSkipVerify bool) (PowerstoreClonner, error) {
	log := klog.Background().WithName(loggerName).WithName("setup")

	if hostname == "" {
		return PowerstoreClonner{}, fmt.Errorf("hostname is required")
	}
	if username == "" {
		return PowerstoreClonner{}, fmt.Errorf("username is required")
	}
	if password == "" {
		return PowerstoreClonner{}, fmt.Errorf("password is required")
	}

	log.V(2).Info("creating PowerStore client", "hostname", hostname)
	clientOptions := gopowerstore.NewClientOptions()
	clientOptions.SetInsecure(sslSkipVerify)

	client, err := gopowerstore.NewClientWithArgs(hostname, username, password, clientOptions)
	if err != nil {
		return PowerstoreClonner{}, fmt.Errorf("failed to create PowerStore client: %w", err)
	}

	client.SetCustomHTTPHeaders(http.Header{
		"Application-Type": {"MTV"},
	})

	ctx := context.Background()
	_, err = client.GetCluster(ctx)
	if err != nil {
		return PowerstoreClonner{}, fmt.Errorf("failed to authenticate with PowerStore backend %s: %w", hostname, err)
	}

	log.V(2).Info("authenticated to PowerStore backend", "hostname", hostname)

	clonner := PowerstoreClonner{
		Client: client,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Dell",
			Product: "PowerStore",
		},
	}

	// Fetch software version from the API
	sw, err := client.GetSoftwareInstalled(ctx)
	if err != nil {
		log.Info("failed to get PowerStore software version for metrics", "err", err)
	} else {
		for _, s := range sw {
			if s.IsCluster {
				clonner.arrayInfo.Version = s.ReleaseVersion
				log.V(2).Info("PowerStore array info", "vendor", clonner.arrayInfo.Vendor, "product", clonner.arrayInfo.Product, "version", clonner.arrayInfo.Version)
				break
			}
		}
	}

	return clonner, nil
}
