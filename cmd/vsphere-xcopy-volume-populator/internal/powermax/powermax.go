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
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
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
	ctx := context.TODO()
	var volume *pmxtypes.Volume
	err := retryOnTransient(ctx, "GetVolumeByID", func() error {
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

	klog.Infof("Volume %s is in Storage Group(s): %v\n", targetLUN.ProviderID, volume.StorageGroups)

	foundHostGroups := []string{}

	for _, sgID := range volume.StorageGroups {
		foundHostGroups = append(foundHostGroups, sgID.StorageGroupName)
		var maskingViewList *pmxtypes.MaskingViewList
		err := retryOnTransient(ctx, "GetMaskingViewList", func() error {
			var e error
			maskingViewList, e = p.client.GetMaskingViewList(ctx, p.symmetrixID)
			return e
		})
		if err != nil {
			klog.Infof("Error getting masking views for Storage Group %s: %v", sgID, err)
			continue
		}

		if len(maskingViewList.MaskingViewIDs) == 0 {
			klog.Infof("No masking views found for Storage Group %s.\n", sgID)
			continue
		}

		// Step 3: Get details of each Masking View to find the Host Group
		for _, mvID := range maskingViewList.MaskingViewIDs {
			var maskingView *pmxtypes.MaskingView
			err := retryOnTransient(ctx, "GetMaskingViewByID", func() error {
				var e error
				maskingView, e = p.client.GetMaskingViewByID(ctx, p.symmetrixID, mvID)
				return e
			})
			if err != nil {
				klog.Errorf("Error getting masking view %s: %v", mvID, err)
				continue
			}

			if maskingView.HostID != "" {
				// This masking view is directly mapped to a Host, not a Host Group
				klog.Infof("Volume %s is mapped via Masking View %s to Host: %s\n", targetLUN.ProviderID, mvID, maskingView.HostID)
				foundHostGroups = append(foundHostGroups, maskingView.HostID)
			} else if maskingView.HostGroupID != "" {
				// This masking view is mapped to a Host Group
				klog.Infof("Volume %s is mapped via Masking View %s to Host Group: %s\n", targetLUN.ProviderID, mvID, maskingView.HostGroupID)
				foundHostGroups = append(foundHostGroups, maskingView.HostGroupID)
			}
		}
	}

	if len(foundHostGroups) > 0 {
		klog.Info("Unique Host Groups found for the volume:")
		for _, hg := range foundHostGroups {
			klog.Infof("- %s", hg)
		}
	} else {
		klog.Info("No host groups found for the volume.")
	}
	return foundHostGroups, nil
}

// EnsureClonnerIgroup implements populator.StorageApi.
func (p *PowermaxClonner) EnsureClonnerIgroup(_ string, clonnerIqn []string) (populator.MappingContext, error) {
	ctx := context.TODO()

	randomString, err := generateRandomString(4)
	if err != nil {
		return nil, err
	}
	p.initiatorID = fmt.Sprintf("xcopy-%s", randomString)
	klog.Infof("Generated unique initiator group name: %s", p.initiatorID)

	// steps:
	// 1.create the storage group
	// 2. create a masking view, add the storage group to it - name it with the same name
	// 3. create InitiatorGroup on the masking view
	// 4. add clonnerIqn to that initiar group
	// 5. add port group with protocol type that match the cloner IQN type, only if they all online
	p.storageGroupID = fmt.Sprintf("%s-SG", p.initiatorID)
	klog.Infof("ensuring storage group %s exists with hosts %v", p.storageGroupID, clonnerIqn)
	err = retryOnTransient(ctx, "GetStorageGroup", func() error {
		_, e := p.client.GetStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		return e
	})
	if err == nil {
		klog.Infof("group %s exists", p.storageGroupID)
	} else {
		var pmxErr *pmxtypes.Error
		if errors.As(err, &pmxErr) && pmxErr.HTTPStatusCode == 404 {
			klog.Infof("group %s doesn't exist - create it", p.storageGroupID)
			err = retryOnTransient(ctx, "CreateStorageGroup", func() error {
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

	klog.Infof("storage group %s", p.storageGroupID)

	// Fetch port group to determine protocol type
	var portGroup *pmxtypes.PortGroup
	err = retryOnTransient(ctx, "GetPortGroupByID", func() error {
		var e error
		portGroup, e = p.client.GetPortGroupByID(ctx, p.symmetrixID, p.portGroup)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get port group %s: %w", p.portGroup, err)
	}
	klog.Infof("port group %s has protocol: %s", p.portGroup, portGroup.PortGroupProtocol)

	// Filter initiators based on port group protocol
	filteredInitiators := filterInitiatorsByProtocol(clonnerIqn, portGroup.PortGroupProtocol)
	if len(filteredInitiators) == 0 {
		return nil, fmt.Errorf("no initiators matching protocol %s found in %v", portGroup.PortGroupProtocol, clonnerIqn)
	}
	klog.Infof("filtered initiators for protocol %s: %v", portGroup.PortGroupProtocol, filteredInitiators)

	// Direct initiator lookup (1 API call per initiator instead of N+1)
	for _, filteredInit := range filteredInitiators {
		lookupID := initiatorToLookupID(filteredInit)
		var initiator *pmxtypes.Initiator

		if portGroup.PortGroupProtocol == "SCSI_FC" {
			// FC initiators from ESXi are in WWNN:WWPN format, but the PowerMax API
			// expects initiator IDs in <director>:<port>:<wwn> format (e.g., OR-2C:0:10000000c99debc3).
			// Use GetInitiatorList with the WWPN to find the correct PowerMax initiator ID.
			wwpn := extractWWPN(lookupID)
			var initList *pmxtypes.InitiatorList
			err = retryOnTransient(ctx, "GetInitiatorList", func() error {
				var e error
				initList, e = p.client.GetInitiatorList(ctx, p.symmetrixID, wwpn, false, true)
				return e
			})
			if err != nil {
				return nil, fmt.Errorf("failed to list initiators for WWPN %s: %w", wwpn, err)
			}
			if len(initList.InitiatorIDs) == 0 {
				klog.V(2).Infof("no initiators found for WWPN %s", wwpn)
				continue
			}
			pmxInitID := initList.InitiatorIDs[0]
			klog.V(2).Infof("resolved FC initiator WWPN %s to PowerMax ID %s", wwpn, pmxInitID)
			err = retryOnTransient(ctx, "GetInitiatorByID", func() error {
				var e error
				initiator, e = p.client.GetInitiatorByID(ctx, p.symmetrixID, pmxInitID)
				return e
			})
			if err != nil {
				return nil, fmt.Errorf("failed to get initiator %s: %w", pmxInitID, err)
			}
		} else {
			// For iSCSI, the IQN is the initiator ID — direct lookup works
			err = retryOnTransient(ctx, "GetInitiatorByID", func() error {
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
			klog.Infof("found matching host %s for initiator %s", p.hostID, lookupID)
			break
		}
	}
	if p.hostID == "" {
		return nil, fmt.Errorf("can't find a host on symmetrix %s with initiators matching %v. "+
			"Ensure the ESXi host has a corresponding host object in PowerMax with the correct FC/iSCSI initiators registered",
			p.symmetrixID, filteredInitiators)
	}
	klog.Infof("found host ID %s matching protocol %s", p.hostID, portGroup.PortGroupProtocol)

	klog.Infof("port group ID %s", p.portGroup)
	mappingContext := map[string]any{}
	return mappingContext, err
}

// Map implements populator.StorageApi.
// On PowerMax, the volume is never removed from its original storage groups during xcopy,
// so re-mapping after cleanup is a no-op. Only the initial xcopy mapping (via MapTarget)
// needs to add the volume to the temporary xcopy storage group.
func (p *PowermaxClonner) Map(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	// After full cleanup the xcopy SG has been deleted; skip re-mapping since
	// the volume is still in its original storage groups.
	if v, ok := mappingContext[populator.CleanupXcopyInitiatorGroup]; ok && v.(bool) {
		klog.V(2).Infof("skipping Map after cleanup, volume %s remains in original storage groups", targetLUN.ProviderID)
		return targetLUN, nil
	}

	klog.Infof("mapping volume %s to storage group %s", targetLUN.ProviderID, p.storageGroupID)

	ctx := context.TODO()
	var volumesMapped []string
	err := retryOnTransient(ctx, "GetVolumeIDListInStorageGroup", func() error {
		var e error
		volumesMapped, e = p.client.GetVolumeIDListInStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		return e
	})
	if err != nil {
		return targetLUN, err
	}
	if slices.Contains(volumesMapped, targetLUN.ProviderID) {
		klog.Infof("volume %s already mapped to storage-group %s", targetLUN.ProviderID, p.storageGroupID)
		return targetLUN, nil
	}

	klog.V(2).Infof("adding volume %s to storage group %s", targetLUN.ProviderID, p.storageGroupID)
	err = retryOnTransient(ctx, "AddVolumesToStorageGroupS", func() error {
		return p.client.AddVolumesToStorageGroupS(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
	})
	if err != nil {
		klog.Infof("failed mapping volume %s to %s: %v", targetLUN.ProviderID, p.storageGroupID, err)
		return targetLUN, err
	}

	var mv *pmxtypes.MaskingView
	err = retryOnTransient(ctx, "GetMaskingViewByID", func() error {
		var e error
		mv, e = p.client.GetMaskingViewByID(ctx, p.symmetrixID, p.initiatorID)
		return e
	})
	if err != nil {
		// probably not found, will be created later
		var pmxErr *pmxtypes.Error
		if errors.As(err, &pmxErr) && pmxErr.HTTPStatusCode == 404 {
			klog.V(2).Infof("masking view %s not found, will be created", p.initiatorID)
		} else {
			return populator.LUN{}, err
		}
	}

	if mv == nil {
		klog.V(2).Infof("creating masking view %s with storage group %s, host %s, port group %s", p.initiatorID, p.storageGroupID, p.hostID, p.portGroup)
		err = retryOnTransient(ctx, "CreateMaskingView", func() error {
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
			err = retryOnTransient(ctx, "GetMaskingViewByID", func() error {
				var e error
				mv, e = p.client.GetMaskingViewByID(ctx, p.symmetrixID, p.initiatorID)
				return e
			})
			if err != nil {
				return populator.LUN{}, fmt.Errorf("masking view %s exists (409) but failed to fetch it: %w", p.initiatorID, err)
			}
		}
	}

	klog.Infof("successfully mapped volume %s to %s with masking view %s", targetLUN.ProviderID, p.initiatorID, mv.MaskingViewID)
	p.maskingViewID = mv.MaskingViewID
	return targetLUN, err
}

// ResolvePVToLUN implements populator.StorageApi.
func (p *PowermaxClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	ctx := context.TODO()
	volID := pv.VolumeHandle[strings.LastIndex(pv.VolumeHandle, "-")+1:]
	klog.V(2).Infof("extracting volume ID %s from handle", volID)

	var volume *pmxtypes.Volume
	err := retryOnTransient(ctx, "GetVolumeByID", func() error {
		var e error
		volume, e = p.client.GetVolumeByID(ctx, p.symmetrixID, volID)
		return e
	})
	if err != nil || volume.VolumeID == "" {
		return populator.LUN{}, fmt.Errorf("failed getting details for volume %v: %v", volume, err)
	}
	naa := fmt.Sprintf("naa.%s", volume.WWN)
	return populator.LUN{Name: volume.VolumeIdentifier, ProviderID: volume.VolumeID, NAA: naa}, nil

}

// UnMap implements populator.StorageApi.
func (p *PowermaxClonner) UnMap(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	ctx := context.TODO()

	cleanup, ok := mappingContext[populator.CleanupXcopyInitiatorGroup]
	if ok && cleanup.(bool) {
		klog.V(2).Infof("full cleanup requested, deleting masking view %s", p.maskingViewID)
		err := retryOnTransient(ctx, "DeleteMaskingView", func() error {
			return p.client.DeleteMaskingView(ctx, p.symmetrixID, p.maskingViewID)
		})
		if err != nil {
			return fmt.Errorf("failed to delete masking view: %w", err)
		}

		klog.V(2).Infof("removing volume %s from storage group %s", targetLUN.ProviderID, p.storageGroupID)
		err = retryOnTransient(ctx, "RemoveVolumesFromStorageGroup", func() error {
			_, e := p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
			return e
		})
		if err != nil {
			return fmt.Errorf("failed removing volume from storage group:  %w", err)
		}

		klog.V(2).Infof("deleting storage group %s", p.storageGroupID)
		err = retryOnTransient(ctx, "DeleteStorageGroup", func() error {
			return p.client.DeleteStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		})
		if err != nil {
			return fmt.Errorf("failed to delete storage group: %w", err)
		}
		return nil
	}

	klog.Infof("removing volume ID %s from storage group %s", targetLUN.ProviderID, p.storageGroupID)

	err := retryOnTransient(ctx, "RemoveVolumesFromStorageGroup", func() error {
		_, e := p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
		return e
	})
	if err != nil {
		return fmt.Errorf("failed removing volume from storage group:  %w", err)
	}
	return nil
}

var newClientWithArgs = gopowermax.NewClientWithArgs

func NewPowermaxClonner(hostname, username, password string, sslSkipVerify bool) (PowermaxClonner, error) {
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
	klog.Info("successfuly logged in to PowerMax")

	clonner := PowermaxClonner{
		client:      client,
		symmetrixID: symID,
		portGroup:   portGroup,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "Dell",
			Product: "PowerMax",
		},
	}

	// Fetch model and version from the API
	sym, err := client.GetSymmetrixByID(context.TODO(), symID)
	if err != nil {
		klog.Warningf("Failed to get PowerMax symmetrix info for metrics: %v", err)
	} else {
		clonner.arrayInfo.Model = sym.Model
		clonner.arrayInfo.Version = sym.Ucode
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
func retryOnTransient(ctx context.Context, operation string, fn func() error) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   1.0,
		Steps:    5,
	}
	var lastErr error
	attempt := 0
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(_ context.Context) (bool, error) {
		attempt++
		lastErr = fn()
		if lastErr == nil {
			if attempt > 1 {
				klog.Infof("%s succeeded after %d attempts", operation, attempt)
			}
			return true, nil
		}
		var pmxErr *pmxtypes.Error
		if errors.As(lastErr, &pmxErr) {
			if pmxErr.HTTPStatusCode == 503 {
				klog.Infof("transient 503 error during %s (attempt %d/%d), retrying: %v", operation, attempt, backoff.Steps, lastErr)
				return false, nil
			}
			if pmxErr.HTTPStatusCode == 409 {
				klog.Infof("409 conflict during %s, treating as success (operation likely completed on a prior attempt): %v", operation, lastErr)
				return true, nil
			}
			klog.Warningf("non-retryable PowerMax API error during %s: HTTP %d, message=%q, errorCode=%d, type=%T",
				operation, pmxErr.HTTPStatusCode, pmxErr.Message, pmxErr.ErrorCode, lastErr)
		} else if strings.Contains(lastErr.Error(), "Service Unavailable") {
			// Some SDK methods (e.g. AddVolumesToStorageGroupS) wrap the original
			// *pmxtypes.Error with fmt.Errorf("%s", ...) which destroys the type.
			// Fall back to string matching for these cases.
			klog.Infof("transient Service Unavailable error during %s (attempt %d/%d), retrying: %v (error type: %T)",
				operation, attempt, backoff.Steps, lastErr, lastErr)
			return false, nil
		} else {
			klog.Warningf("non-retryable error during %s (not a PowerMax API error): type=%T, error=%v",
				operation, lastErr, lastErr)
		}
		return false, lastErr
	})
	if wait.Interrupted(err) {
		klog.Errorf("%s failed after %d attempts, last error: %v", operation, attempt, lastErr)
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

// filterInitiatorsByProtocol filters the initiator list based on the port group protocol
// iSCSI protocol requires IQN format initiators (e.g., "iqn.1994-05.com.redhat:...")
// SCSI_FC protocol requires FC WWN format initiators (e.g., "10000000c9a12345:10000000c9a12346")
func filterInitiatorsByProtocol(initiators []string, protocol string) []string {
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
			klog.Warningf("Unknown protocol %s, skipping initiator filtering", protocol)
			// For unknown protocols, return all initiators
			return initiators
		}
	}

	return filtered
}
