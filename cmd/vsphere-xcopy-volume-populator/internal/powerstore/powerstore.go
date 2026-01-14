package powerstore

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/dell/gopowerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const (
	hostIDContextKey      string = "hostID"
	esxLogicalHostNameKey string = "esxLogicalHostName"
	esxRealHostNameKey    string = "esxRealHostName"
	adapterIdContextKey   string = "adapterId"
)

type PowerstoreClonner struct {
	Client gopowerstore.Client
	populator.AdapterIdHandlerImpl
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowerstoreClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
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
			klog.Warningf("Failed to get host info for host ID %s: %s", mapping.HostID, err)
			continue
		}
		mappedHosts = append(mappedHosts, host.Name)
	}
	if len(mappedHosts) == 0 {
		return nil, fmt.Errorf("volume %s is not mapped to any host", targetLUN.Name)
	}

	return mappedHosts, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowerstoreClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	ctx := context.Background()
	mappingContext := make(map[string]any)

	hosts, err := p.Client.GetHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initiator groups: %w", err)
	}
	found, mappingContext, err := p.getHostByInitiator(adapterIds, &hosts, initiatorGroup)
	if err != nil {
		klog.Errorf("failed to get host by initiator: %v", err)
	}
	if found {
		return mappingContext, nil
	}
	hostGroups, err := p.Client.GetHostGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host groups: %w", err)
	}
	for _, hostGroup := range hostGroups {
		found, mappingContext, err := p.getHostByInitiator(adapterIds, &hostGroup.Hosts, initiatorGroup)
		if err != nil {
			klog.Errorf("failed to get host by initiator: %v", err)
		}
		if found {
			return mappingContext, nil
		}
	}
	return nil, fmt.Errorf("no host found with adapter IDs %v", adapterIds)
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
		return wwpn, nil
	case gopowerstore.InitiatorProtocolTypeEnumNVME:
		return adapterId, nil
	}
	return "", fmt.Errorf("invalid port type: %s", portType)
}

func (p *PowerstoreClonner) getHostByInitiator(adapterIds []string, hosts *[]gopowerstore.Host, initiatorGroup string) (bool, populator.MappingContext, error) {
	for _, host := range *hosts {
		for _, initiator := range host.Initiators {
			for _, adapterId := range adapterIds {
				portType, err := detectPortType(adapterId)
				if err != nil {
					klog.Errorf("failed to detect port type for adapter %s: %v", adapterId, err)
				}
				formattedAdapterId, err := extractAdapterIdByPortType(adapterId, portType)
				if err != nil {
					klog.Errorf("failed to modify WWN by type for adapter %s: %v", adapterId, err)
				}
				if initiator.PortName == formattedAdapterId {
					klog.Infof("Found existing initiator group %s with ID %s name %s port name %s", initiatorGroup, host.ID, host.Name, initiator.PortName)
					p.AddAdapterID(adapterId)
					return true, createMappingContext(&host, initiatorGroup), nil
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
	if targetLUN.IQN == "" {
		return targetLUN, fmt.Errorf("target LUN IQN is required")
	}
	if mappingContext == nil {
		return targetLUN, fmt.Errorf("mapping context is required")
	}

	klog.Infof("mapping volume %s to initiator-group %s", targetLUN.Name, initiatorGroup)

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
				klog.Infof("Volume %s already mapped to initiatior group %s", targetLUN.Name, hostName)
				return targetLUN, nil
			}
		}
	}

	attachParams := &gopowerstore.HostVolumeAttach{
		VolumeID: &targetLUN.IQN,
	}

	_, err = p.Client.AttachVolumeToHost(ctx, hostID, attachParams)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to attach volume %s to initiatior group %s: %w", targetLUN.Name, hostID, err)
	}

	klog.Infof("Successfully mapped volume %s to initiatior group %s", targetLUN.Name, hostName)
	return targetLUN, nil
}

// ResolveVolumeHandleToLUN implements populator.StorageApi.
func (p *PowerstoreClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	if pv.VolumeAttributes == nil {
		return populator.LUN{}, fmt.Errorf("PersistentVolume attributes are required")
	}

	name := pv.VolumeAttributes["Name"]
	if name == "" {
		return populator.LUN{}, fmt.Errorf("PersistentVolume 'Name' attribute is required to locate the volume in PowerStore")
	}
	ctx := context.Background()
	volume, err := p.Client.GetVolumeByName(ctx, name)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get volume %s: %w", name, err)
	}

	klog.Infof("Successfully resolved volume %s", name)
	return populator.LUN{
		Name:         name,
		VolumeHandle: pv.VolumeHandle,
		Protocol:     pv.VolumeAttributes["Protocol"],
		NAA:          volume.Wwn, // volume.Wwn contains naa. prefix
		ProviderID:   volume.ID,
		IQN:          volume.ID,
	}, nil
}

// UnMap implements populator.StorageApi.
func (p *PowerstoreClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	if targetLUN.IQN == "" {
		return fmt.Errorf("target LUN IQN is required")
	}
	if mappingContext == nil {
		return fmt.Errorf("mapping context is required")
	}

	klog.Infof("unmapping volume %s from initiator-group %s", targetLUN.Name, initiatorGroup)
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
	_, err := p.Client.DetachVolumeFromHost(ctx, hostID, detachParams)
	if err != nil {
		return fmt.Errorf("failed to detach volume %s from initiator group %s: %w", targetLUN.Name, hostID, err)
	}

	klog.Infof("Successfully unmapped volume %s from initiator group %s", targetLUN.Name, hostName)
	return nil
}

func NewPowerstoreClonner(hostname, username, password string, sslSkipVerify bool) (PowerstoreClonner, error) {
	if hostname == "" {
		return PowerstoreClonner{}, fmt.Errorf("hostname is required")
	}
	if username == "" {
		return PowerstoreClonner{}, fmt.Errorf("username is required")
	}
	if password == "" {
		return PowerstoreClonner{}, fmt.Errorf("password is required")
	}

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

	return PowerstoreClonner{Client: client}, nil
}
