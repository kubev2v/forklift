package powerstore

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dell/gopowerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const (
	initiatorGroupContextKey string = "initiatorGroup"
	hostIDContextKey         string = "hostID"
	hostNameContextKey       string = "hostName"
)

type PowerstoreClonner struct {
	Client gopowerstore.Client
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

	klog.Infof("Volume %s is currently mapped to hosts: %v", targetLUN.Name, mappedHosts)
	hostName, ok := mappingContext[hostNameContextKey].(string)
	if !ok || hostName == "" {
		return nil, fmt.Errorf("mappingContext missing %q", hostNameContextKey)
	}
	initiatorGroup, ok := mappingContext[initiatorGroupContextKey].(string)
	if !ok || initiatorGroup == "" {
		return nil, fmt.Errorf("mappingContext missing %q", initiatorGroupContextKey)
	}
	if slices.Contains(mappedHosts, hostName) {
		mappedHosts = append(mappedHosts, initiatorGroup)
	}
	return mappedHosts, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowerstoreClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	klog.Infof("ensuring initiator group %s for adapters %v", initiatorGroup, adapterIds)

	ctx := context.Background()
	mappingContext := make(map[string]any)

	hosts, err := p.Client.GetHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initiator groups: %w", err)
	}
	found, mappingContext, err := getHostByInitiator(adapterIds, &hosts, initiatorGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to get host by initiator: %w", err)
	}
	if found {
		return mappingContext, nil
	}
	hostGroups, err := p.Client.GetHostGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host groups: %w", err)
	}
	for _, hostGroup := range hostGroups {
		found, mappingContext, err := getHostByInitiator(adapterIds, &hostGroup.Hosts, initiatorGroup)
		if err != nil {
			return nil, fmt.Errorf("failed to get host by initiator: %w", err)
		}
		if found {
			return mappingContext, nil
		}
	}
	host, err := p.Client.GetHostByName(ctx, initiatorGroup)
	if err != nil {
		klog.Infof("initiator group %s not found, creating new initiator group", initiatorGroup)
		osType := gopowerstore.OSTypeEnumESXi
		inits := make([]gopowerstore.InitiatorCreateModify, 0, len(adapterIds))
		for _, a := range adapterIds {
			pt, err := detectPortType(a)
			if err != nil {
				return nil, fmt.Errorf("failed to detect port type for adapter %s: %w", a, err)
			}
			inits = append(inits, gopowerstore.InitiatorCreateModify{
				PortName: &a,
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

		klog.Infof("Successfully created initiator group %s with ID %s", initiatorGroup, host.ID)
	} else {
		klog.Infof("Found existing initiator group %s with ID %s", initiatorGroup, host.ID)
	}

	// Step 2: Add initiators (adapter IDs) to the host
	atLeastOneAdded := false

	for _, adapterId := range adapterIds {
		portType, err := detectPortType(adapterId)
		if err != nil {
			return nil, fmt.Errorf("failed to detect port type for adapter %s: %w", adapterId, err)
		}
		klog.Infof("Processing adapter %s with type %s", adapterId, portType)

		// Check if initiator already exists on this host
		found := false
		for _, initiator := range host.Initiators {
			if initiator.PortName == adapterId {
				klog.Infof("Initiator %s already exists on initiator group %s", adapterId, initiatorGroup)
				found = true
				atLeastOneAdded = true
				break
			}
		}

		if !found {
			modifyParams := &gopowerstore.HostModify{
				AddInitiators: &[]gopowerstore.InitiatorCreateModify{
					{
						PortName: &adapterId,
						PortType: &portType,
					},
				},
			}

			_, err = p.Client.ModifyHost(ctx, modifyParams, host.ID)
			if err != nil {
				klog.Warningf("Failed to add initiator %s to initiator group %s: %s", adapterId, initiatorGroup, err)
				continue
			}
			klog.Infof("Successfully added initiator %s with port-type %s to initiator-group %s", adapterId, portType, initiatorGroup)
			atLeastOneAdded = true
		}
	}

	if !atLeastOneAdded {
		return nil, fmt.Errorf("failed to add any adapters to initiator group %s", initiatorGroup)
	}

	mappingContext[hostIDContextKey] = host.ID
	mappingContext[hostNameContextKey] = host.Name
	mappingContext[initiatorGroupContextKey] = initiatorGroup

	klog.Infof("Successfully ensured initiator group %s with %d adapters", initiatorGroup, len(adapterIds))
	return mappingContext, nil
}

func getHostByInitiator(adapterIds []string, hosts *[]gopowerstore.Host, initiatorGroup string ) (bool, populator.MappingContext, error) {
	for _, host := range *hosts {
		for _, initiator := range host.Initiators {
			for _, adapterId := range adapterIds {
				if initiator.PortName == adapterId {
					klog.Infof("Found existing initiator group %s with ID %s name %s", initiatorGroup, host.ID, host.Name)
					return true, populator.MappingContext{
						hostIDContextKey:         host.ID,
						hostNameContextKey:       host.Name,
						initiatorGroupContextKey: initiatorGroup,
					}, nil
				}
			}
		}
	}
	return false, populator.MappingContext{}, nil
}

func detectPortType(adapterId string) (gopowerstore.InitiatorProtocolTypeEnum, error) {
	switch {
	case strings.HasPrefix(adapterId, "iqn."):
		return gopowerstore.InitiatorProtocolTypeEnumISCSI
	case strings.HasPrefix(adapterId, "fc."):
		return gopowerstore.InitiatorProtocolTypeEnumFC
	case strings.HasPrefix(adapterId, "nqn."):
		return gopowerstore.InitiatorProtocolTypeEnumNVME
	default:
		return 0, fmt.Errorf("Could not determine port type for adapter ID: %s", adapterId)
	}
}

func (p *PowerstoreClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	if targetLUN.IQN == "" {
		return targetLUN, fmt.Errorf("target LUN IQN is required")
	}
	if mappingContext == nil {
		return targetLUN, fmt.Errorf("mapping context is required")
	}

	hostID, ok := mappingContext[hostIDContextKey].(string)
	if !ok || hostID == "" {
		return targetLUN, fmt.Errorf("host ID not found in mapping context")
	}
	klog.Infof("mapping volume %s to initiator-group %s", targetLUN.Name, initiatorGroup)

	ctx := context.Background()
	// idempotency: skip attach if already mapped
	existing, err := p.Client.GetHostVolumeMappingByVolumeID(ctx, targetLUN.IQN)
	if err == nil {
		for _, m := range existing {
			if m.HostID == hostID {
				klog.Infof("Volume %s already mapped to host %s", targetLUN.Name, initiatorGroup)
				return targetLUN, nil
			}
		}
	}

	attachParams := &gopowerstore.HostVolumeAttach{
		VolumeID: &targetLUN.IQN,
	}

	_, err = p.Client.AttachVolumeToHost(ctx, hostID, attachParams)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to attach volume %s to host %s: %w", targetLUN.Name, hostID, err)
	}

	klog.Infof("Successfully mapped volume %s to initiator-group %s", targetLUN.Name, initiatorGroup)
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

	hostID, ok := mappingContext[hostIDContextKey].(string)
	if !ok || hostID == "" {
		return fmt.Errorf("host ID not found in mapping context")
	}

	klog.Infof("unmapping volume %s from initiator-group %s", targetLUN.Name, initiatorGroup)

	// Detach volume from host
	detachParams := &gopowerstore.HostVolumeDetach{
		VolumeID: &targetLUN.IQN,
	}
	ctx := context.Background()
	_, err := p.Client.DetachVolumeFromHost(ctx, hostID, detachParams)
	if err != nil {
		return fmt.Errorf("failed to detach volume %s from initiator-group %s: %w", targetLUN.Name, hostID, err)
	}

	klog.Infof("Successfully unmapped volume %s from initiator-group %s", targetLUN.Name, initiatorGroup)
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

	return PowerstoreClonner{Client: client}, nil
}
