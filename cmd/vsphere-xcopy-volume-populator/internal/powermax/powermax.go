package powermax

//go:generate mockgen -destination=mock_powermax_client_test.go -package=powermax github.com/dell/gopowermax/v2 Pmax

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"strings"

	gopowermax "github.com/dell/gopowermax/v2"
	pmxtypes "github.com/dell/gopowermax/v2/types/v100"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
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
}

// CurrentMappedGroups implements populator.StorageApi.
func (p *PowermaxClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	ctx := context.TODO()
	volume, err := p.client.GetVolumeByID(ctx, p.symmetrixID, targetLUN.ProviderID)
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
		maskingViewList, err := p.client.GetMaskingViewList(ctx, p.symmetrixID)
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
			maskingView, err := p.client.GetMaskingViewByID(ctx, p.symmetrixID, mvID)
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
	_, err = p.client.GetStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
	if err == nil {
		klog.Infof("group %s exists", p.storageGroupID)
	}
	if e, ok := err.(*pmxtypes.Error); ok && e.HTTPStatusCode == 404 {
		klog.Infof("group %s doesn't exist - create it", p.storageGroupID)
		_, err := p.client.CreateStorageGroup(ctx, p.symmetrixID, p.storageGroupID, "none", "", true, nil)
		if err != nil {
			klog.Errorf("failed to create group %v ", err)
			return nil, err
		}
	}

	klog.Infof("storage group %s", p.storageGroupID)

	// Fetch port group to determine protocol type
	portGroup, err := p.client.GetPortGroupByID(ctx, p.symmetrixID, p.portGroup)
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

	hosts, err := p.client.GetHostList(ctx, p.symmetrixID)
h:
	for _, hostId := range hosts.HostIDs {
		host, err := p.client.GetHostByID(ctx, p.symmetrixID, hostId)
		if err != nil {
			return nil, err
		}
		klog.Infof("host ID %s and initiators %v", host.HostID, host.Initiators)
		for _, initiator := range host.Initiators {
			for _, filteredInit := range filteredInitiators {
				if strings.HasSuffix(filteredInit, initiator) {
					p.hostID = hostId
					break h
				}
			}
		}
	}
	if p.hostID != "" {
		klog.Infof("found host ID %s matching protocol %s", p.hostID, portGroup.PortGroupProtocol)
	} else {
		klog.Infof("cannot find host matching filtered initiators %v", filteredInitiators)
	}

	klog.Infof("port group ID %s", p.portGroup)
	mappingContext := map[string]any{}
	return mappingContext, err
}

// Map implements populator.StorageApi.
func (p *PowermaxClonner) Map(_ string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	klog.Infof("mapping volume %s to %s", targetLUN.ProviderID, p.storageGroupID)
	ctx := context.TODO()
	volumesMapped, err := p.client.GetVolumeIDListInStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
	if err != nil {
		return targetLUN, err
	}
	if slices.Contains(volumesMapped, targetLUN.ProviderID) {
		klog.Infof("volume %s already mapped to storage-group %s", targetLUN.ProviderID, p.storageGroupID)
		return targetLUN, nil
	}

	err = p.client.AddVolumesToStorageGroupS(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
	if err != nil {
		klog.Infof("failed mapping volume %s to %s: %v", targetLUN.ProviderID, p.storageGroupID, err)
		return targetLUN, err
	}

	mv, err := p.client.GetMaskingViewByID(ctx, p.symmetrixID, p.initiatorID)
	if err != nil {
		// probably not found, will be created later
		if e, ok := err.(*pmxtypes.Error); ok && e.HTTPStatusCode == 404 {
			klog.Infof("masking view not found %s ", e)
		} else {
			return populator.LUN{}, err
		}
	}

	if mv == nil {
		mv, err = p.client.CreateMaskingView(ctx, p.symmetrixID, p.initiatorID, p.storageGroupID, p.hostID, false, p.portGroup)
		if err != nil {
			return populator.LUN{}, err
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
	volume, err := p.client.GetVolumeByID(ctx, p.symmetrixID, volID)
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
		klog.Infof("deleting masking view %s", p.maskingViewID)
		err := p.client.DeleteMaskingView(ctx, p.symmetrixID, p.maskingViewID)
		if err != nil {
			return fmt.Errorf("failed to delete masking view: %w", err)
		}

		klog.Infof("removing volume ID %s from storage group %s", targetLUN.ProviderID, p.storageGroupID)
		_, err = p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
		if err != nil {
			return fmt.Errorf("failed removing volume from storage group:  %w", err)
		}

		klog.Infof("deleting storage group %s", p.storageGroupID)
		err = p.client.DeleteStorageGroup(ctx, p.symmetrixID, p.storageGroupID)
		if err != nil {
			return fmt.Errorf("failed to delete storage group: %w", err)
		}
		return nil
	}

	klog.Infof("removing volume ID %s from storage group %s", targetLUN.ProviderID, p.storageGroupID)

	_, err := p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, p.storageGroupID, false, targetLUN.ProviderID)
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
	return PowermaxClonner{client: client, symmetrixID: symID, portGroup: portGroup}, nil
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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
