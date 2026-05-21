package powermax

//go:generate mockgen -destination=mock_powermax_client_test.go -package=powermax github.com/dell/gopowermax/v2 Pmax

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	gopowermax "github.com/dell/gopowermax/v2"
	pmxtypes "github.com/dell/gopowermax/v2/types/v100"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/logger"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type PowermaxClonner struct {
	client         gopowermax.Pmax
	symmetrixID    string
	portGroup      string
	initiatorID    string
	storageGroupID string
	hostID         string
	maskingViewID  string
	arrayInfo      populator.StorageArrayInfo
	log            klog.Logger
}

// Ensure PowermaxClonner implements StorageArrayInfoProvider
var _ populator.StorageArrayInfoProvider = &PowermaxClonner{}

// GetStorageArrayInfo returns metadata about the PowerMax array for metric labels.
func (p *PowermaxClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return p.arrayInfo
}

func (p *PowermaxClonner) MapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return p.Map(p.initiatorID, targetLUN, mappingContext)
}

func (p *PowermaxClonner) UnmapTarget(targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return p.UnMap(p.initiatorID, targetLUN, mappingContext)
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowermaxClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	p.log.V(2).Info("querying current mapped groups", "volume", targetLUN.ProviderID)

	ctx := context.TODO()
	volume, err := p.client.GetVolumeByID(ctx, p.symmetrixID, targetLUN.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("Error getting volume %s: %v", targetLUN.ProviderID, err)
	}

	if len(volume.StorageGroups) == 0 {
		return nil, fmt.Errorf("Volume %s is not associated with any Storage Group.\n", targetLUN.ProviderID)
	}

	p.log.V(2).Info("volume storage groups", "volume", targetLUN.ProviderID, "storage_groups", volume.StorageGroups)

	foundHostGroups := []string{}

	for _, sgID := range volume.StorageGroups {
		foundHostGroups = append(foundHostGroups, sgID.StorageGroupName)
		maskingViewList, err := p.client.GetMaskingViewList(ctx, p.symmetrixID)
		if err != nil {
			p.log.Info("failed to get masking views for storage group", "storage_group", sgID, "err", err)
			continue
		}

		if len(maskingViewList.MaskingViewIDs) == 0 {
			p.log.V(2).Info("no masking views found for storage group", "storage_group", sgID)
			continue
		}

		// Step 3: Get details of each Masking View to find the Host Group
		for _, mvID := range maskingViewList.MaskingViewIDs {
			maskingView, err := p.client.GetMaskingViewByID(ctx, p.symmetrixID, mvID)
			if err != nil {
				p.log.Info("failed to get masking view", "masking_view", mvID, "err", err)
				continue
			}

			if maskingView.HostID != "" {
				// This masking view is directly mapped to a Host, not a Host Group
				p.log.V(2).Info("volume mapped via masking view to host", "volume", targetLUN.ProviderID, "masking_view", mvID, "host", maskingView.HostID)
				foundHostGroups = append(foundHostGroups, maskingView.HostID)
			} else if maskingView.HostGroupID != "" {
				// This masking view is mapped to a Host Group
				p.log.V(2).Info("volume mapped via masking view to host group", "volume", targetLUN.ProviderID, "masking_view", mvID, "host_group", maskingView.HostGroupID)
				foundHostGroups = append(foundHostGroups, maskingView.HostGroupID)
			}
		}
	}

	if len(foundHostGroups) > 0 {
		p.log.V(2).Info("found mapped groups", "volume", targetLUN.ProviderID, "groups", foundHostGroups)
	} else {
		p.log.V(2).Info("no host groups found for volume", "volume", targetLUN.ProviderID)
	}
	return foundHostGroups, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowermaxClonner) EnsureClonnerIgroup(_ string, clonnerIqn []string) (populator.MappingContext, error) {
	p.log.Info("ensuring initiator group", "adapters", clonnerIqn)

	ctx := context.TODO()

	randomString, err := generateRandomString(4)
	if err != nil {
		return nil, err
	}
	p.initiatorID = fmt.Sprintf("xcopy-%s", randomString)
	p.log.V(2).Info("generated unique initiator group name", "group", p.initiatorID)

	// steps:
	// 1.create the storage group
	// 2. create a masking view, add the storage group to it - name it with the same name
	// 3. create InitiatorGroup on the masking view
	// 4. add clonnerIqn to that initiar group
	// 5. add port group with protocol type that match the cloner IQN type, only if they all online
	p.storageGroupID = fmt.Sprintf("%s-SG", p.initiatorID)
	p.log.V(2).Info("ensuring storage group exists", "storage_group", p.storageGroupID)
	err = retryOnTransient(ctx, p.log, "GetStorageGroup", func() error {
		_, e := p.client.GetStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		return e
	})
	if err == nil {
		p.log.V(2).Info("storage group exists", "storage_group", p.storageGroupID)
	} else {
		var pmxErr *pmxtypes.Error
		if errors.As(err, &pmxErr) && pmxErr.HTTPStatusCode == 404 {
			p.log.V(2).Info("creating storage group", "storage_group", p.storageGroupID)
			err = retryOnTransient(ctx, p.log, "CreateStorageGroup", func() error {
				_, e := p.client.CreateStorageGroup(ctx, p.symmetrixID, p.storageGroupID, "none", "", true, nil)
				return e
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create group: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to check storage group %s: %w", p.storageGroupID, err)
		}
	}

	p.log.V(2).Info("storage group ready", "storage_group", p.storageGroupID)

	// Fetch port group to determine protocol type
	var portGroup *pmxtypes.PortGroup
	err = retryOnTransient(ctx, p.log, "GetPortGroupByID", func() error {
		var e error
		portGroup, e = p.client.GetPortGroupByID(ctx, p.symmetrixID, p.portGroup)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get port group %s: %w", p.portGroup, err)
	}
	p.log.V(2).Info("port group protocol", "port_group", p.portGroup, "protocol", portGroup.PortGroupProtocol)

	// Filter initiators based on port group protocol
	filteredInitiators := filterInitiatorsByProtocol(clonnerIqn, portGroup.PortGroupProtocol, p.log)
	if len(filteredInitiators) == 0 {
		return nil, fmt.Errorf("no initiators matching protocol %s found in %v", portGroup.PortGroupProtocol, clonnerIqn)
	}
	p.log.V(2).Info("filtered initiators by protocol", "protocol", portGroup.PortGroupProtocol, "initiators", filteredInitiators)

	// Direct initiator lookup (1 API call per initiator instead of N+1)
	for _, filteredInit := range filteredInitiators {
		lookupID := initiatorToLookupID(filteredInit)
		var initiator *pmxtypes.Initiator
		err = retryOnTransient(ctx, p.log, "GetInitiatorByID", func() error {
			var e error
			initiator, e = p.client.GetInitiatorByID(ctx, p.symmetrixID, lookupID)
			return e
		})
		if err != nil {
			return nil, fmt.Errorf("failed to look up initiator %s: %w", lookupID, err)
		}
		hostID := initiator.HostID
		if hostID == "" {
			hostID = initiator.Host
		}
		if hostID != "" {
			p.hostID = hostID
			p.log.Info("found matching host", "host_id", p.hostID, "initiator", lookupID)
			break
		}
	}
	if p.hostID == "" {
		return nil, fmt.Errorf("can't find a host on symmetrix %s with initiators matching %v. "+
			"Ensure the ESXi host has a corresponding host object in PowerMax with the correct FC/iSCSI initiators registered",
			p.symmetrixID, filteredInitiators)
	}
	p.log.Info("found matching host", "host_id", p.hostID, "protocol", portGroup.PortGroupProtocol)

	p.log.V(2).Info("port group configured", "port_group", p.portGroup)
	p.log.Info("initiator group ready", "group", p.initiatorID)
	mappingContext := map[string]any{}
	return mappingContext, err
}

// Map implements populator.StorageApi.
func (p *PowermaxClonner) Map(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	p.log.Info("mapping volume to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)

	ctx := context.TODO()
	volumesMapped, err := p.client.GetVolumeIDListInStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
	if err != nil {
		return targetLUN, err
	}
	if slices.Contains(volumesMapped, targetLUN.ProviderID) {
		p.log.V(2).Info("volume already mapped to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)
		return targetLUN, nil
	}

	p.log.V(2).Info("adding volume to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)
	err = p.client.AddVolumesToStorageGroupS(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
	if err != nil {
		p.log.Info("failed to add volume to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID, "err", err)
		return targetLUN, err
	}

	mv, err := p.client.GetMaskingViewByID(ctx, p.symmetrixID, p.initiatorID)
	if err != nil {
		// probably not found, will be created later
		if e, ok := err.(*pmxtypes.Error); ok && e.HTTPStatusCode == 404 {
			p.log.V(2).Info("masking view not found, will be created", "initiator_id", p.initiatorID)
		} else {
			return populator.LUN{}, err
		}
	}

	if mv == nil {
		p.log.V(2).Info("creating masking view", "initiator_id", p.initiatorID, "storage_group", p.storageGroupID, "host_id", p.hostID, "port_group", p.portGroup)
		mv, err = p.client.CreateMaskingView(ctx, p.symmetrixID, p.initiatorID, p.storageGroupID, p.hostID, false, p.portGroup)
		if err != nil {
			return populator.LUN{}, err
		}
	}

	p.log.Info("volume mapped successfully", "volume", targetLUN.ProviderID, "masking_view", mv.MaskingViewID)
	p.maskingViewID = mv.MaskingViewID
	return targetLUN, err
}

// ResolvePVToLUN implements populator.StorageApi.
func (p *PowermaxClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	p.log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	ctx := context.TODO()
	volID := pv.VolumeHandle[strings.LastIndex(pv.VolumeHandle, "-")+1:]
	p.log.V(2).Info("extracting volume ID from handle", "volume_id", volID)

	volume, err := p.client.GetVolumeByID(ctx, p.symmetrixID, volID)
	if err != nil || volume.VolumeID == "" {
		return populator.LUN{}, fmt.Errorf("failed getting details for volume %v: %v", volume, err)
	}

	naa := fmt.Sprintf("naa.%s", volume.WWN)
	lun := populator.LUN{Name: volume.VolumeIdentifier, ProviderID: volume.VolumeID, NAA: naa}
	p.log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA, "provider_id", lun.ProviderID)
	return lun, nil
}

// UnMap implements populator.StorageApi.
func (p *PowermaxClonner) UnMap(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	p.log.Info("unmapping volume from storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)

	ctx := context.TODO()

	cleanup, ok := mappingContext[populator.CleanupXcopyInitiatorGroup]
	if ok && cleanup.(bool) {
		p.log.V(2).Info("full cleanup requested, deleting masking view", "masking_view", p.maskingViewID)
		err := p.client.DeleteMaskingView(ctx, p.symmetrixID, p.maskingViewID)
		if err != nil {
			return fmt.Errorf("failed to delete masking view: %w", err)
		}

		p.log.V(2).Info("removing volume from storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)
		_, err = p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
		if err != nil {
			return fmt.Errorf("failed removing volume from storage group:  %w", err)
		}

		p.log.V(2).Info("deleting storage group", "storage_group", p.storageGroupID)
		err = p.client.DeleteStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		if err != nil {
			return fmt.Errorf("failed to delete storage group: %w", err)
		}

		p.log.Info("volume unmapped successfully with full cleanup", "volume", targetLUN.ProviderID)
		return nil
	}

	p.log.V(2).Info("removing volume from storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)

	_, err := p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
	if err != nil {
		return fmt.Errorf("failed removing volume from storage group:  %w", err)
	}

	p.log.Info("volume unmapped successfully", "volume", targetLUN.ProviderID)
	return nil
}

var newClientWithArgs = gopowermax.NewClientWithArgs

func NewPowermaxClonner(hostname, username, password string, sslSkipVerify bool) (PowermaxClonner, error) {
	log := logger.New("powermax")

	symID := os.Getenv("POWERMAX_SYMMETRIX_ID")
	if symID == "" {
		return PowermaxClonner{}, fmt.Errorf("Please set POWERMAX_SYMMETRIX_ID in the pod environment or in the secret" +
			" attached to the relevant storage map")
	}
	portGroup := os.Getenv("POWERMAX_PORT_GROUP_NAME")
	if portGroup == "" {
		return PowermaxClonner{}, fmt.Errorf("Please set POWERMAX_PORT_GROUP_NAME in the pod environment or in the secret" +
			" attached to the relevant storage map")
	}

	log.V(2).Info("creating PowerMax client", "hostname", hostname, "symmetrix_id", symID, "port_group", portGroup)

	// using the same application name as the driver
	applicationName := "csi"
	client, err := newClientWithArgs(
		hostname,
		applicationName,
		sslSkipVerify,
		false,
		"")

	if err != nil {
		return PowermaxClonner{}, err
	}

	c := gopowermax.ConfigConnect{
		Endpoint: hostname,
		Version:  "",
		Username: username,
		Password: password,
	}
	err = client.Authenticate(context.TODO(), &c)
	if err != nil {
		return PowermaxClonner{}, err
	}

	log.V(2).Info("authenticated to PowerMax", "symmetrix_id", symID)

	clonner := PowermaxClonner{
		client:      client,
		symmetrixID: symID,
		portGroup:   portGroup,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Dell",
			Product: "PowerMax",
		},
		log: log,
	}

	// Fetch model and version from the API
	sym, err := client.GetSymmetrixByID(context.TODO(), symID)
	if err != nil {
		log.Info("failed to get PowerMax symmetrix info for metrics", "err", err)
	} else {
		clonner.arrayInfo.Model = sym.Model
		clonner.arrayInfo.Version = sym.Ucode
		log.V(2).Info("PowerMax array info", "vendor", clonner.arrayInfo.Vendor, "product", clonner.arrayInfo.Product, "model", clonner.arrayInfo.Model, "version", clonner.arrayInfo.Version)
	}

	return clonner, nil
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// retryOnTransient retries the given function with exponential backoff when the
// error is a transient Unisphere 503 Service Unavailable response.
func retryOnTransient(ctx context.Context, log klog.Logger, operation string, fn func() error) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    5,
	}
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(_ context.Context) (bool, error) {
		lastErr = fn()
		if lastErr == nil {
			return true, nil
		}
		var pmxErr *pmxtypes.Error
		if errors.As(lastErr, &pmxErr) && pmxErr.HTTPStatusCode == 503 {
			log.Info("transient 503 error, retrying", "operation", operation, "err", lastErr)
			return false, nil
		}
		return false, lastErr
	})
	if err != nil && lastErr != nil && !errors.Is(err, lastErr) {
		return fmt.Errorf("%w: %w", err, lastErr)
	}
	return err
}

// initiatorToLookupID strips the "fc." prefix from FC-style initiator identifiers
// so the raw WWN can be used for PowerMax API lookups (e.g., GetInitiatorByID).
func initiatorToLookupID(initiator string) string {
	return strings.TrimPrefix(initiator, "fc.")
}

// filterInitiatorsByProtocol filters the initiator list based on the port group protocol
// iSCSI protocol requires IQN format initiators (e.g., "iqn.1994-05.com.redhat:...")
// SCSI_FC protocol requires FC WWN format initiators (e.g., "10000000c9a12345:10000000c9a12346")
func filterInitiatorsByProtocol(initiators []string, protocol string, log klog.Logger) []string {
	var filtered []string

	for _, initiator := range initiators {
		switch protocol {
		case "iSCSI":
			// iSCSI initiators start with "iqn."
			if strings.HasPrefix(strings.ToLower(initiator), "iqn.") {
				filtered = append(filtered, initiator)
			}
		case "SCSI_FC":
			// FC initiators are in WWNN:WWPN format (hex pairs separated by colon)
			// They don't start with "iqn." and typically contain colons
			if !strings.HasPrefix(strings.ToLower(initiator), "iqn.") && strings.Contains(initiator, ":") {
				filtered = append(filtered, initiator)
			}
		default:
			log.Info("unknown protocol, skipping initiator filtering", "protocol", protocol)
			// For unknown protocols, return all initiators
			return initiators
		}
	}

	return filtered
}
