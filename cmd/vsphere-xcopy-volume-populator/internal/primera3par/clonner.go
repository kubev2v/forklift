package primera3par

import (
	"context"
	"fmt"
	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
)

const PROVIDER_ID = "60002ac"

type Primera3ParClonner struct {
	client Primera3ParClient
}

func NewPrimera3ParClonner(storageHostname, storageUsername, storagePassword string, sslSkipVerify bool) (Primera3ParClonner, error) {
	clon := NewPrimera3ParClientWsImpl(storageHostname, storageUsername, storagePassword, sslSkipVerify)
	return Primera3ParClonner{
		client: &clon,
	}, nil
}

// EnsureClonnerIgroup creates or update an initiator group with the clonnerIqn
func (c *Primera3ParClonner) EnsureClonnerIgroup(initiatorGroup string, adapterIds []string) (populator.MappingContext, error) {
	hostNames, err := c.client.EnsureHostsWithIds(adapterIds)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure host with IQN: %w", err)
	}

	err = c.client.EnsureHostSetExists(initiatorGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure host set: %w", err)
	}

	for _, hostName := range hostNames {
		klog.Infof("adding host %s, to initiatorGroup: %s", hostName, initiatorGroup)
		err = c.client.AddHostToHostSet(initiatorGroup, hostName)
		if err != nil {
			return nil, fmt.Errorf("failed to add host to host set: %w", err)
		}
	}
	return nil, nil
}

func (p *Primera3ParClonner) GetNaaID(lun populator.LUN) populator.LUN {
	return lun
}

// Map is responsible to mapping an initiator group to a LUN
func (c *Primera3ParClonner) Map(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) (populator.LUN, error) {
	return c.client.EnsureLunMapped(initiatorGroup, targetLUN)
}

// UnMap is responsible to unmapping an initiator group from a LUN
func (c *Primera3ParClonner) UnMap(initiatorGroup string, targetLUN populator.LUN, mappingContext populator.MappingContext) error {
	return c.client.LunUnmap(context.TODO(), initiatorGroup, targetLUN.Name)
}

// Return initiatorGroups the LUN is mapped to
func (p *Primera3ParClonner) CurrentMappedGroups(targetLUN populator.LUN, mappingContext populator.MappingContext) ([]string, error) {
	res, err := p.client.CurrentMappedGroups(targetLUN.Name, nil)
	if err != nil {
		return []string{}, fmt.Errorf("failed to get current mapped groups: %w", err)
	}
	return res, nil
}

func (c *Primera3ParClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	lun := populator.LUN{VolumeHandle: pv.VolumeHandle}
	lun, err := c.client.GetLunDetailsByVolumeName(pv.VolumeHandle, lun)
	if err != nil {
		return populator.LUN{}, err
	}
	return lun, nil
}
