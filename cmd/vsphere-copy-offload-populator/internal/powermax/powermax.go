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
	var volume *pmxtypes.Volume
	err := retryOnTransient(ctx, p.log, "GetVolumeByID", func() error {
		var e error
		volume, e = p.client.GetVolumeByID(ctx, p.symmetrixID, targetLUN.ProviderID)
		return e
	})
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
		var maskingViewList *pmxtypes.MaskingViewList
		err := retryOnTransient(ctx, p.log, "GetMaskingViewList", func() error {
			var e error
			maskingViewList, e = p.client.GetMaskingViewList(ctx, p.symmetrixID)
			return e
		})
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
			var maskingView *pmxtypes.MaskingView
			err := retryOnTransient(ctx, p.log, "GetMaskingViewByID", func() error {
				var e error
				maskingView, e = p.client.GetMaskingViewByID(ctx, p.symmetrixID, mvID)
				return e
			})
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
	p.maskingViewID = ""
	p.storageGroupID = ""

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
	protocol, err := resolvePortGroupProtocol(portGroup, p.log)
	if err != nil {
		return nil, fmt.Errorf("port group %s: %w", p.portGroup, err)
	}
	p.log.V(2).Info("port group protocol", "port_group", p.portGroup, "protocol", protocol)

	// Filter initiators based on port group protocol
	filteredInitiators := filterInitiatorsByProtocol(clonnerIqn, protocol, p.log)
	if len(filteredInitiators) == 0 {
		return nil, fmt.Errorf("no initiators matching protocol %s found in %v", protocol, clonnerIqn)
	}
	p.log.V(2).Info("filtered initiators by protocol", "protocol", protocol, "initiators", filteredInitiators)

	// Direct initiator lookup (1 API call per initiator instead of N+1)
	for _, filteredInit := range filteredInitiators {
		lookupID := initiatorToLookupID(filteredInit)
		var initiator *pmxtypes.Initiator

		if protocol == "SCSI_FC" {
			// FC initiators from ESXi are in WWNN:WWPN format, but the PowerMax API
			// expects initiator IDs in <director>:<port>:<wwn> format (e.g., OR-2C:0:10000000c99debc3).
			// Use GetInitiatorList with the WWPN to find the correct PowerMax initiator ID.
			wwpn := extractWWPN(lookupID)
			var initList *pmxtypes.InitiatorList
			err = retryOnTransient(ctx, p.log, "GetInitiatorList", func() error {
				var e error
				initList, e = p.client.GetInitiatorList(ctx, p.symmetrixID, wwpn, false, true)
				return e
			})
			if err != nil {
				return nil, fmt.Errorf("failed to list initiators for WWPN %s: %w", wwpn, err)
			}
			if len(initList.InitiatorIDs) == 0 {
				p.log.V(2).Info("no initiators found for WWPN", "wwpn", wwpn)
				continue
			}
			pmxInitID := initList.InitiatorIDs[0]
			p.log.V(2).Info("resolved FC initiator", "wwpn", wwpn, "powermax_id", pmxInitID)
			err = retryOnTransient(ctx, p.log, "GetInitiatorByID", func() error {
				var e error
				initiator, e = p.client.GetInitiatorByID(ctx, p.symmetrixID, pmxInitID)
				return e
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get initiator %s: %w", pmxInitID, err)
			}
		} else {
			// For iSCSI, the IQN is the initiator ID — direct lookup works
			err = retryOnTransient(ctx, p.log, "GetInitiatorByID", func() error {
				var e error
				initiator, e = p.client.GetInitiatorByID(ctx, p.symmetrixID, lookupID)
				return e
			})
			if err != nil {
				return nil, fmt.Errorf("failed to look up initiator %s: %w", lookupID, err)
			}
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
	p.log.Info("found matching host", "host_id", p.hostID, "protocol", protocol)

	p.log.V(2).Info("port group configured", "port_group", p.portGroup)
	p.log.Info("initiator group ready", "group", p.initiatorID)
	mappingContext := map[string]any{}
	return mappingContext, err
}

// Map implements populator.StorageApi.
// On PowerMax, the volume is never removed from its original storage groups during xcopy,
// so re-mapping after cleanup is a no-op. Only the initial xcopy mapping (via MapTarget)
// needs to add the volume to the temporary xcopy storage group.
func (p *PowermaxClonner) Map(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	// storageGroupID is a guard to decide if we need to map.
	if p.storageGroupID == "" {
		p.log.V(2).Info("skipping Map after cleanup, volume remains in original storage groups", "volume", targetLUN.ProviderID)
		return targetLUN, nil
	}

	p.log.Info("mapping volume to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)

	ctx := context.TODO()
	var volumesMapped []string
	err := retryOnTransient(ctx, p.log, "GetVolumeIDListInStorageGroup", func() error {
		var e error
		volumesMapped, e = p.client.GetVolumeIDListInStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		return e
	})
	if err != nil {
		return targetLUN, err
	}
	if slices.Contains(volumesMapped, targetLUN.ProviderID) {
		p.log.V(2).Info("volume already mapped to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)
		return targetLUN, nil
	}

	p.log.V(2).Info("adding volume to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)
	err = retryOnTransient(ctx, p.log, "AddVolumesToStorageGroupS", func() error {
		return p.client.AddVolumesToStorageGroupS(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
	})
	if err != nil {
		p.log.Info("failed to add volume to storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID, "err", err)
		return targetLUN, err
	}

	var mv *pmxtypes.MaskingView
	err = retryOnTransient(ctx, p.log, "GetMaskingViewByID", func() error {
		var e error
		mv, e = p.client.GetMaskingViewByID(ctx, p.symmetrixID, p.initiatorID)
		return e
	})
	if err != nil {
		// probably not found, will be created later
		var pmxErr *pmxtypes.Error
		if errors.As(err, &pmxErr) && pmxErr.HTTPStatusCode == 404 {
			p.log.V(2).Info("masking view not found, will be created", "initiator_id", p.initiatorID)
		} else {
			return populator.LUN{}, err
		}
	}

	if mv == nil {
		p.log.V(2).Info("creating masking view", "initiator_id", p.initiatorID, "storage_group", p.storageGroupID, "host_id", p.hostID, "port_group", p.portGroup)
		err = retryOnTransient(ctx, p.log, "CreateMaskingView", func() error {
			var e error
			mv, e = p.client.CreateMaskingView(ctx, p.symmetrixID, p.initiatorID, p.storageGroupID, p.hostID, false, p.portGroup)
			return e
		})
		if err != nil {
			return populator.LUN{}, err
		}
		// If mv is still nil after a 409 (treated as success), the masking view
		// was created by a prior attempt — fetch it.
		if mv == nil {
			err = retryOnTransient(ctx, p.log, "GetMaskingViewByID", func() error {
				var e error
				mv, e = p.client.GetMaskingViewByID(ctx, p.symmetrixID, p.initiatorID)
				return e
			})
			if err != nil {
				return populator.LUN{}, fmt.Errorf("masking view %s exists (409) but failed to fetch it: %w", p.initiatorID, err)
			}
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

	var volume *pmxtypes.Volume
	err := retryOnTransient(ctx, p.log, "GetVolumeByID", func() error {
		var e error
		volume, e = p.client.GetVolumeByID(ctx, p.symmetrixID, volID)
		return e
	})
	if err != nil || volume.VolumeID == "" {
		return populator.LUN{}, fmt.Errorf("failed getting details for volume %v: %v", volume, err)
	}

	naa := fmt.Sprintf("naa.%s", volume.WWN)
	lun := populator.LUN{Name: volume.VolumeIdentifier, ProviderID: volume.VolumeID, NAA: naa}
	p.log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA, "provider_id", lun.ProviderID)
	return lun, nil
}

// UnMap implements populator.StorageApi.
// Masking view deletion is a blocking step: PowerMax rejects storage group
// deletion while the SG is still attached to a masking view, so any non-404
// DeleteMaskingView failure aborts the cleanup and returns immediately.
// A 404 on DeleteMaskingView is treated as success (idempotent retry).
func (p *PowermaxClonner) UnMap(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	p.log.Info("unmapping volume from storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)

	ctx := context.TODO()
	var errs []error

	if p.maskingViewID != "" {
		p.log.V(2).Info("deleting masking view", "masking_view", p.maskingViewID)
		err := retryOnTransient(ctx, p.log, "DeleteMaskingView", func() error {
			return p.client.DeleteMaskingView(ctx, p.symmetrixID, p.maskingViewID)
		})
		if err != nil {
			var pmxErr *pmxtypes.Error
			if errors.As(err, &pmxErr) && pmxErr.HTTPStatusCode == 404 {
				p.log.V(2).Info("masking view already deleted", "masking_view", p.maskingViewID)
			} else {
				// SG cannot be deleted while the MV is still attached; treat as blocking.
				p.log.Info("failed to delete masking view, skipping SG cleanup", "masking_view", p.maskingViewID, "err", err)
				p.maskingViewID = ""
				p.storageGroupID = ""
				return fmt.Errorf("failed to delete masking view: %w", err)
			}
		}
	}

	if p.storageGroupID != "" {
		p.log.V(2).Info("removing volume from storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID)
		err := retryOnTransient(ctx, p.log, "RemoveVolumesFromStorageGroup", func() error {
			_, e := p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
			return e
		})
		if err != nil {
			p.log.Info("failed removing volume from storage group", "volume", targetLUN.ProviderID, "storage_group", p.storageGroupID, "err", err)
			errs = append(errs, err)
		}

		p.log.V(2).Info("deleting storage group", "storage_group", p.storageGroupID)
		err = retryOnTransient(ctx, p.log, "DeleteStorageGroup", func() error {
			return p.client.DeleteStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		})
		if err != nil {
			p.log.Info("failed to delete storage group", "storage_group", p.storageGroupID, "err", err)
			errs = append(errs, err)
		}
	}

	p.maskingViewID = ""
	p.storageGroupID = ""
	p.log.Info("volume unmapped and cleaned up", "volume", targetLUN.ProviderID)
	return errors.Join(errs...)
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
		Jitter:   1.0,
		Steps:    7,
	}
	var lastErr error
	attempt := 0
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(_ context.Context) (bool, error) {
		attempt++
		lastErr = fn()
		if lastErr == nil {
			if attempt > 1 {
				log.Info("operation succeeded after retries", "operation", operation, "attempts", attempt)
			}
			return true, nil
		}
		var pmxErr *pmxtypes.Error
		if errors.As(lastErr, &pmxErr) {
			if pmxErr.HTTPStatusCode == 503 || pmxErr.HTTPStatusCode == 500 {
				log.Info("transient error, retrying", "operation", operation, "httpStatus", pmxErr.HTTPStatusCode, "attempt", attempt, "maxAttempts", backoff.Steps, "err", lastErr)
				return false, nil
			}
			if pmxErr.HTTPStatusCode == 409 {
				log.Info("409 conflict, treating as success", "operation", operation, "err", lastErr)
				return true, nil
			}
			log.Info("non-retryable PowerMax API error", "operation", operation, "httpStatus", pmxErr.HTTPStatusCode, "message", pmxErr.Message, "errorCode", pmxErr.ErrorCode)
		} else if strings.Contains(lastErr.Error(), "Service Unavailable") {
			// Some SDK methods (e.g. AddVolumesToStorageGroupS) wrap the original
			// *pmxtypes.Error with fmt.Errorf("%s", ...) which destroys the type.
			// Fall back to string matching for these cases.
			log.Info("transient Service Unavailable error, retrying", "operation", operation, "attempt", attempt, "maxAttempts", backoff.Steps, "err", lastErr)
			return false, nil
		} else {
			log.Info("non-retryable error", "operation", operation, "errType", fmt.Sprintf("%T", lastErr), "err", lastErr)
		}
		return false, lastErr
	})
	if wait.Interrupted(err) {
		log.Error(lastErr, "operation failed after retries", "operation", operation, "attempts", attempt)
	}
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

// extractWWPN extracts the WWPN (World Wide Port Name) from a WWNN:WWPN format string.
// If no colon is present, the input is returned as-is.
func extractWWPN(wwnnWwpn string) string {
	if idx := strings.LastIndex(wwnnWwpn, ":"); idx >= 0 {
		return wwnnWwpn[idx+1:]
	}
	return wwnnWwpn
}

// resolvePortGroupProtocol returns the protocol for a port group. On V4 arrays (2500/8500),
// the port_group_protocol field is returned directly. On V3 arrays (2000/8000), this field
// is absent so we fall back to the type field: "Fibre" maps to "SCSI_FC", "iSCSI" stays "iSCSI".
func resolvePortGroupProtocol(pg *pmxtypes.PortGroup, log klog.Logger) (string, error) {
	if pg.PortGroupProtocol != "" {
		return pg.PortGroupProtocol, nil
	}
	switch pg.PortGroupType {
	case "Fibre":
		log.Info("port_group_protocol not set (V3 array), resolved from type field", "type", pg.PortGroupType, "protocol", "SCSI_FC")
		return "SCSI_FC", nil
	case "iSCSI":
		log.Info("port_group_protocol not set (V3 array), resolved from type field", "type", pg.PortGroupType, "protocol", "iSCSI")
		return "iSCSI", nil
	default:
		return "", fmt.Errorf("unable to determine port group protocol: port_group_protocol is empty and type %q is not recognized (expected \"Fibre\" or \"iSCSI\")", pg.PortGroupType)
	}
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
			log.Info("unknown protocol, returning no initiators as a safety net", "protocol", protocol)
			return nil
		}
	}

	return filtered
}
