package ontap

import (
	"context"
	"fmt"
	"os"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	drivers "github.com/netapp/trident/storage_drivers"
	"github.com/netapp/trident/storage_drivers/ontap/api"
	"k8s.io/klog/v2"
)

const OntapProviderID = "600a0980"

// Ensure NetappClonner implements required interfaces
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
