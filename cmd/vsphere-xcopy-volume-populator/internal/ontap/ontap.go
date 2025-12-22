package ontap

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	drivers "github.com/netapp/trident/storage_drivers"
	"github.com/netapp/trident/storage_drivers/ontap/api"
	"k8s.io/klog/v2"
)

const OntapProviderID = "600a0980"

// Ensure NetappClonner implements required interfaces
var _ populator.RDMCapable = &NetappClonner{}
var _ populator.VMDKCapable = &NetappClonner{}

type NetappClonner struct {
	api api.OntapAPI
}

// Map the targetLUN to the initiator group.
func (c *NetappClonner) Map(initatorGroup string, targetLUN populator.LUN, _ populator.MappingContext) (populator.LUN, error) {
	_, err := c.api.EnsureLunMapped(context.TODO(), initatorGroup, targetLUN.Name)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("Failed to map lun path %s to group %s: %w ", targetLUN.Name, initatorGroup, err)
	}
	return targetLUN, nil
}

func (c *NetappClonner) UnMap(initatorGroup string, targetLUN populator.LUN, _ populator.MappingContext) error {
	return c.api.LunUnmap(context.TODO(), initatorGroup, targetLUN.Name)
}

func (c *NetappClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	// esxs needs "vmware" as the group protocol.
	err := c.api.IgroupCreate(context.Background(), initiatorGroup, "mixed", "vmware")
	if err != nil {
		// TODO ignore if exists error? with ontap there is no error
		return nil, fmt.Errorf("failed adding igroup %w", err)
	}

	atLeastOneAdded := false

	for _, adapterId := range adapterIds {
		err = c.api.EnsureIgroupAdded(context.Background(), initiatorGroup, adapterId)
		if err != nil {
			klog.Warningf("failed adding host to igroup %s", err)
			continue
		}
		atLeastOneAdded = true
	}
	if !atLeastOneAdded {
		return nil, fmt.Errorf("failed adding any host to igroup")
	}
	return nil, nil
}

func NewNetappClonner(hostname, username, password string) (NetappClonner, error) {
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
		klog.V(2).Infof("ONTAP client initialization error details: %v", err)
		return NetappClonner{}, fmt.Errorf("failed to initialize ONTAP client (common causes: incorrect password, invalid SVM name, network connectivity): %w", err)
	}

	nc := NetappClonner{api: client}
	return nc, nil
}

func (c *NetappClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	// trident sets internalName attribute on a volume, and that is the real volume name in the system
	internalName, ok := pv.VolumeAttributes["internalName"]
	if !ok {
		return populator.LUN{}, fmt.Errorf("intenalName attribute is missing on the PersistentVolume %s", pv.Name)
	}
	l, err := c.api.LunGetByName(context.Background(), fmt.Sprintf("/vol/%s/lun0", internalName))
	if err != nil {
		return populator.LUN{}, err
	}

	klog.Infof("found lun %s with serial %s", l.Name, l.SerialNumber)
	// in RHEL lsblk needs that swap. In fedora it doesn't
	//serialNumber :=  strings.ReplaceAll(l.SerialNumber, "?", "\\\\x3f")
	naa := fmt.Sprintf("naa.%s%x", OntapProviderID, l.SerialNumber)
	lun := populator.LUN{Name: l.Name, VolumeHandle: pv.VolumeHandle, SerialNumber: l.SerialNumber, NAA: naa}
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
	lunMappedIgroups, err := c.api.LunListIgroupsMapped(context.Background(), targetLUN.Name)
	if err != nil {
		return nil, fmt.Errorf("Failed to get mapped luns by path %s: %w ", targetLUN.Name, err)
	}
	return lunMappedIgroups, nil
}

// RDMCopy performs a copy operation for RDM-backed disks using NetApp storage APIs
func (c *NetappClonner) RDMCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint64) error {
	klog.Infof("Ontap RDM Copy: Starting RDM copy operation for VM %s", vmId)

	// Get disk backing info to find the RDM device
	backing, err := vsphereClient.GetVMDiskBacking(context.Background(), vmId, sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to get RDM disk backing info: %w", err)
	}

	if !backing.IsRDM {
		return fmt.Errorf("disk %s is not an RDM disk", sourceVMDKFile)
	}

	klog.Infof("Ontap RDM Copy: Found RDM device: %s", backing.DeviceName)

	// Resolve the source LUN from the RDM device name
	sourceLUN, err := c.resolveRDMToLUN(backing.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to resolve RDM device to source LUN: %w", err)
	}

	// Resolve the target PV to LUN
	targetLUN, err := c.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	klog.Infof("Ontap RDM Copy: Copying from source LUN %s to target LUN %s", sourceLUN.Name, targetLUN.Name)

	// Report progress start
	progress <- 10

	// Extract volume name from target LUN path for clone operation
	// LUN path format is typically: /vol/<volume_name>/lun0
	targetVolName := extractVolumeName(targetLUN.Name)
	if targetVolName == "" {
		return fmt.Errorf("failed to extract volume name from LUN path: %s", targetLUN.Name)
	}

	// Use ONTAP's LUN clone capability to copy data
	// LunCloneCreate creates a clone from a source LUN to a new LUN in the target volume
	err = c.api.LunCloneCreate(context.Background(), targetVolName, sourceLUN.Name, "lun0", api.QosPolicyGroup{})
	if err != nil {
		return fmt.Errorf("ONTAP LUN clone failed: %w", err)
	}

	// Report progress complete
	progress <- 100

	klog.Infof("Ontap RDM Copy: Copy operation completed successfully")
	return nil
}

// resolveRDMToLUN resolves an RDM device name to an ONTAP LUN
func (c *NetappClonner) resolveRDMToLUN(deviceName string) (populator.LUN, error) {
	klog.Infof("Ontap RDM Copy: Resolving RDM device %s to LUN", deviceName)

	// The device name from RDM typically contains the NAA identifier
	// Format is usually like "naa.600a0980..." or just the hex serial
	// We need to find the corresponding LUN in ONTAP

	// List all LUNs and find the one matching the device name
	luns, err := c.api.LunList(context.Background(), "*")
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to list LUNs: %w", err)
	}

	for _, lun := range luns {
		// Compare serial numbers - the RDM device name contains the serial
		naa := fmt.Sprintf("naa.%s%x", OntapProviderID, lun.SerialNumber)
		if naa == deviceName || containsSerial(deviceName, lun.SerialNumber) {
			klog.Infof("Ontap RDM Copy: Found matching LUN %s for device %s", lun.Name, deviceName)
			return populator.LUN{
				Name:         lun.Name,
				SerialNumber: lun.SerialNumber,
				NAA:          naa,
			}, nil
		}
	}

	return populator.LUN{}, fmt.Errorf("could not find LUN matching RDM device %s", deviceName)
}

// extractVolumeName extracts the volume name from a LUN path
// LUN path format: /vol/<volume_name>/lun0
func extractVolumeName(lunPath string) string {
	parts := strings.Split(lunPath, "/")
	if len(parts) >= 3 && parts[1] == "vol" {
		return parts[2]
	}
	return ""
}

// containsSerial checks if a device name contains the serial number
func containsSerial(deviceName, serial string) bool {
	return strings.Contains(strings.ToLower(deviceName), strings.ToLower(serial))
}
