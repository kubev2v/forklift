package powerstore

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dell/gopowerstore"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/storage"
	"k8s.io/klog/v2"
)

const (
	hostIDContextKey      string = "hostID"
	esxLogicalHostNameKey string = "esxLogicalHostName"
	esxRealHostNameKey    string = "esxRealHostName"

	PowerStoreProviderID = "68ccf098" // Dell PowerStore NAA OUI prefix
)

type PowerstoreClonner struct {
	Client         gopowerstore.Client
	initiatorGroup string
	arrayInfo      populator.StorageArrayInfo
	log            klog.Logger
	hostname       string
	username       string
	password       string
	sslSkipVerify  bool
}

// Ensure PowerstoreClonner implements StorageArrayInfoProvider
var _ populator.StorageArrayInfoProvider = &PowerstoreClonner{}
var _ storage.ArrayIdentifier = &PowerstoreClonner{}

// GetStorageArrayInfo returns metadata about the PowerStore array for metric labels.
func (p *PowerstoreClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return p.arrayInfo
}

// MatchesDevice returns true if the given device name belongs to this PowerStore array.
// It checks whether the device name carries the Dell PowerStore vendor OUI prefix (naa.68ccf098).
func (p *PowerstoreClonner) MatchesDevice(deviceName string) (bool, error) {
	prefix := "naa." + PowerStoreProviderID
	if !strings.HasPrefix(strings.ToLower(deviceName), prefix) {
		p.log.V(1).Info("device does not match vendor prefix", "device", deviceName, "prefix", prefix)
		return false, nil
	}

	lower := strings.ToLower(deviceName)
	p.log.V(1).Info("querying array for volume ownership", "device", lower)
	found, err := p.volumeExistsByWWN(lower)
	if err != nil {
		return false, fmt.Errorf("failed to query volume by WWN %s: %w", deviceName, err)
	}

	if found {
		p.log.V(1).Info("device confirmed on this array", "device", deviceName)
	} else {
		p.log.V(1).Info("volume not found on this array", "device", deviceName)
	}
	return found, nil
}

type powerstoreVolumeResponse []struct {
	Name string `json:"name"`
	WWN  string `json:"wwn"`
}

// volumeExistsByWWN queries the PowerStore REST API directly because the
// gopowerstore SDK only supports GetVolumeByName, not WWN-based lookup.
func (p *PowerstoreClonner) volumeExistsByWWN(wwn string) (bool, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   p.hostname,
		Path:   "/api/rest/volume",
	}
	q := u.Query()
	q.Set("select", "name,wwn")
	q.Set("wwn", "eq."+wwn)
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return false, err
	}
	req.SetBasicAuth(p.username, p.password)

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: p.sslSkipVerify},
	}}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("PowerStore API returned %d", resp.StatusCode)
	}

	var result powerstoreVolumeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return len(result) > 0, nil
}

func (p *PowerstoreClonner) MapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return p.Map(p.initiatorGroup, targetLUN, mappingContext)
}

