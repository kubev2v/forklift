package ontap

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/fcutil"
	"github.com/kubev2v/forklift/cmd/vsphere-copy-offload-populator/internal/populator"
	drivers "github.com/netapp/trident/storage_drivers"
	"github.com/netapp/trident/storage_drivers/ontap/api"
	"k8s.io/klog/v2"
)

const (
	OntapProviderID = "600a0980"
	loggerName      = "copy-offload"
)

// Ensure NetappClonner implements required interfaces
var _ populator.VMDKCapable = &NetappClonner{}
var _ populator.StorageArrayInfoProvider = &NetappClonner{}

type NetappClonner struct {
	api                  api.OntapAPI
	initiatorHostOrGroup string
	arrayInfo            populator.StorageArrayInfo
}

// GetStorageArrayInfo returns metadata about the ONTAP array for metric labels.
func (c *NetappClonner) GetStorageArrayInfo() populator.StorageArrayInfo {
	return c.arrayInfo
}

// Map the targetLUN to the initiator group.
func (c *NetappClonner) Map(initatorGroup string, targetLUN populator.LUN, _ populator.MappingContext) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("mapping volume to group", "volume", targetLUN.Name, "group", initatorGroup)

	_, err := c.api.EnsureLunMapped(context.TODO(), initatorGroup, targetLUN.Name)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("Failed to map lun path %s to group %s: %w ", targetLUN.Name, initatorGroup, err)
	}

	log.Info("volume mapped successfully", "volume", targetLUN.Name, "group", initatorGroup)
	return targetLUN, nil
}

func (c *NetappClonner) UnMap(initatorGroup string, targetLUN populator.LUN, _ populator.MappingContext) error {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.Info("unmapping volume from group", "volume", targetLUN.Name, "group", initatorGroup)

	err := c.api.LunUnmap(context.TODO(), initatorGroup, targetLUN.Name)
	if err != nil {
		return err
	}

	log.Info("volume unmapped successfully", "volume", targetLUN.Name, "group", initatorGroup)
	return nil
}

func (c *NetappClonner) MapTarget(targetLUN populator.LUN, context populator.MappingContext) (populator.LUN, error) {
	return c.Map(c.initiatorHostOrGroup, targetLUN, context)
}

func (c *NetappClonner) UnmapTarget(targetLUN populator.LUN, context populator.MappingContext) error {
	return c.UnMap(c.initiatorHostOrGroup, targetLUN, context)
}

func (c *NetappClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	log := klog.Background().WithName(loggerName).WithName("map").WithName("ensure-igroup")
	log.Info("ensuring initiator group", "group", initiatorGroup, "adapters", adapterIds)

	// Detect protocol from adapters to avoid mixed protocol groups
	protocol := "mixed"
	for _, id := range adapterIds {
		if strings.HasPrefix(id, "fc.") || strings.HasPrefix(id, "20") {
			protocol = "fcp" // NetApp uses 'fcp' for Fibre Channel protocol
			break
		}
		if strings.HasPrefix(id, "iqn.") || strings.HasPrefix(id, "eui.") || strings.HasPrefix(id, "nqn.") {
			protocol = "iscsi"
			break
		}
	}

	// Append protocol suffix to avoid mixed protocol igroup errors
	c.initiatorHostOrGroup = initiatorGroup + "-" + protocol
	log.V(2).Info("detected protocol", "protocol", protocol, "final_group", c.initiatorHostOrGroup)

	// esxs needs "vmware" as the group protocol.
	err := c.api.IgroupCreate(context.Background(), c.initiatorHostOrGroup, protocol, "vmware")
	if err != nil {
		// TODO ignore if exists error? with ontap there is no error
		return nil, fmt.Errorf("failed adding igroup %w", err)
	}

	atLeastOneAdded := false

	for _, adapterId := range adapterIds {
		// Convert FC initiators from ESXi format (fc.WWNN:WWPN) to ONTAP format (colon-separated WWPN)
		ontapInitiator := adapterId
		if strings.HasPrefix(adapterId, "fc.") {
			converted, convErr := fcutil.ExtractAndFormatWWPN(adapterId)
			if convErr != nil {
				log.Info("failed to convert FC adapter to ONTAP format", "adapter", adapterId, "err", convErr)
				continue
			}
			log.V(2).Info("converted FC adapter to ONTAP format", "adapter", adapterId, "converted", converted)
			ontapInitiator = converted
		}

		err = c.api.EnsureIgroupAdded(context.Background(), c.initiatorHostOrGroup, ontapInitiator)
		if err != nil {
			log.Info("failed adding initiator to igroup", "initiator", ontapInitiator, "err", err)
			if strings.Contains(err.Error(), "[409]") {
				// duplicate initiator in a group
				atLeastOneAdded = true
			}
			continue
		}
		atLeastOneAdded = true
	}
	if !atLeastOneAdded {
		return nil, fmt.Errorf("failed adding any host to igroup")
	}

	log.Info("initiator group ready", "group", c.initiatorHostOrGroup)
	return nil, nil
}

