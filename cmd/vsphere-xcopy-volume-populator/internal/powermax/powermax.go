package powermax

//go:generate mockgen -destination=mock_powermax_client_test.go -package=powermax github.com/dell/gopowermax/v2 Pmax

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	gopowermax "github.com/dell/gopowermax/v2"
	pmxtypes "github.com/dell/gopowermax/v2/types/v100"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const portGroupIDKey = "portGroupID"

type PowermaxClonner struct {
	client      gopowermax.Pmax
	symmetrixID string
	portGroupID string
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
func (p *PowermaxClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (populator.MappingContext, error) {
	ctx := context.TODO()

	// steps:
	// 1.create the storage group
	// 2. create a masking view, add the storage group to it - name it with the same name
	// 3. create InitiatorGroup on the masking view
	// 4. add clonnerIqn to that initiar group
	// 5. add port group with protocol type that match the cloner IQN type, only if they all online
	klog.Infof("ensuring storage group %s exists with hosts %v", initiatorGroup, clonnerIqn)
	sg, err := p.client.GetStorageGroup(ctx, p.symmetrixID, initiatorGroup)
	if err == nil {
		klog.Infof("group %s exists", initiatorGroup)
	}
	if e, ok := err.(*pmxtypes.Error); ok && e.HTTPStatusCode == 404 {
		klog.Infof("group %s doesn't exist - create it", initiatorGroup)
		_, err := p.client.CreateStorageGroup(ctx, p.symmetrixID, initiatorGroup, "none", "", true, nil)
		if err != nil {
			klog.Errorf("failed to create group %v ", err)
			return nil, err
		}
	}

	klog.Infof("storage group %s", sg.BaseSLOName)

	hostIdsToAdd := []string{}
	hosts, err := p.client.GetHostList(ctx, p.symmetrixID)
	for _, hostId := range hosts.HostIDs {
		host, err := p.client.GetHostByID(ctx, p.symmetrixID, hostId)
		if err != nil {
			return nil, err
		}
		for _, initiator := range host.Initiators {
			for _, iqn := range clonnerIqn {
				if strings.HasSuffix(iqn, initiator) {
					if !slices.Contains(hostIdsToAdd, host.HostID) {
						hostIdsToAdd = append(hostIdsToAdd, host.HostID)
					}
				}
			}
		}
	}

	hg, err := p.client.GetHostGroupByID(ctx, p.symmetrixID, initiatorGroup)
	if err != nil {
		if e, ok := err.(*pmxtypes.Error); ok && e.HTTPStatusCode == 404 {
			hg, err = p.client.CreateHostGroup(ctx, p.symmetrixID, initiatorGroup, hostIdsToAdd, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create new hostGroup with id %s: %w", initiatorGroup, err)
			}
		} else {
			return nil, err
		}
	}

	for _, host := range hg.Hosts {
		// add the host to the group if not there
		klog.Infof("host %s initiators %s", host.HostID, host.Initiators)
		for _, iqn := range clonnerIqn {
			if slices.Contains(host.Initiators, iqn) {
				klog.Infof("adding host %s to host group", host.HostID)
				_, err := p.client.UpdateHostGroupHosts(ctx, p.symmetrixID, initiatorGroup, []string{host.HostID})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	klog.Infof("port group ID %s", p.portGroupID)
	mappingContext := map[string]any{portGroupIDKey: p.portGroupID}
	return mappingContext, err
}

// Map implements populator.StorageApi.
func (p *PowermaxClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	klog.Infof("mapping volume %s to %s", targetLUN.ProviderID, initiatorGroup)
	ctx := context.TODO()
	volumesMapped, err := p.client.GetVolumeIDListInStorageGroup(ctx, p.symmetrixID, initiatorGroup)
	if err != nil {
		return targetLUN, err
	}
	if slices.Contains(volumesMapped, targetLUN.ProviderID) {
		klog.Infof("volume %s already mapped to storage-group %s", targetLUN.ProviderID, initiatorGroup)
		return targetLUN, nil
	}

	err = p.client.AddVolumesToStorageGroupS(ctx, p.symmetrixID, initiatorGroup, false, targetLUN.ProviderID)
	if err != nil {

		klog.Infof("failed mapping volume %s to %s: %v", targetLUN.ProviderID, initiatorGroup, err)
		return targetLUN, err
	}

	mv, err := p.client.GetMaskingViewByID(ctx, p.symmetrixID, initiatorGroup)
	if err != nil {
		// probably not found, will be created later
		if e, ok := err.(*pmxtypes.Error); ok && e.HTTPStatusCode == 404 {
			klog.Infof("masking view not found %s ", e)
		} else {
			return populator.LUN{}, err
		}
	}

	portGroupID, ok := mappingContext[portGroupIDKey]
	if !ok {
		return populator.LUN{}, fmt.Errorf("there is no port group in the mappning context, can't continue with mapping")
	}

	if mv == nil {
		mv, err = p.client.CreateMaskingView(ctx, p.symmetrixID, initiatorGroup, initiatorGroup, initiatorGroup, false, portGroupID.(string))
		if err != nil {
			return populator.LUN{}, err
		}
	}

	klog.Infof("successfully mapped volume %s to %s with masking view %s", targetLUN.ProviderID, initiatorGroup, mv.MaskingViewID)
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
func (p *PowermaxClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	ctx := context.TODO()
	klog.Infof("removing volume ID %s from storage group %s", targetLUN.ProviderID, initiatorGroup)

	_, err := p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, initiatorGroup, false, targetLUN.ProviderID)
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
	portGroupID := os.Getenv("POWERMAX_PORT_GROUP_ID")
	if portGroupID == "" {
		return PowermaxClonner{}, fmt.Errorf("Please set POWERMAX_PORT_GROUP_ID in the pod environment or in the secret" +
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
	return PowermaxClonner{client: client, symmetrixID: symID, portGroupID: portGroupID}, nil
}
