package pure

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/devans10/pugo/flasharray"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/fcutil"
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

// EnsureClonnerIgroup creates or updates an initiator group with the ESX adapters
// Named hgroup in flash terminology
func (f *FlashArrayClonner) EnsureClonnerIgroup(initiatorGroup string, esxAdapters []string) (populator.MappingContext, error) {
	// pure does not allow a single host to connect to 2 separae groups. Hence
	// we must connect map the volume to the host, and not to the group
	hosts, err := f.client.Hosts.ListHosts(nil)
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		klog.Infof("checking host %s, iqns: %v, wwns: %v", h.Name, h.Iqn, h.Wwn)
		for _, iqn := range h.Iqn {
			if slices.Contains(esxAdapters, iqn) {
				klog.Infof("adding host to group %v", h.Name)
				return populator.MappingContext{"hosts": []string{h.Name}}, nil
			}
		}
		for _, wwn := range h.Wwn {
			for _, hostAdapter := range esxAdapters {
				if !strings.HasPrefix(hostAdapter, "fc.") {
					continue
				}
				adapterWWPN, err := fcUIDToWWPN(hostAdapter)
				if err != nil {
					klog.Warningf("failed to extract WWPN from adapter %s: %s", hostAdapter, err)
					continue
				}

				// Compare WWNs using the utility function that normalizes formatting
				klog.Infof("comparing ESX adapter WWPN %s with Pure host WWN %s", adapterWWPN, wwn)
				if fcutil.CompareWWNs(adapterWWPN, wwn) {
					klog.Infof("match found. Adding host %s to mapping context.", h.Name)
					return populator.MappingContext{"hosts": []string{h.Name}}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no hosts found matching any of the provided IQNs/FC adapters: %v", esxAdapters)
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
			return populator.LUN{}, fmt.Errorf("connect host %q to volume %q: %w", host, targetLUN.Name, err)
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

// fcUIDToWWPN extracts the WWPN (port name) from an ESXi fcUid string.
// The expected input is of the form: 'fc.WWNN:WWPN' where the WWNN and WWPN
// are not separated with columns every byte (2 hex chars) like 00:00:00:00:00:00:00:00
func fcUIDToWWPN(fcUid string) (string, error) {
	return fcutil.ExtractAndFormatWWPN(fcUid)
}
