package powermax

import (
	"context"
	"fmt"
	"os"
	"strings"

	pmax "github.com/dell/gopowermax/v2"
	pmaxtypes "github.com/dell/gopowermax/v2/types/v100"
	"github.com/google/uuid"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const portGroupIDKey = "portGroupID"
const storageGroupIDKey = "storageGroupID"
const hostIDKey = "hostID"
const maskingViewIDKey = "maskingViewID"
const uuidKey = "uuid"

type PowermaxClonner struct {
	client      pmax.Pmax
	symmetrixID string
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func findSubstringInList(list []string, sub string) (bool, int) {
	for i, s := range list {
		if strings.Contains(s, sub) {
			return true, i
		}
	}
	return false, -1
}

func shortUUID() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")[:6]
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

const (
	testSG = "xcopy-demo-SG"
	testIG = "xcopy-demo-IG"
	testPG = "xcopy-demo-PG"
	testMV = "xcopy-demo-MV"
)

func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

// EnsureClonerIgroup implements populator.StorageApi.
func (p *PowermaxClonner) EnsureClonnerIgroupWithHost(initiatorGroup string, initiators []string, esxiHostName string) (populator.MappingContext, error) {
	ctx := context.TODO()

	// remoe "." from the string  for both IP and fqdn
	esxiHostName = strings.ReplaceAll(esxiHostName, ".", "")

	prefix := "mtv-xcopy-" + esxiHostName // base prefix
	shortuuid := shortUUID()

	klog.Infof("Prefix for temporary PowerMax objects used in migration:%s ", prefix)

	// Generate names for host group, storage group, masking view
	sgname := "sg-" + prefix + "-" + shortuuid
	mvname := "mv-" + prefix + "-" + shortuuid

	hgprefix := "hg-" + prefix
	hgname := hgprefix + "-" + shortuuid

	pgprefix := "pg-" + prefix
	pgname := pgprefix + "-" + shortuuid

	klog.Infof("StorageGroup name: %s", sgname)
	klog.Infof("HostGroup name: %s", hgname)
	klog.Infof("PortGroup name: %s", pgname)
	klog.Infof("MaskingView name: %s", mvname)

	var wwns []string
	for _, fcaddress := range initiators {
		parts := strings.Split(fcaddress, ":")
		if len(parts) != 2 {
			continue
		}
		wwns = append(wwns, parts[1])
	}

	// Lookup all hosts and find matching prefix
	hostList, err := p.client.GetHostList(ctx, p.symmetrixID)
	if err != nil {
		return nil, fmt.Errorf("failed to get host list: %w", err)
	}
	var matchedHostID string
	for _, hostID := range hostList.HostIDs {
		if strings.HasPrefix(hostID, hgprefix) {
			matchedHostID = hostID
			break
		}
	}

	// Check for missing initiators, Add initiators or create new Host
	if matchedHostID != "" {
		klog.Infof("Host %s found", matchedHostID)
		klog.Infof("Check if all the initiators are present in host %s", matchedHostID)

		host, err := p.client.GetHostByID(ctx, p.symmetrixID, matchedHostID)
		if err != nil {
			return nil, fmt.Errorf("Failed to get host by ID %s: %w", matchedHostID, err)
		}

		existingInitiators := host.Initiators
		missingInitiators := difference(wwns, existingInitiators)
		if len(missingInitiators) > 0 {
			klog.Infof("Initiators missing in host %s: %v", matchedHostID, missingInitiators)
			_, err = p.client.UpdateHostInitiators(ctx, p.symmetrixID, host, missingInitiators)
			if err != nil {
				return nil, fmt.Errorf("Failed to update host initiators: %w", err)
			}
		}
	} else {
		// Check if the initiators already present in a host
		klog.Infof("Check for existing HostGroup : %v", wwns)
		var existingHostID string
		for _, hostID := range hostList.HostIDs {
			klog.Infof("Check Host %s: ", hostID)
			host, err := p.client.GetHostByID(ctx, p.symmetrixID, hostID)
			if err != nil {
				continue
			}

			for _, init := range host.Initiators {
				if contains(wwns, init) {
					existingHostID = hostID
					klog.Infof("Host %s has the initiator %s", hostID, init)
					break
				}
			}
		}
		if existingHostID != "" {
			klog.Infof("Use existing host %s:", existingHostID)
			hgname = existingHostID
		} else {

			// Create new host with all initiators
			klog.Infof("Create new host with all initiators : %v", wwns)
			hostFlags := &pmaxtypes.HostFlags{
				VolumeSetAddressing: &pmaxtypes.HostFlag{},
				DisableQResetOnUA:   &pmaxtypes.HostFlag{},
				EnvironSet:          &pmaxtypes.HostFlag{},
				AvoidResetBroadcast: &pmaxtypes.HostFlag{},
				OpenVMS:             &pmaxtypes.HostFlag{},
				SCSI3:               &pmaxtypes.HostFlag{},
				Spc2ProtocolVersion: &pmaxtypes.HostFlag{
					Enabled:  true,
					Override: true,
				},
				SCSISupport1:  &pmaxtypes.HostFlag{},
				ConsistentLUN: false,
			}
			_, err := p.client.CreateHost(ctx, p.symmetrixID, hgname, wwns, hostFlags)
			if err != nil {
				return nil, fmt.Errorf("Failed to create new host %s: %w", hgname, err)
			}
		}
		matchedHostID = hgname
	}

	// Lookup all hosts and find matching prefix
	pgList, err := p.client.GetPortGroupList(ctx, p.symmetrixID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get host list: %w", err)
	}

	var matchedPGID string
	for _, pgID := range pgList.PortGroupIDs {
		if strings.HasPrefix(pgID, pgprefix) {
			matchedPGID = pgID
			break
		}
	}

	if matchedPGID != "" {
		klog.Infof("PortGroup %s found", matchedPGID)
		pgname = matchedPGID
	} else {
		initiatorList, err := p.client.GetInitiatorList(ctx, p.symmetrixID, "", false, true)
		if err != nil {
			return nil, fmt.Errorf("failed to get initiator list: %w", err)
		}

		klog.Infof("Initiator list %v: ", initiatorList.InitiatorIDs)

		// Create PortGroup based on logged-in initiators' ports
		var loggedInPorts []pmaxtypes.PortKey
		for _, initiator := range wwns {
			found, index := findSubstringInList(initiatorList.InitiatorIDs, initiator)
			if found {
				pmax_initiator, err := p.client.GetInitiatorByID(ctx, p.symmetrixID, initiatorList.InitiatorIDs[index])
				if err != nil {
					// return nil, fmt.Errorf("Failed to get initiator by ID %s: %w", initiator, err)
					continue
				}
				if pmax_initiator.LoggedIn {
					for _, portKey := range pmax_initiator.SymmetrixPortKey {
						klog.Infof("Logged in initiator found %s: ", pmax_initiator.InitiatorID)
						loggedInPorts = append(loggedInPorts, pmaxtypes.PortKey(portKey))
					}
				}
			}
		}

		if len(loggedInPorts) == 0 {
			return nil, fmt.Errorf("No logged-in initiators found")
		}
		klog.Infof("Found %d logged-in initiators", len(loggedInPorts))

		_, err = p.client.CreatePortGroup(ctx, p.symmetrixID, pgname, loggedInPorts, "SCSI_FC")
		if err != nil {
			return nil, fmt.Errorf("failed to create port group %s: %w", pgname, err)
		}
	}

	// Create Sorage Group
	_, err = p.client.CreateStorageGroup(ctx, p.symmetrixID, sgname, "none", "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to creeate storage group %s: %w", sgname, err)
	}

	mappingContext := map[string]any{portGroupIDKey: pgname, storageGroupIDKey: sgname, hostIDKey: hgname, maskingViewIDKey: mvname, uuidKey: shortuuid}
	return mappingContext, nil
}

func (p *PowermaxClonner) EnsureClonnerIgroup(initiatorGroup string, initiators []string) (populator.MappingContext, error) {
	return nil, fmt.Errorf("Failed to get mapping context")
}

func (p *PowermaxClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	ctx := context.TODO()
	portGroupID, ok := mappingContext[portGroupIDKey]
	if !ok {
		return populator.LUN{}, fmt.Errorf("there is no port group in the mapping context, can't continue with mapping")
	}

	storageGroupID, ok := mappingContext[storageGroupIDKey]
	if !ok {
		return populator.LUN{}, fmt.Errorf("there is no storage group in the mapping context, can't continue with mapping")
	}

	hostID, ok := mappingContext[hostIDKey]
	if !ok {
		return populator.LUN{}, fmt.Errorf("there is no host in the mapping context, can't continue with mapping")
	}

	maskingViewID, ok := mappingContext[maskingViewIDKey]
	if !ok {
		return populator.LUN{}, fmt.Errorf("there is no masking view in the mapping context, can't continue with mapping")
	}
	klog.Infof("Masking view ID %s", maskingViewID.(string))
	klog.Infof("storage group ID %s", storageGroupID.(string))
	klog.Infof("port group ID %s", portGroupID.(string))
	klog.Infof("host ID %s", hostID.(string))

	// Add volume to the pre-created SG
	err := p.client.AddVolumesToStorageGroupS(ctx, p.symmetrixID, storageGroupID.(string), false, targetLUN.ProviderID)
	if err != nil {
		return targetLUN, fmt.Errorf("failed mapping volume %s to SG %s: %w", targetLUN.ProviderID, storageGroupID, err)
	}

	// Create Masking View
	_, err = p.client.CreateMaskingView(ctx, p.symmetrixID, maskingViewID.(string), storageGroupID.(string), hostID.(string), true, portGroupID.(string))
	if err != nil {
		return targetLUN, fmt.Errorf("Failed to create masking view %s: %w", maskingViewID.(string), err)
	}

	klog.Infof("Mapped volume %s to SG %s", targetLUN.ProviderID, storageGroupID)
	return targetLUN, nil
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

func (p *PowermaxClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	ctx := context.TODO()
	portGroupID, ok := mappingContext[portGroupIDKey]
	if !ok {
		return fmt.Errorf("there is no port group in the mapping context, can't continue with mapping")
	}

	storageGroupID, ok := mappingContext[storageGroupIDKey]
	if !ok {
		return fmt.Errorf("there is no storage group in the mapping context, can't continue with mapping")
	}

	hostID, ok := mappingContext[hostIDKey]
	if !ok {
		return fmt.Errorf("there is no host in the mapping context, can't continue with mapping")
	}

	uuid, ok := mappingContext[uuidKey]
	if !ok {
		return fmt.Errorf("there is no uuid in the mapping context, can't continue with mapping")
	}

	maskingViewID, ok := mappingContext[maskingViewIDKey]
	if !ok {
		return fmt.Errorf("there is no masking view in the mapping context, can't continue with mapping")
	}
	klog.Infof("Masking view ID %s", maskingViewID.(string))
	klog.Infof("storage group ID %s", storageGroupID.(string))
	klog.Infof("port group ID %s", portGroupID.(string))
	klog.Infof("host ID %s", hostID.(string))

	err := p.client.DeleteMaskingView(ctx, p.symmetrixID, maskingViewID.(string))
	klog.Infof("Removing volume %s from Storage Group %s", targetLUN.ProviderID, storageGroupID.(string))

	// Remove the volume from the pre-created SG
	_, err = p.client.RemoveVolumesFromStorageGroup(ctx, p.symmetrixID, storageGroupID.(string), false, targetLUN.ProviderID)
	if err != nil {
		return fmt.Errorf("failed removing volume %s from SG %s: %w", targetLUN.ProviderID, storageGroupID.(string), err)
	}

	klog.Infof("Successfully removed volume %s from Storage Group %s", targetLUN.ProviderID, storageGroupID.(string))
	err = p.client.DeleteStorageGroup(ctx, p.symmetrixID, storageGroupID.(string))
	if strings.Contains(portGroupID.(string), uuid.(string)) {
		err = p.client.DeletePortGroup(ctx, p.symmetrixID, portGroupID.(string))
	}
	return nil
}

func NewPowermaxClonner(hostname, username, password string, sslSkipVerify bool) (PowermaxClonner, error) {
	// using the same application name as the driver
	applicationName := "csi"
	client, err := pmax.NewClientWithArgs(
		hostname,
		applicationName,
		sslSkipVerify,
		false,
		"")

	if err != nil {
		return PowermaxClonner{}, err
	}

	c := pmax.ConfigConnect{
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

	symID := os.Getenv("POWERMAX_SYMMETRIX_ID")
	return PowermaxClonner{client: client, symmetrixID: symID}, nil
}
