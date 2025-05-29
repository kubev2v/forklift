package pure

import (
	"slices"
	"strings"

	"github.com/devans10/pugo/flasharray"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const FlashProviderID = "624a9370"

type FlashArrayClonner struct {
	client *flasharray.Client
}

func NewFlashArrayClonner(hostname, username, password string, skipSSLVerification bool) (FlashArrayClonner, error) {
	client, err := flasharray.NewClient(
		hostname, username, password, "", "", true, false, "", map[string]string{})
	if err != nil {
		return FlashArrayClonner{}, err
	}
	array, err := client.Array.Get(nil)
	if err != nil {
		klog.Fatalf("Error getting array status: %v", err)
	}
	klog.Infof("Array Name: %s, ID: %s", array.ArrayName, array.ID)
	return FlashArrayClonner{client: client}, nil
}

// EnsureClonnerIgroup creates or updates an initiator group with the clonnerIqn
// Named hgroup in flash terminology
func (f *FlashArrayClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (populator.MappingContext, error) {
	// pure does not allow a single host to connect to 2 separae groups. Hence
	// we must connect map the volume to the host, and not to the group
	hostNames := []string{}
	hosts, err := f.client.Hosts.ListHosts(nil)
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		for _, iqn := range h.Iqn {
			if slices.Contains(clonnerIqn, iqn) {
				klog.Infof("adding host to group %v", h.Name)
				hostNames = append(hostNames, h.Name)
			}
		}
		for _, wwn := range h.Wwn {
			if slices.Contains(clonnerIqn, wwn) {
				klog.Infof("adding host to group %v", h.Name)
				hostNames = append(hostNames, h.Name)
			}
		}
	}
	return populator.MappingContext{"hosts": hostNames}, nil
}

// Map is responsible to mapping an initiator group to a populator.LUN
func (f *FlashArrayClonner) Map(
	initatorGroup string,
	targetLUN populator.LUN,
	context populator.MappingContext) (populator.LUN, error) {
	hosts, ok := context["hosts"]
	if ok {
		hs, ok := hosts.([]string)
		if ok && len(hs) > 0 {
			for _, host := range hs {
				klog.Infof("connecting host %s to volume %s", host, targetLUN.Name)
				_, err := f.client.Hosts.ConnectHost(host, targetLUN.Name, nil)
				if err != nil {
					if strings.Contains(err.Error(), "Connection already exists.") {
						continue
					}
					return populator.LUN{}, err
				}

			}
		}
	}

	if true {
		return targetLUN, nil
	}

	g, err := f.client.Hostgroups.GetHostgroup(initatorGroup, nil)
	if err != nil {
		return populator.LUN{}, err
	}
	connectedVolume, err := f.client.Hostgroups.ConnectHostgroup(g.Name, targetLUN.Name, nil)
	if err != nil {
		return populator.LUN{}, nil
	}
	klog.Infof("target LUN %v connected to volume %+v", targetLUN, connectedVolume)
	return targetLUN, nil
}

// UnMap is responsible to unmapping an initiator group from a populator.LUN
func (f *FlashArrayClonner) UnMap(initatorGroup string, targetLUN populator.LUN, context populator.MappingContext) error {
	hosts, ok := context["hosts"]
	if ok {
		hs, ok := hosts.([]string)
		if ok && len(hs) > 0 {
			for _, host := range hs {
				klog.Infof("disconnecting host %s from volume %s", host, targetLUN.Name)
				_, err := f.client.Hosts.DisconnectHost(host, targetLUN.Name)
				if err != nil {
					return err
				}

			}
		}
	}
	return nil
}

// CurrentMappedGroups returns the initiator groups the populator.LUN is mapped to
func (f *FlashArrayClonner) CurrentMappedGroups(targetLUN populator.LUN, context populator.MappingContext) ([]string, error) {
	connectedHostGroups := []string{}
	hgs, err := f.client.Hostgroups.ListHostgroups(nil)
	if err != nil {
		return nil, err
	}
	for _, hg := range hgs {
		connections, err := f.client.Hostgroups.ListHostgroupConnections(hg.Name)
		if err != nil {
			return nil, err
		}
		for _, c := range connections {
			if c.Vol == targetLUN.Name {
				connectedHostGroups = append(connectedHostGroups, hg.Name)
			}
		}
	}
	return nil, nil
}

func (f *FlashArrayClonner) ResolveVolumeHandleToLUN(volumeHandle string) (populator.LUN, error) {
	v, err := f.client.Volumes.GetVolume(volumeHandle, nil)
	if err != nil {
		return populator.LUN{}, err
	}
	klog.Infof("volume %+v\n", v)
	l := populator.LUN{Name: v.Name, SerialNumber: v.Serial, NAA: FlashProviderID + strings.ToLower(v.Serial)}
	return l, nil
}
