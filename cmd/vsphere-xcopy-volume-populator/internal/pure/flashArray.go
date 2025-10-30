package pure

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/devans10/pugo/flasharray"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"k8s.io/klog/v2"
)

const FlashProviderID = "624a9370"

type FlashArrayClonner struct {
	client        *flasharray.Client
	clusterPrefix string
}

const ClusterPrefixEnv = "PURE_CLUSTER_PREFIX"
const helpMessage = `clusterPrefix is missing and PURE_CLUSTER_PREFIX is not set.
Use this to extract the value:
printf "px_%.8s" $(oc get storagecluster -A -o=jsonpath='{.items[?(@.spec.cloudStorage.provider=="pure")].status.clusterUid}')
`

func NewFlashArrayClonner(hostname, username, password string, skipSSLVerification bool, clusterPrefix string) (FlashArrayClonner, error) {
	if clusterPrefix == "" {
		return FlashArrayClonner{}, errors.New(helpMessage)
	}
	client, err := flasharray.NewClient(
		hostname, username, password, "", "", true, false, "", map[string]string{})
	if err != nil {
		return FlashArrayClonner{}, err
	}
	array, err := client.Array.Get(nil)
	if err != nil {
		klog.Fatalf("Error getting array status: %v", err)
	}
	klog.Infof("Array Name: %s, ID: %s all %+v", array.ArrayName, array.ID, array)
	return FlashArrayClonner{client: client, clusterPrefix: clusterPrefix}, nil
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
		klog.Infof("checking host %s, iqns: %v, wwns: %v", h.Name, h.Iqn, h.Wwn)
		for _, iqn := range h.Iqn {
			if slices.Contains(clonnerIqn, iqn) {
				klog.Infof("adding host to group %v", h.Name)
				hostNames = append(hostNames, h.Name)
			}
		}
		for _, wwn := range h.Wwn {
			for _, fcUid := range clonnerIqn {
				adapterWWN, err := fcUidToWWN(fcUid)
				if err != nil {
					klog.Warningf("failed to format %s to wwn %s", fcUid, err)
					continue
				}
				klog.Infof("comparing wwn %s from the ESX adapter with host Wwn %s", adapterWWN, wwn)
				if adapterWWN == wwn {
					klog.Infof("adding host to group %v", h.Name)
					hostNames = append(hostNames, h.Name)
				}
			}
		}
	}
	if len(hostNames) == 0 {
		return nil, errors.New("no hosts found for the given IQNs/WWNs")
	}
	return populator.MappingContext{"hosts": hostNames}, nil
}

// Map is responsible to mapping an initiator group to a populator.LUN
func (f *FlashArrayClonner) Map(
	initatorGroup string,
	targetLUN populator.LUN,
	context populator.MappingContext) (populator.LUN, error) {
	hostsVal, ok := context["hosts"]
	if !ok {
		return populator.LUN{}, errors.New("hosts not found in mapping context")
	}

	hosts, ok := hostsVal.([]string)
	if !ok || len(hosts) == 0 {
		return populator.LUN{}, errors.New("invalid or empty hosts list in mapping context")
	}

	for _, host := range hosts {
		klog.Infof("connecting host %s to volume %s", host, targetLUN.Name)
		_, err := f.client.Hosts.ConnectHost(host, targetLUN.Name, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Connection already exists.") {
				continue
			}
			return populator.LUN{}, err
		}
	}

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
	// we don't use the host group feature, as a host in pure flasharray can not belong to two separate groups, and we
	// definitely don't want to break host from their current groups. insted we'll just map/unmap the volume to individual hosts
	return nil, nil
}

func (f *FlashArrayClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	v, err := f.client.Volumes.GetVolume(fmt.Sprintf("%s-%s", f.clusterPrefix, pv.Name), nil)
	if err != nil {
		return populator.LUN{}, err
	}
	klog.Infof("volume %+v\n", v)
	l := populator.LUN{Name: v.Name, SerialNumber: v.Serial, NAA: fmt.Sprintf("naa.%s%s", FlashProviderID, strings.ToLower(v.Serial))}
	return l, nil
}

func fcUidToWWN(fcUid string) (string, error) {
	// the fcuuid expected input of the form:
	// 'fc.20000025b5120030:20000025b56a0030'
	//  fc.    | 20000025b5120030 | : | 20000025b56a0030
	//  prefix | WWNN (node name) | : | WWPN (port name)
	// string the prefix and use the WWNN
	formattedWwn, ok := strings.CutPrefix(fcUid, "fc.")
	if !ok {
		return "", fmt.Errorf("fcUid %q doesn't strat with 'fc.'", fcUid)
	}
	formattedWwn = formattedWwn[:strings.Index(formattedWwn, ":")]
	formattedWwn = strings.ToUpper(formattedWwn)
	if len(formattedWwn)%2 != 0 {
		return "", fmt.Errorf("fcUid %q length isn't even", fcUid)
	}
	var parts []string
	for i := 0; i < len(formattedWwn); i += 2 {
		parts = append(parts, formattedWwn[i:i+2])
	}
	return strings.Join(parts, ":"), nil

}
