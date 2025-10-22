package powermax

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	gopowermax "github.com/dell/gopowermax/v2"
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
	client      gopowermax.Pmax
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

// implements populator.PowerMaxApi
func (p *PowermaxClonner) EnsureClonnerIgroupForHost(initiatorGroup string, initiators []string, esxiHostName string) (populator.MappingContext, error) {
	ctx := context.TODO()
	// remove "." from the string  for both IP and fqdn
	esxiHostName = strings.ReplaceAll(esxiHostName, ".", "")

	prefix := "mtv-xcopy-" + esxiHostName // base prefix
	shortuuid := shortUUID()

	klog.Infof("Prefix for temporary PowerMax objects used in this migration:%s ", prefix)

	//STEP 1 - Generate unique names for host group, port group, storage group, masking view
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

	//STEP 2 - If there is an existing host group use it, else create a new one
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
	if matchedHostID == "" {
		// Check if the initiators already present in a host
		klog.Infof("Check for existing HostGroup : %v", wwns)
		var existingHostID string
		for _, hostID := range hostList.HostIDs {
			//klog.Infof("Check Host %s: ", hostID)
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

	// STEP - 3 If default port group - POWERMAX_PORT_GROUP_ID - is empty then
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

	// STEP 4 - Create new storage group for each migration
	_, err = p.client.CreateStorageGroup(ctx, p.symmetrixID, sgname, "none", "", true, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to creeate storage group %s: %w", sgname, err)
	}

	// STEP 5 - save mapping context - uuid necessary to cleanup temp powermax objects
	mappingContext := map[string]any{portGroupIDKey: pgname, storageGroupIDKey: sgname, hostIDKey: hgname, maskingViewIDKey: mvname, uuidKey: shortuuid}

	return mappingContext, nil

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
	if e, ok := err.(*pmaxtypes.Error); ok && e.HTTPStatusCode == 404 {
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
		if e, ok := err.(*pmaxtypes.Error); ok && e.HTTPStatusCode == 404 {
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

	fibre := ""
	iscsi := ""
	for _, iqn := range clonnerIqn {
		if strings.HasPrefix(iqn, "iqn") {
			iscsi = "iscsi"
		} else if strings.HasPrefix(iqn, "wwn") {
			fibre = "fibre"
		}
	}

	pgIds := []string{}
	if fibre != "" {
		pgs, err := p.client.GetPortGroupList(ctx, p.symmetrixID, fibre)
		if err != nil {
			return nil, err
		}
		pgIds = append(pgIds, pgs.PortGroupIDs...)
	}
	if iscsi != "" {
		pgs, err := p.client.GetPortGroupList(ctx, p.symmetrixID, iscsi)
		if err != nil {
			return nil, err
		}
		pgIds = append(pgIds, pgs.PortGroupIDs...)
	}
	klog.Infof("port group IDs %s", pgIds)

	portGroupID := ""
	for _, pgId := range pgIds {
		pg, err := p.client.GetPortGroupByID(ctx, p.symmetrixID, pgId)
		if err != nil {
			return nil, err
		}
		allPortsOnline := false
		for _, portKey := range pg.SymmetrixPortKey {
			port, err := p.client.GetPort(ctx, p.symmetrixID, portKey.DirectorID, portKey.PortID)
			if err != nil {
				return nil, err
			}
			if port.SymmetrixPort.PortStatus == "Online" {
				allPortsOnline = true
			} else {
				allPortsOnline = false
				break
			}
		}
		if allPortsOnline {
			portGroupID = pg.PortGroupID
			break
		}
	}

	klog.Infof("port group ID %s", portGroupID)
	mappingContext := map[string]any{portGroupIDKey: portGroupID}
	return mappingContext, err
}

// Map implements populator.StorageApi.
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

	// Add volume to the SG
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

// UnMap implements populator.StorageApi.
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
	symID := os.Getenv("POWERMAX_SYMMETRIX_ID")
	if symID == "" {
		return PowermaxClonner{}, fmt.Errorf("Please set POWERMAX_SYMMETRIX_ID in the pod environment or in the secret" +
			" attached to the relevant storage map")
	}
	// using the same application name as the driver
	applicationName := "csi"
	client, err := gopowermax.NewClientWithArgs(
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
	return PowermaxClonner{client: client, symmetrixID: symID}, nil
}
