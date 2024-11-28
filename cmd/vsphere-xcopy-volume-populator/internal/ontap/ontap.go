package ontap

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	drivers "github.com/netapp/trident/storage_drivers"
	"github.com/netapp/trident/storage_drivers/ontap/api"
	"k8s.io/klog/v2"
)

const OntapProviderID = "600a0980"

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

func (c *NetappClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn string) (populator.MappingContext, error) {
	// esxs needs "vmware" as the group protocol.
	err := c.api.IgroupCreate(context.Background(), initiatorGroup, "iscsi", "vmware")
	if err != nil {
		// TODO ignore if exists error? with ontap there is no error
		return nil, fmt.Errorf("failed adding igroup %w", err)
	}
	err = c.api.EnsureIgroupAdded(context.Background(), initiatorGroup, clonnerIqn)
	if err != nil {
		return nil, fmt.Errorf("failed adding host to igroup %w", err)
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
		return NetappClonner{}, err
	}

	nc := NetappClonner{api: client}
	return nc, nil
}

func (c *NetappClonner) ResolveVolumeHandleToLUN(volumeHandle string) (populator.LUN, error) {
	// for trident we need convert the dashes to underscores so pvc-123-456 becomes pvc_123_456
	volumeHandle = strings.ReplaceAll(volumeHandle, "-", "_")
	l, err := c.api.LunGetByName(context.Background(), fmt.Sprintf("/vol/trident_%s/lun0", volumeHandle))
	if err != nil {
		return populator.LUN{}, err
	}

	klog.Infof("found lun %s with serial %s", l.Name, l.SerialNumber)
	// in RHEL lsblk needs that swap. In fedora it doesn't
	//serialNumber :=  strings.ReplaceAll(l.SerialNumber, "?", "\\\\x3f")
	naa := fmt.Sprintf("%s%x", OntapProviderID, l.SerialNumber)
	lun := populator.LUN{Name: l.Name, VolumeHandle: volumeHandle, SerialNumber: l.SerialNumber, NAA: naa}
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