func NewNetappClonner(hostname, username, password string) (NetappClonner, error) {
	log := klog.Background().WithName(loggerName).WithName("setup")

	// additional ontap values should be passed as env variables using prefix ONTAP_
	svm := os.Getenv("ONTAP_SVM")
	config := drivers.OntapStorageDriverConfig{
		CommonStorageDriverConfig: &drivers.CommonStorageDriverConfig{},
		ManagementLIF:             hostname,
		Username:                  username,
		Password:                  password,
		LimitAggregateUsage:       "",
		SVM:                       svm,
	}

	client, err := api.NewRestClientFromOntapConfig(context.TODO(), &config)
	if err != nil {
		log.V(2).Info("ONTAP client initialization error details", "err", err)
		return NetappClonner{}, fmt.Errorf("failed to initialize ONTAP client (common causes: incorrect password, invalid SVM name, network connectivity): %w", err)
	}

	nc := NetappClonner{
		api: client,
		arrayInfo: populator.StorageArrayInfo{
			Vendor:  "NetApp",
			Product: "ONTAP",
		},
	}

	// Fetch ONTAP API version
	ontapVersion, err := client.APIVersion(context.TODO())
	if err != nil {
		log.Info("failed to get ONTAP version for metrics", "err", err)
	} else {
		nc.arrayInfo.Version = ontapVersion
		log.V(2).Info("ONTAP array info", "vendor", nc.arrayInfo.Vendor, "product", nc.arrayInfo.Product, "version", nc.arrayInfo.Version)
	}

	return nc, nil
}

// parseInternalIDToLunPath converts internalID format to LUN path format.
// internalID format: /svm/{svm}/flexvol/{flexvol}/lun/{lun}
// LUN path format: /vol/{flexvol}/{lun}
func parseInternalIDToLunPath(internalID string) (string, error) {
	// Find the flexvol section
	_, reminder, ok := strings.Cut(internalID, "/flexvol/")
	if !ok {
		return "", fmt.Errorf("invalid internalID format: missing /flexvol/ in %s", internalID)
	}

	// Validate that the remainder contains /lun/
	if !strings.Contains(reminder, "/lun/") {
		return "", fmt.Errorf("invalid internalID format: missing /lun/ in %s", internalID)
	}

	flexVol, lunName, ok := strings.Cut(reminder, "/lun/")
	if !ok {
		return "", fmt.Errorf("invalid internalID format: missing /lun/ in %s", internalID)
	}

	// Prepend "/vol/" to convert the format
	return fmt.Sprintf("/vol/%s/%s", flexVol, lunName), nil
}

func (c *NetappClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	log := klog.Background().WithName(loggerName).WithName("resolve")
	log.Info("resolving PV to LUN", "pv", pv.Name, "volume_handle", pv.VolumeHandle)

	var lunPath string

	// Check for ontap-san-economy storage class (has internalID with full path)
	if internalID, ok := pv.VolumeAttributes["internalID"]; ok {
		log.V(2).Info("using economy storage class path resolution", "pv", pv.Name, "internal_id", internalID)
		parsedPath, err := parseInternalIDToLunPath(internalID)
		if err != nil {
			return populator.LUN{}, fmt.Errorf("failed to parse internalID for PV %s: %w", pv.Name, err)
		}
		lunPath = parsedPath
		log.V(2).Info("parsed LUN path from internalID", "lun_path", lunPath)
	} else {
		// Standard ontap-san storage class - uses dedicated FlexVol with lun0
		internalName, ok := pv.VolumeAttributes["internalName"]
		if !ok {
			return populator.LUN{}, fmt.Errorf("neither internalID nor internalName attribute found on PersistentVolume %s", pv.Name)
		}
		lunPath = fmt.Sprintf("/vol/%s/lun0", internalName)
		log.V(2).Info("using standard storage class LUN path", "lun_path", lunPath)
	}

	l, err := c.api.LunGetByName(context.Background(), lunPath)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get LUN at path %s: %w", lunPath, err)
	}

	// in RHEL lsblk needs that swap. In fedora it doesn't
	//serialNumber :=  strings.ReplaceAll(l.SerialNumber, "?", "\\\\x3f")
	naa := fmt.Sprintf("naa.%s%x", OntapProviderID, l.SerialNumber)
	lun := populator.LUN{Name: l.Name, VolumeHandle: pv.VolumeHandle, SerialNumber: l.SerialNumber, NAA: naa}

	log.Info("LUN resolved", "lun", lun.Name, "naa", lun.NAA, "serial", lun.SerialNumber)
	return lun, nil
}

func (c *NetappClonner) Get(lun populator.LUN, _ populator.MappingContext) (string, error) {
	// this code is from netapp/trident/storage_drivers/ontap/ontap_common.go
	// FIXME - this ips list needs to be intersected with the list of reporting
	// nodes for the LUN? see c.api.LunMapGetReportingNodes
	ips, err := c.api.NetInterfaceGetDataLIFs(context.Background(), "iscsi")
	if err != nil || len(ips) < 1 {
		return "", err
	}
	return ips[0], nil
}

func (c *NetappClonner) CurrentMappedGroups(targetLUN populator.LUN, _ populator.MappingContext) ([]string, error) {
	log := klog.Background().WithName(loggerName).WithName("map")
	log.V(2).Info("querying current mapped groups", "lun", targetLUN.Name)

	lunMappedIgroups, err := c.api.LunListIgroupsMapped(context.Background(), targetLUN.Name)
	if err != nil {
		return nil, fmt.Errorf("Failed to get mapped luns by path %s: %w ", targetLUN.Name, err)
	}

	log.V(2).Info("found mapped groups", "lun", targetLUN.Name, "groups", lunMappedIgroups)
	return lunMappedIgroups, nil
}
