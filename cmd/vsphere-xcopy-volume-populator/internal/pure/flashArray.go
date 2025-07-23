package pure

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator"
	"github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/vmware"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
)

const FlashProviderID = "624a9370"

type FlashArrayClonner struct {
	restClient    *RestClient
	clusterPrefix string
}

const ClusterPrefixEnv = "PURE_CLUSTER_PREFIX"
const helpMessage = `clusterPrefix is missing. Please copy the cluster uuid and pass it in the pure secret under PURE_CLUSTER_PREFIX. use that to help \
oc get storagecluster -o yaml -A -o=jsonpath='{.items[?(@.spec.cloudStorage.provider=="pure")].status.clusterUid} | head -c 8'
`

func NewFlashArrayClonner(hostname, username, password string, skipSSLVerification bool, clusterPrefix string) (FlashArrayClonner, error) {
	if clusterPrefix == "" {
		return FlashArrayClonner{}, fmt.Errorf(helpMessage)
	}

	// Create the REST client for all operations
	restClient, err := NewRestClient(hostname, username, password)
	if err != nil {
		return FlashArrayClonner{}, fmt.Errorf("failed to create REST client: %w", err)
	}

	return FlashArrayClonner{
		restClient:    restClient,
		clusterPrefix: clusterPrefix,
	}, nil
}

// EnsureClonnerIgroup creates or updates an initiator group with the clonnerIqn
// Named hgroup in flash terminology
func (f *FlashArrayClonner) EnsureClonnerIgroup(initiatorGroup string, clonnerIqn []string) (populator.MappingContext, error) {
	// pure does not allow a single host to connect to 2 separae groups. Hence
	// we must connect map the volume to the host, and not to the group
	hostNames := []string{}
	hosts, err := f.restClient.ListHosts()
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
				err := f.restClient.ConnectHost(host, targetLUN.Name)
				if err != nil {
					if strings.Contains(err.Error(), "Connection already exists.") {
						continue
					}
					return populator.LUN{}, err
				}

			}
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
				err := f.restClient.DisconnectHost(host, targetLUN.Name)
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

// ResolvePVToLUN resolves a PersistentVolume to Pure FlashArray LUN details
func (f *FlashArrayClonner) ResolvePVToLUN(pv populator.PersistentVolume) (populator.LUN, error) {
	klog.Infof("Resolving target volume for PV %s", pv.Name)

	volumeName := fmt.Sprintf("%s-%s", f.clusterPrefix, pv.Name)
	klog.Infof("Target volume name: %s", volumeName)

	v, err := f.restClient.GetVolume(volumeName)
	if err != nil {
		return populator.LUN{}, fmt.Errorf("failed to get target volume from Pure FlashArray: %w", err)
	}

	naa := FlashProviderID + strings.ToLower(v.Serial)

	l := populator.LUN{
		Name:         v.Name,
		SerialNumber: v.Serial,
		NAA:          naa,
	}

	klog.Infof("Target volume resolved: %s (Serial: %s)", l.Name, l.SerialNumber)
	return l, nil
}

// CopyWithVSphere performs a direct copy operation using vSphere API to discover source volume
func (f *FlashArrayClonner) VvolCopy(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint) error {
	klog.Infof("Starting VVol copy operation for VM %s", vmId)

	// Parse the VMDK path
	vmDisk, err := populator.ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		return fmt.Errorf("failed to parse VMDK path: %w", err)
	}

	// Resolve target volume details
	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}

	// Try to get source volume from vSphere API
	sourceVolume, err := f.getSourceVolumeFromVSphere(vsphereClient, vmId, vmDisk)
	if err != nil {
		return fmt.Errorf("failed to get source volume from vSphere: %w", err)
	}

	klog.Infof("Copying from source volume %s to target volume %s", sourceVolume, targetLUN.Name)

	// Perform the copy operation
	err = f.performVolumeCopy(sourceVolume, targetLUN.Name, progress)
	if err != nil {
		return fmt.Errorf("copy operation failed: %w", err)
	}

	klog.Infof("VVol copy operation completed successfully")
	return nil
}

// getSourceVolumeFromVSphere uses vSphere API to find the Pure volume name for a VMDK
func (f *FlashArrayClonner) getSourceVolumeFromVSphere(vsphereClient vmware.Client, vmId string, vmDisk populator.VMDisk) (string, error) {
	ctx := context.Background()

	// Get VM object from vSphere
	finder := find.NewFinder(vsphereClient.(*vmware.VSphereClient).Client.Client, true)
	vm, err := finder.VirtualMachine(ctx, vmId)
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %w", err)
	}

	// Get VM hardware configuration
	var vmObject mo.VirtualMachine
	pc := property.DefaultCollector(vsphereClient.(*vmware.VSphereClient).Client.Client)
	err = pc.RetrieveOne(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmObject)
	if err != nil {
		return "", fmt.Errorf("failed to get VM hardware config: %w", err)
	}

	// Look through VM's virtual disks to find VVol backing
	if vmObject.Config == nil || vmObject.Config.Hardware.Device == nil {
		return "", fmt.Errorf("VM config or hardware devices not found")
	}

	for _, device := range vmObject.Config.Hardware.Device {
		if disk, ok := device.(*types.VirtualDisk); ok {
			if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
				// Check if this is a VVol backing and matches our target VMDK
				if backing.BackingObjectId != "" && f.matchesVMDKPath(backing.FileName, vmDisk) {
					klog.Infof("Found VVol backing for VMDK %s with ID %s", vmDisk.VmdkFile, backing.BackingObjectId)

					// Use REST client to find the volume by VVol ID
					volumeName, err := f.restClient.FindVolumeByVVolID(backing.BackingObjectId)
					if err != nil {
						klog.Warningf("Failed to find volume by VVol ID %s: %v", backing.BackingObjectId, err)
						continue
					}

					return volumeName, nil
				}
			}
		}
	}

	return "", fmt.Errorf("VVol backing for VMDK %s not found", vmDisk.VmdkFile)
}

// matchesVMDKPath checks if a vSphere VVol filename matches the target VMDK
func (f *FlashArrayClonner) matchesVMDKPath(fileName string, vmDisk populator.VMDisk) bool {
	return strings.Contains(fileName, vmDisk.VmdkFile)
}

// performVolumeCopy executes the volume copy operation on Pure FlashArray
func (f *FlashArrayClonner) performVolumeCopy(sourceVolumeName, targetVolumeName string, progress chan<- uint) error {
	// Perform the copy operation using Pure FlashArray API
	err := f.restClient.CopyVolume(sourceVolumeName, targetVolumeName)
	if err != nil {
		return fmt.Errorf("Pure FlashArray CopyVolume failed: %w", err)
	}

	progress <- 100
	return nil
}