func (p *PowerstoreClonner) UnmapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return p.UnMap(p.initiatorGroup, targetLUN, mappingContext)
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowerstoreClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	p.log.V(2).Info("querying current mapped groups", "volume", targetLUN.Name)

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
			p.log.Info("failed to get host info", "host_id", mapping.HostID, "err", err)
			continue
		}
		mappedHosts = append(mappedHosts, host.Name)
	}
	if len(mappedHosts) == 0 {
		return nil, fmt.Errorf("volume %s is not mapped to any host", targetLUN.Name)
	}

	p.log.V(2).Info("found mapped groups", "volume", targetLUN.Name, "hosts", mappedHosts)
	return mappedHosts, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowerstoreClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	p.log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", adapterIds)

	p.initiatorGroup = initiatorGroup

	ctx := context.Background()
	mappingContext := make(map[string]any)

	hosts, err := p.Client.GetHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initiator groups: %w", err)
	}
	found, mappingContext, err := getHostByInitiator(adapterIds, &hosts, initiatorGroup, p.log)
	if err != nil {
		return nil, fmt.Errorf("failed to get host by initiator: %w", err)
	}
	if found {
		p.log.Info("initiator group ready", "group", initiatorGroup)
		return mappingContext, nil
	}
	hostGroups, err := p.Client.GetHostGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host groups: %w", err)
	}
	for _, hostGroup := range hostGroups {
		found, mappingContext, err = getHostByInitiator(adapterIds, &hostGroup.Hosts, initiatorGroup, p.log)
		if err != nil {
			return nil, fmt.Errorf("failed to get host by initiator: %w", err)
		}
		if found {
			p.log.Info("initiator group ready", "group", initiatorGroup)
			return mappingContext, nil
		}
	}
	// if no host group found or host, create new host group

	host, err := p.Client.GetHostByName(ctx, initiatorGroup)
	if err != nil {
		p.log.V(2).Info("initiator group not found, creating new", "group", initiatorGroup)
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

		p.log.Info("created initiator group", "group", initiatorGroup, "host_id", host.ID)
	} else {
		p.log.V(2).Info("found existing initiator group", "group", initiatorGroup, "host_id", host.ID)
	}

	mappingContext = createMappingContext(&host, initiatorGroup)

	p.log.Info("initiator group ready", "group", initiatorGroup, "adapter_count", len(adapterIds))
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
	p.log.Info("mapping volume to group", "volume", targetLUN.Name, "group", initiatorGroup)

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

	host, err := p.Client.GetHostByName(ctx, hostName)
	if err != nil {
		return targetLUN, fmt.Errorf("failed to find host for host name %s: %w", hostName, err)
	}

	attachParams := &gopowerstore.HostVolumeAttach{
		VolumeID: &targetLUN.IQN,
	}

	// PowerStore rejects individual host attach/detach when the host belongs to a
	// host group — the volume must be mapped to the group instead.
	if host.HostGroupID != "" {
		p.log.Info("host belongs to host group, attaching volume to host group", "host", hostName, "host_group", host.HostGroupID)

		existing, err := p.Client.GetHostVolumeMappingByVolumeID(ctx, targetLUN.IQN)
		if err != nil {
			p.log.Info("unable to check existing mappings, proceeding with attach", "volume", targetLUN.Name, "err", err)
		} else {
			for _, m := range existing {
				if m.HostGroupID == host.HostGroupID {
					p.log.Info("volume already mapped to host group, skipping attach", "volume", targetLUN.Name, "host_group", host.HostGroupID)
					return targetLUN, nil
				}
			}
		}

		_, err = p.Client.AttachVolumeToHostGroup(ctx, host.HostGroupID, attachParams)
		if err != nil {
			return targetLUN, fmt.Errorf("failed to attach volume %s to host group %s: %w", targetLUN.Name, host.HostGroupID, err)
		}
		p.log.Info("volume mapped to host group", "volume", targetLUN.Name, "host_group", host.HostGroupID)
	} else {
		existing, err := p.Client.GetHostVolumeMappingByVolumeID(ctx, targetLUN.IQN)
		if err != nil {
			p.log.Info("unable to check existing mappings, proceeding with attach", "volume", targetLUN.Name, "err", err)
		} else {
			for _, m := range existing {
				if m.HostID == host.ID {
					p.log.Info("volume already mapped to host, skipping attach", "volume", targetLUN.Name, "host", hostName)
					return targetLUN, nil
				}
			}
		}

		_, err = p.Client.AttachVolumeToHost(ctx, host.ID, attachParams)
		if err != nil {
			return targetLUN, fmt.Errorf("failed to attach volume %s to host %s: %w", targetLUN.Name, host.ID, err)
		}
		p.log.Info("volume mapped to host", "volume", targetLUN.Name, "host", hostName)
	}

	return targetLUN, nil
}

// ResolveVolumeHandleToLUN implements populator.StorageApi.
func (p *PowerstoreClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	p.log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	if pv.VolumeAttributes == nil {
		return populator.LUN{}, fmt.Errorf("PersistentVolume attributes are required")
	}

	name := pv.VolumeAttributes["Name"]
	if name == "" {
		return populator.LUN{}, fmt.Errorf("PersistentVolume 'Name' attribute is required to locate the volume in PowerStore")
	}

	p.log.V(2).Info("looking up volume by name", "name", name)
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
	p.log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA, "provider_id", lun.ProviderID)
	return lun, nil
}

// UnMap implements populator.StorageApi.
func (p *PowerstoreClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	p.log.Info("unmapping volume from group", "volume", targetLUN.Name, "group", initiatorGroup)

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

	detachParams := &gopowerstore.HostVolumeDetach{
		VolumeID: &targetLUN.IQN,
	}
	host, err := p.Client.GetHostByName(ctx, hostName)
	if err != nil {
		return fmt.Errorf("failed to find host for host name %s: %w", hostName, err)
	}

	// PowerStore rejects individual host attach/detach when the host belongs to a
	// host group — the volume must be detached from the group instead.
	if host.HostGroupID != "" {
		p.log.Info("host belongs to host group, detaching volume from host group", "host", hostName, "host_group", host.HostGroupID)
		_, err = p.Client.DetachVolumeFromHostGroup(ctx, host.HostGroupID, detachParams)
		if err != nil {
			return fmt.Errorf("failed to detach volume %s from host group %s: %w", targetLUN.Name, host.HostGroupID, err)
		}
		p.log.Info("volume unmapped from host group", "volume", targetLUN.Name, "host_group", host.HostGroupID)
	} else {
		p.log.Info("detaching volume from host", "volume_id", targetLUN.IQN, "host_id", host.ID)
		_, err = p.Client.DetachVolumeFromHost(ctx, host.ID, detachParams)
		if err != nil {
			return fmt.Errorf("failed to detach volume %s from host %s: %w", targetLUN.Name, host.ID, err)
		}
		p.log.Info("volume unmapped from host", "volume", targetLUN.Name, "host", hostName)
	}

	return nil
}

func NewPowerstoreClonner(hostname, username, password string, sslSkipVerify bool) (PowerstoreClonner, error) {
	log := logger.New("powerstore")

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
		log:           log,
		hostname:      hostname,
		username:      username,
		password:      password,
		sslSkipVerify: sslSkipVerify,
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
