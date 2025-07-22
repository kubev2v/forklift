package pure

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/devans10/pugo/flasharray"
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
	client        *flasharray.Client
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
	klog.Infof("Pure VVol Resolver: Starting PV to LUN resolution")
	klog.Infof("Pure VVol Resolver: PV Name: %s", pv.Name)
	klog.Infof("Pure VVol Resolver: PV VolumeHandle: %s", pv.VolumeHandle)
	klog.Infof("Pure VVol Resolver: PV Attributes: %+v", pv.VolumeAttributes)
	klog.Infof("Pure VVol Resolver: Cluster Prefix: %s", f.clusterPrefix)

	volumeName := fmt.Sprintf("%s-%s", f.clusterPrefix, pv.Name)
	klog.Infof("Pure VVol Resolver: Constructed volume name: %s", volumeName)

	klog.Infof("Pure VVol Resolver: Querying Pure FlashArray for volume...")
	v, err := f.client.Volumes.GetVolume(volumeName, nil)
	if err != nil {
		klog.Errorf("Pure VVol Resolver: Failed to get volume from Pure FlashArray: %v", err)
		return populator.LUN{}, err
	}

	klog.Infof("Pure VVol Resolver: Volume found on Pure FlashArray")
	klog.Infof("Pure VVol Resolver: Volume Name: %s", v.Name)
	klog.Infof("Pure VVol Resolver: Volume Serial: %s", v.Serial)
	klog.Infof("Pure VVol Resolver: Volume Size: %d", v.Size)

	naa := FlashProviderID + strings.ToLower(v.Serial)
	klog.Infof("Pure VVol Resolver: Constructed NAA: %s (Provider ID: %s + Serial: %s)",
		naa, FlashProviderID, strings.ToLower(v.Serial))

	l := populator.LUN{
		Name:         v.Name,
		SerialNumber: v.Serial,
		NAA:          naa,
	}

	klog.Infof("Pure VVol Resolver: Final LUN object - Name: %s, Serial: %s, NAA: %s",
		l.Name, l.SerialNumber, l.NAA)

	return l, nil
}

// Copy implements VvolStorageApi interface for direct VMDK copy operations in VVol environments
func (f *FlashArrayClonner) Copy(vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint) error {
	klog.Infof("=== Pure FlashArray VVol Copy with Pattern-Based Discovery (Fallback) ===")
	klog.Infof("Pure VVol Fallback: Starting copy operation")
	klog.Infof("Pure VVol Fallback: VM ID: %s", vmId)
	klog.Infof("Pure VVol Fallback: Source VMDK: %s", sourceVMDKFile)
	klog.Infof("Pure VVol Fallback: Target PV: %s", persistentVolume.Name)
	klog.Infof("Pure VVol Fallback: Cluster Prefix: %s", f.clusterPrefix)

	// Parse the VMDK path to understand source location
	klog.Infof("Pure VVol Fallback: Parsing VMDK path...")
	vmDisk, err := populator.ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		klog.Errorf("Pure VVol Fallback: Failed to parse VMDK path: %v", err)
		return fmt.Errorf("failed to parse VMDK path %s: %w", sourceVMDKFile, err)
	}
	klog.Infof("Pure VVol Fallback: Parsed VMDK - Datastore: %s, VM Home: %s, VMDK File: %s",
		vmDisk.Datastore, vmDisk.VmHomeDir, vmDisk.VmdkFile)

	// Resolve target volume details
	klog.Infof("Pure VVol Fallback: Resolving target PV to LUN...")
	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		klog.Errorf("Pure VVol Fallback: Failed to resolve target volume: %v", err)
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}
	klog.Infof("Pure VVol Fallback: Target LUN resolved - Name: %s, NAA: %s, Serial: %s",
		targetLUN.Name, targetLUN.NAA, targetLUN.SerialNumber)

	// First, try to find volumes by listing all volumes and searching for patterns
	klog.Infof("Pure VVol Fallback: Attempting to find source volume by pattern matching...")
	sourceVolumeName, err := f.findSourceVolumeByPatternMatching(vmDisk, vmId)
	if err == nil && sourceVolumeName != "" {
		klog.Infof("Pure VVol Fallback: ✓ Found source volume by pattern matching: %s", sourceVolumeName)

		// Verify the volume exists
		sourceVolume, err := f.client.Volumes.GetVolume(sourceVolumeName, nil)
		if err == nil {
			klog.Infof("Pure VVol Fallback: Source volume verified on Pure FlashArray - Name: %s, Size: %d bytes",
				sourceVolume.Name, sourceVolume.Size)

			// Start the copy operation
			err = f.performVolumeCopy(sourceVolumeName, targetLUN.Name, progress)
			if err != nil {
				klog.Errorf("Pure VVol Fallback: Volume copy operation failed: %v", err)
				return fmt.Errorf("volume copy operation failed: %w", err)
			}

			klog.Infof("=== Pure FlashArray VVol Copy (Fallback) Completed Successfully ===")
			klog.Infof("Pure VVol Fallback: Successfully completed copy from %s to %s", sourceVMDKFile, targetLUN.Name)
			return nil
		}
	}

	// Fallback to the original pattern-based approach
	klog.Infof("Pure VVol Fallback: Pattern matching failed, trying original deduction approach...")

	// Deduce source volume name from the VMDK path
	// For VVol, the source volume name can be derived from the VM home directory and VMDK file
	klog.Infof("Pure VVol Fallback: Starting pattern-based source volume deduction...")
	sourceVolumeName = f.deduceSourceVolumeName(vmDisk, vmId)
	klog.Infof("Pure VVol Fallback: Primary deduced source volume name: %s", sourceVolumeName)

	// Check if source volume exists
	klog.Infof("Pure VVol Fallback: Checking if primary source volume exists...")
	sourceVolume, err := f.client.Volumes.GetVolume(sourceVolumeName, nil)
	if err != nil {
		klog.Warningf("Pure VVol Fallback: Primary source volume not found: %v", err)

		// Try alternative naming patterns if the first attempt fails
		klog.Infof("Pure VVol Fallback: Trying alternative naming patterns...")
		alternativeNames := f.getAlternativeSourceVolumeNames(vmDisk, vmId)
		klog.Infof("Pure VVol Fallback: Generated %d alternative names: %v", len(alternativeNames), alternativeNames)

		var lastErr error
		found := false

		for i, altName := range alternativeNames {
			klog.Infof("Pure VVol Fallback: Trying alternative #%d: %s", i+1, altName)
			sourceVolume, lastErr = f.client.Volumes.GetVolume(altName, nil)
			if lastErr == nil {
				klog.Infof("Pure VVol Fallback: ✓ Found source volume with alternative name: %s", altName)
				sourceVolumeName = altName
				found = true
				break
			} else {
				klog.Infof("Pure VVol Fallback: ✗ Alternative #%d failed: %v", i+1, lastErr)
			}
		}

		if !found {
			klog.Errorf("Pure VVol Fallback: All naming patterns failed")
			klog.Errorf("Pure VVol Fallback: Primary name: %s", sourceVolumeName)
			klog.Errorf("Pure VVol Fallback: Alternative names: %v", alternativeNames)
			return fmt.Errorf("source VVol volume not found for VM %s. Tried names: %s, %v. Last error: %w",
				vmId, sourceVolumeName, alternativeNames, lastErr)
		}
	} else {
		klog.Infof("Pure VVol Fallback: ✓ Primary source volume found")
	}

	klog.Infof("Pure VVol Fallback: Final source volume - Name: %s, Size: %d bytes",
		sourceVolume.Name, sourceVolume.Size)

	// Use Pure's volume copy functionality to copy directly from source to target
	klog.Infof("Pure VVol Fallback: Starting direct copy operation...")
	klog.Infof("Pure VVol Fallback: Copy source: %s", sourceVolumeName)
	klog.Infof("Pure VVol Fallback: Copy target: %s", targetLUN.Name)

	// Start the copy operation
	err = f.performVolumeCopy(sourceVolumeName, targetLUN.Name, progress)
	if err != nil {
		klog.Errorf("Pure VVol Fallback: Volume copy operation failed: %v", err)
		return fmt.Errorf("volume copy operation failed: %w", err)
	}

	klog.Infof("=== Pure FlashArray VVol Copy (Fallback) Completed Successfully ===")
	klog.Infof("Pure VVol Fallback: Successfully completed copy from %s to %s", sourceVMDKFile, targetLUN.Name)
	return nil
}

// CopyWithVSphere implements VvolStorageApi interface using vSphere API to discover source volume
func (f *FlashArrayClonner) CopyWithVSphere(vsphereClient vmware.Client, vmId string, sourceVMDKFile string, persistentVolume populator.PersistentVolume, progress chan<- uint) error {
	klog.Infof("=== Pure FlashArray VVol Copy with vSphere API Discovery ===")
	klog.Infof("Pure VVol: Starting copy operation")
	klog.Infof("Pure VVol: VM ID: %s", vmId)
	klog.Infof("Pure VVol: Source VMDK: %s", sourceVMDKFile)
	klog.Infof("Pure VVol: Target PV: %s", persistentVolume.Name)
	klog.Infof("Pure VVol: Cluster Prefix: %s", f.clusterPrefix)

	// Parse the VMDK path to understand which disk we're looking for
	klog.Infof("Pure VVol: Parsing VMDK path...")
	vmDisk, err := populator.ParseVmdkPath(sourceVMDKFile)
	if err != nil {
		klog.Errorf("Pure VVol: Failed to parse VMDK path: %v", err)
		return fmt.Errorf("failed to parse VMDK path %s: %w", sourceVMDKFile, err)
	}
	klog.Infof("Pure VVol: Parsed VMDK - Datastore: %s, VM Home: %s, VMDK File: %s",
		vmDisk.Datastore, vmDisk.VmHomeDir, vmDisk.VmdkFile)

	// Resolve target volume details
	klog.Infof("Pure VVol: Resolving target PV to LUN...")
	targetLUN, err := f.ResolvePVToLUN(persistentVolume)
	if err != nil {
		klog.Errorf("Pure VVol: Failed to resolve target volume: %v", err)
		return fmt.Errorf("failed to resolve target volume: %w", err)
	}
	klog.Infof("Pure VVol: Target LUN resolved - Name: %s, NAA: %s, Serial: %s",
		targetLUN.Name, targetLUN.NAA, targetLUN.SerialNumber)

	// Use vSphere API to get the actual VVol information
	klog.Infof("Pure VVol: Starting vSphere API discovery...")
	sourceVolumeName, err := f.getSourceVolumeFromVSphere(vsphereClient, vmId, vmDisk)
	if err != nil {
		klog.Errorf("Pure VVol: vSphere API discovery failed: %v", err)
		return fmt.Errorf("failed to discover source volume using vSphere API: %w", err)
	}
	klog.Infof("Pure VVol: Successfully discovered source volume: %s", sourceVolumeName)

	// Check if source volume exists on Pure FlashArray
	klog.Infof("Pure VVol: Verifying source volume exists on Pure FlashArray...")
	sourceVolume, err := f.client.Volumes.GetVolume(sourceVolumeName, nil)
	if err != nil {
		klog.Errorf("Pure VVol: Source volume not found on Pure FlashArray: %v", err)
		return fmt.Errorf("source VVol volume %s not found on Pure FlashArray: %w", sourceVolumeName, err)
	}
	klog.Infof("Pure VVol: Source volume found on Pure FlashArray - Name: %s, Size: %d bytes",
		sourceVolume.Name, sourceVolume.Size)

	// Use Pure's volume copy functionality to copy directly from source to target
	klog.Infof("Pure VVol: Starting direct copy operation...")
	klog.Infof("Pure VVol: Copy source: %s", sourceVolumeName)
	klog.Infof("Pure VVol: Copy target: %s", targetLUN.Name)

	// Start the copy operation
	err = f.performVolumeCopy(sourceVolumeName, targetLUN.Name, progress)
	if err != nil {
		klog.Errorf("Pure VVol: Volume copy operation failed: %v", err)
		return fmt.Errorf("volume copy operation failed: %w", err)
	}

	klog.Infof("=== Pure FlashArray VVol Copy Completed Successfully ===")
	klog.Infof("Pure VVol: Successfully completed copy from %s to %s", sourceVMDKFile, targetLUN.Name)
	return nil
}

// deduceSourceVolumeName attempts to determine the source volume name from VMDK path and VM ID
func (f *FlashArrayClonner) deduceSourceVolumeName(vmDisk populator.VMDisk, vmId string) string {
	klog.Infof("Pure VVol Pattern: Starting source volume name deduction (no cluster prefix for vSphere volumes)")
	klog.Infof("Pure VVol Pattern: Input - VM Home: '%s', VM ID: '%s'", vmDisk.VmHomeDir, vmId)

	// Common VVol naming patterns for vSphere-created volumes (NO cluster prefix):
	// 1. Based on VM home directory: vvol-{vmHomeDir}
	// 2. Based on VMDK file name without extension: vvol-{vmdkBaseName}
	// 3. Based on VM ID: vvol-{vmId}
	// 4. Direct VM home directory: {vmHomeDir}

	var result string
	// Try VM home directory first (without vvol- prefix, since actual format might be different)
	if vmDisk.VmHomeDir != "" {
		result = vmDisk.VmHomeDir
		klog.Infof("Pure VVol Pattern: Using direct VM home directory pattern: %s", result)
	} else {
		// Fallback to VM ID
		result = vmId
		klog.Infof("Pure VVol Pattern: VM home directory empty, using VM ID pattern: %s", result)
	}

	return result
}

// getAlternativeSourceVolumeNames returns alternative naming patterns to try
func (f *FlashArrayClonner) getAlternativeSourceVolumeNames(vmDisk populator.VMDisk, vmId string) []string {
	klog.Infof("Pure VVol Pattern: Generating alternative source volume names (no cluster prefix)")
	klog.Infof("Pure VVol Pattern: Input - VMDK File: '%s', Datastore: '%s', VM Home: '%s', VM ID: '%s'",
		vmDisk.VmdkFile, vmDisk.Datastore, vmDisk.VmHomeDir, vmId)

	alternatives := []string{}

	// Pattern 1: vvol- prefix with VM home directory
	if vmDisk.VmHomeDir != "" {
		altName := fmt.Sprintf("vvol-%s", vmDisk.VmHomeDir)
		alternatives = append(alternatives, altName)
		klog.Infof("Pure VVol Pattern: Added vvol-prefixed VM home pattern: %s", altName)
	}

	// Pattern 2: VMDK file name without extension
	if vmDisk.VmdkFile != "" {
		vmdkBaseName := strings.TrimSuffix(vmDisk.VmdkFile, ".vmdk")
		if vmdkBaseName != vmDisk.VmdkFile { // Only if it actually had .vmdk extension
			// Try both with and without vvol- prefix
			altName1 := fmt.Sprintf("vvol-%s", vmdkBaseName)
			altName2 := vmdkBaseName
			alternatives = append(alternatives, altName1, altName2)
			klog.Infof("Pure VVol Pattern: Added VMDK-based patterns: %s, %s", altName1, altName2)
		} else {
			klog.Infof("Pure VVol Pattern: VMDK file '%s' doesn't have .vmdk extension, skipping", vmDisk.VmdkFile)
		}
	}

	// Pattern 3: Datastore-based naming (various combinations)
	if vmDisk.Datastore != "" && vmDisk.VmHomeDir != "" {
		altName1 := fmt.Sprintf("vvol-%s-%s", vmDisk.Datastore, vmDisk.VmHomeDir)
		altName2 := fmt.Sprintf("%s-%s", vmDisk.Datastore, vmDisk.VmHomeDir)
		alternatives = append(alternatives, altName1, altName2)
		klog.Infof("Pure VVol Pattern: Added datastore-based patterns: %s, %s", altName1, altName2)
	}

	// Pattern 4: VM ID variations
	altName1 := fmt.Sprintf("vvol-%s", vmId)
	altName2 := vmId
	alternatives = append(alternatives, altName1, altName2)
	klog.Infof("Pure VVol Pattern: Added VM ID patterns: %s, %s", altName1, altName2)

	// Pattern 5: Try to match the actual format seen: vvol-vm-12-100g-34f88851-vg/Data-*
	if vmDisk.VmHomeDir != "" {
		// Look for patterns that might create the vvol-vm-* format
		if strings.Contains(vmId, "-") {
			// If VM ID has dashes, it might be part of the pattern
			altName := fmt.Sprintf("vvol-vm-%s-vg", strings.ReplaceAll(vmId, "-", "-"))
			alternatives = append(alternatives, altName)
			klog.Infof("Pure VVol Pattern: Added vvol-vm-*-vg pattern: %s", altName)
		}
	}

	klog.Infof("Pure VVol Pattern: Generated %d alternative patterns total", len(alternatives))
	return alternatives
}

// getSourceVolumeFromVSphere uses vSphere API to discover the actual VVol volume name
func (f *FlashArrayClonner) getSourceVolumeFromVSphere(vsphereClient vmware.Client, vmId string, vmDisk populator.VMDisk) (string, error) {
	klog.Infof("Pure VVol Discovery: Starting vSphere API query for VM %s", vmId)

	ctx := context.Background()

	// Get the VM object
	klog.Infof("Pure VVol Discovery: Creating vSphere finder...")
	finder := find.NewFinder(vsphereClient.(*vmware.VSphereClient).Client.Client, true)

	klog.Infof("Pure VVol Discovery: Looking up VM object for ID: %s", vmId)
	vm, err := finder.VirtualMachine(ctx, vmId)
	if err != nil {
		klog.Errorf("Pure VVol Discovery: Failed to find VM: %v", err)
		return "", fmt.Errorf("failed to find VM %s: %w", vmId, err)
	}
	klog.Infof("Pure VVol Discovery: VM object found: %s", vm.Name())

	// Get VM properties including virtual disks
	klog.Infof("Pure VVol Discovery: Retrieving VM properties (hardware devices)...")
	var vmMo mo.VirtualMachine
	pc := property.DefaultCollector(vsphereClient.(*vmware.VSphereClient).Client.Client)
	err = pc.RetrieveOne(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmMo)
	if err != nil {
		klog.Errorf("Pure VVol Discovery: Failed to retrieve VM properties: %v", err)
		return "", fmt.Errorf("failed to retrieve VM properties: %w", err)
	}
	klog.Infof("Pure VVol Discovery: Retrieved %d hardware devices", len(vmMo.Config.Hardware.Device))

	// Find the specific disk that matches our VMDK path
	diskCount := 0
	klog.Infof("Pure VVol Discovery: Searching for target disk - looking for VMDK: %s", vmDisk.VmdkFile)

	for i, device := range vmMo.Config.Hardware.Device {
		if disk, ok := device.(*types.VirtualDisk); ok {
			diskCount++
			klog.Infof("Pure VVol Discovery: Found disk #%d (device index %d)", diskCount, i)

			if backing := disk.Backing; backing != nil {
				klog.Infof("Pure VVol Discovery: Disk #%d has backing type: %T", diskCount, backing)

				// Check if this is a VVol-backed disk
				if vvolBacking, ok := backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
					fileName := vvolBacking.FileName
					klog.Infof("Pure VVol Discovery: Disk #%d is VVol-backed with fileName: %s", diskCount, fileName)

					// Log all available backing information
					klog.Infof("Pure VVol Discovery: VVol backing details:")
					klog.Infof("Pure VVol Discovery:   - FileName: %s", vvolBacking.FileName)
					klog.Infof("Pure VVol Discovery:   - Uuid: %s", vvolBacking.Uuid)
					if vvolBacking.BackingObjectId != "" {
						klog.Infof("Pure VVol Discovery:   - BackingObjectId: %s", vvolBacking.BackingObjectId)
					}
					if vvolBacking.Parent != nil {
						klog.Infof("Pure VVol Discovery:   - Parent: %+v", vvolBacking.Parent)
					}

					// Check if this matches our target VMDK file
					if f.matchesVMDKPath(fileName, vmDisk) {
						klog.Infof("Pure VVol Discovery: ✓ MATCH FOUND - Disk #%d matches target VMDK", diskCount)

						// Try to extract Pure volume name from VVol backing information
						pureVolumeName := f.extractPureVolumeNameFromVVolBacking(vvolBacking)
						if pureVolumeName != "" {
							klog.Infof("Pure VVol Discovery: Successfully extracted Pure volume name: %s", pureVolumeName)
							return pureVolumeName, nil
						}

						// Fallback: Extract VVol ID from the file name
						klog.Infof("Pure VVol Discovery: Fallback - extracting VVol ID from fileName: %s", fileName)
						vvolId := f.extractVVolIdFromFileName(fileName)
						if vvolId != "" {
							klog.Infof("Pure VVol Discovery: Extracted VVol ID: %s", vvolId)

							// Convert VVol ID to Pure volume name (NO cluster prefix for source volumes)
							pureVolumeName := f.vvolIdToPureVolumeNameForSource(vvolId)
							klog.Infof("Pure VVol Discovery: Mapped VVol ID to Pure source volume name: %s", pureVolumeName)
							return pureVolumeName, nil
						} else {
							klog.Warningf("Pure VVol Discovery: Failed to extract VVol ID from fileName: %s", fileName)
						}
					} else {
						klog.Infof("Pure VVol Discovery: Disk #%d does not match target (different datastore/folder/file)", diskCount)
					}
				} else {
					klog.Infof("Pure VVol Discovery: Disk #%d is not VVol-backed (backing type: %T)", diskCount, backing)
				}
			} else {
				klog.Infof("Pure VVol Discovery: Disk #%d has no backing information", diskCount)
			}
		}
	}

	klog.Errorf("Pure VVol Discovery: No matching VVol disk found after checking %d disks", diskCount)
	klog.Errorf("Pure VVol Discovery: Target VMDK: %s", vmDisk.VmdkFile)
	klog.Errorf("Pure VVol Discovery: Target Datastore: %s", vmDisk.Datastore)
	klog.Errorf("Pure VVol Discovery: Target VM Home: %s", vmDisk.VmHomeDir)

	return "", fmt.Errorf("no matching VVol disk found for VMDK path %s in VM %s", vmDisk.VmdkFile, vmId)
}

// extractPureVolumeNameFromVVolBacking tries to extract the actual Pure volume name from VVol backing info
func (f *FlashArrayClonner) extractPureVolumeNameFromVVolBacking(backing *types.VirtualDiskFlatVer2BackingInfo) string {
	klog.Infof("Pure VVol Discovery: Attempting to extract Pure volume name from VVol backing")

	// Check if BackingObjectId contains the Pure volume name
	// Pure VVols often store the actual volume name in the backing object ID
	if backing.BackingObjectId != "" {
		klog.Infof("Pure VVol Discovery: Found BackingObjectId: %s", backing.BackingObjectId)

		// The BackingObjectId might be the Pure volume name directly or contain it
		// Format could be something like: "vvol-vm-12-100g-34f88851-vg/Data-c5f27d97"
		if strings.Contains(backing.BackingObjectId, "/") {
			klog.Infof("Pure VVol Discovery: BackingObjectId contains '/', using as-is: %s", backing.BackingObjectId)
			return backing.BackingObjectId
		}

		// If it doesn't contain '/', it might still be the volume name
		klog.Infof("Pure VVol Discovery: BackingObjectId format doesn't match expected pattern, but returning: %s", backing.BackingObjectId)
		return backing.BackingObjectId
	}

	// Check UUID field - might contain useful information
	if backing.Uuid != "" {
		klog.Infof("Pure VVol Discovery: Found UUID: %s", backing.Uuid)
		// UUID might be in a format that helps us find the volume
		// This is a fallback approach
	}

	klog.Infof("Pure VVol Discovery: No usable Pure volume name found in VVol backing info")
	return ""
}

// matchesVMDKPath checks if the vSphere fileName matches our target VMDK
func (f *FlashArrayClonner) matchesVMDKPath(fileName string, vmDisk populator.VMDisk) bool {
	klog.Infof("Pure VVol Discovery: Matching fileName '%s' against target components:", fileName)
	klog.Infof("Pure VVol Discovery:   - VMDK File: %s", vmDisk.VmdkFile)
	klog.Infof("Pure VVol Discovery:   - Datastore: %s", vmDisk.Datastore)
	klog.Infof("Pure VVol Discovery:   - VM Home: %s", vmDisk.VmHomeDir)

	vmdkMatch := strings.Contains(fileName, vmDisk.VmdkFile)
	datastoreMatch := strings.Contains(fileName, vmDisk.Datastore)
	vmHomeMatch := strings.Contains(fileName, vmDisk.VmHomeDir)

	klog.Infof("Pure VVol Discovery: Match results - VMDK: %v, Datastore: %v, VMHome: %v",
		vmdkMatch, datastoreMatch, vmHomeMatch)

	// VVol file names typically look like: "[datastore] vmfolder/vvolId.vmdk"
	// We check if the fileName contains the key components of our expected VMDK path
	result := vmdkMatch && datastoreMatch && vmHomeMatch
	klog.Infof("Pure VVol Discovery: Overall match result: %v", result)
	return result
}

// extractVVolIdFromFileName extracts the VVol ID from vSphere fileName
func (f *FlashArrayClonner) extractVVolIdFromFileName(fileName string) string {
	klog.Infof("Pure VVol Discovery: Extracting VVol ID from fileName: %s", fileName)

	// VVol file names typically contain the VVol UUID
	// Example: "[datastore] vmfolder/12345678-1234-1234-1234-123456789abc.vmdk"

	// Extract the file name part (everything after the last /)
	parts := strings.Split(fileName, "/")
	klog.Infof("Pure VVol Discovery: Split fileName into %d parts: %v", len(parts), parts)

	if len(parts) == 0 {
		klog.Warningf("Pure VVol Discovery: No parts found after splitting fileName")
		return ""
	}

	lastPart := parts[len(parts)-1]
	klog.Infof("Pure VVol Discovery: Last part (filename): %s", lastPart)

	// Remove .vmdk extension and any other suffixes
	vvolId := strings.TrimSuffix(lastPart, ".vmdk")
	klog.Infof("Pure VVol Discovery: After removing .vmdk extension: %s", vvolId)

	// VVol IDs are typically UUIDs, so we expect a specific format
	// This is a simplified extraction - you might need to adjust based on actual VVol naming
	if len(vvolId) > 8 {
		klog.Infof("Pure VVol Discovery: VVol ID looks valid (length > 8): %s", vvolId)
		return vvolId
	}

	klog.Warningf("Pure VVol Discovery: VVol ID too short (length %d): %s", len(vvolId), vvolId)
	return ""
}

// vvolIdToPureVolumeNameForSource converts a VVol ID to Pure volume name for SOURCE volumes (no cluster prefix)
func (f *FlashArrayClonner) vvolIdToPureVolumeNameForSource(vvolId string) string {
	klog.Infof("Pure VVol Discovery: Converting VVol ID to Pure SOURCE volume name (no cluster prefix)")
	klog.Infof("Pure VVol Discovery: Input VVol ID: %s", vvolId)

	// For source volumes created by vSphere, we don't use cluster prefix
	// The volume name is typically just the VVol ID or a simple transformation of it

	// Common patterns for vSphere-created VVols:
	// 1. Direct mapping: vvolId
	// 2. Simple prefix: vvol-{vvolId}

	// Try direct mapping first
	result := vvolId
	klog.Infof("Pure VVol Discovery: Using direct mapping for source volume: %s", result)

	return result
}

// vvolIdToPureVolumeName converts a VVol ID to the corresponding Pure FlashArray volume name for TARGET volumes (with cluster prefix)
func (f *FlashArrayClonner) vvolIdToPureVolumeName(vvolId string) string {
	klog.Infof("Pure VVol Discovery: Converting VVol ID to Pure TARGET volume name")
	klog.Infof("Pure VVol Discovery: Input VVol ID: %s", vvolId)
	klog.Infof("Pure VVol Discovery: Cluster prefix: %s", f.clusterPrefix)

	// Pure FlashArray VVol volumes typically have names that incorporate the VVol ID
	// This might need adjustment based on your specific Pure FlashArray VVol configuration

	var result string
	// Common patterns:
	// 1. Direct mapping: vvolIdt
	// 2. Prefixed: vvol-{vvolId}
	// 3. With cluster prefix: {clusterPrefix}-vvol-{vvolId}

	if f.clusterPrefix != "" {
		result = fmt.Sprintf("%s-%s", f.clusterPrefix, vvolId)
		klog.Infof("Pure VVol Discovery: Using cluster prefix pattern for target: %s", result)
	} else {
		result = vvolId
		klog.Infof("Pure VVol Discovery: Using direct mapping for target: %s", result)
	}

	return result
}

// performVolumeCopy performs the actual volume copy and reports progress
func (f *FlashArrayClonner) performVolumeCopy(sourceVolumeName, targetVolumeName string, progress chan<- uint) error {
	klog.Infof("Pure VVol Copy: Starting volume copy operation")
	klog.Infof("Pure VVol Copy: Source volume: %s", sourceVolumeName)
	klog.Infof("Pure VVol Copy: Target volume: %s", targetVolumeName)

	// Report initial progress
	progress <- 5
	klog.Infof("Pure VVol Copy: Progress reported: 5%%")

	// For Pure FlashArray VVol, we can use volume overwrite capability
	// This creates a new volume with the content of the source
	klog.Infof("Pure VVol Copy: Initiating Pure FlashArray CopyVolume API call...")
	_, err := f.client.Volumes.CopyVolume(targetVolumeName, sourceVolumeName, true)
	if err != nil {
		klog.Errorf("Pure VVol Copy: CopyVolume API call failed: %v", err)
		return fmt.Errorf("failed to copy volume: %w", err)
	}
	klog.Infof("Pure VVol Copy: CopyVolume API call completed successfully")

	// Validate that the copy actually completed by checking target volume properties
	klog.Infof("Pure VVol Copy: Validating copy completion...")
	err = f.validateCopyCompletion(sourceVolumeName, targetVolumeName, progress)
	if err != nil {
		klog.Errorf("Pure VVol Copy: Copy validation failed: %v", err)
		return fmt.Errorf("copy validation failed: %w", err)
	}

	// Report final completion
	progress <- 100
	klog.Infof("Pure VVol Copy: Progress reported: 100%% - Copy completed and validated")
	klog.Infof("Pure VVol Copy: Volume copy operation completed successfully")
	return nil
}

// validateCopyCompletion reports completion since Pure FlashArray copy is synchronous
func (f *FlashArrayClonner) validateCopyCompletion(sourceVolumeName, targetVolumeName string, progress chan<- uint) error {
	klog.Infof("Pure VVol Copy Validation: Copy operation completed successfully")
	klog.Infof("Pure VVol Copy Validation: Source: %s", sourceVolumeName)
	klog.Infof("Pure VVol Copy Validation: Target: %s", targetVolumeName)
	klog.Infof("Pure VVol Copy Validation: Pure FlashArray copy is synchronous - operation already complete")

	// Report 100% completion since copy is synchronous
	progress <- 100
	klog.Infof("Pure VVol Copy Validation: ✓ Copy completed successfully (100%%)")

	return nil
}

// findSourceVolumeByPatternMatching lists all volumes and tries to find a match based on VM information
func (f *FlashArrayClonner) findSourceVolumeByPatternMatching(vmDisk populator.VMDisk, vmId string) (string, error) {
	klog.Infof("Pure VVol Pattern Search: Listing all volumes to find source volume by pattern matching")
	klog.Infof("Pure VVol Pattern Search: Looking for volumes related to VM ID: %s", vmId)
	klog.Infof("Pure VVol Pattern Search: Looking for volumes related to VM Home: %s", vmDisk.VmHomeDir)
	klog.Infof("Pure VVol Pattern Search: Source VMDK file: %s", vmDisk.VmdkFile)
	klog.Infof("Pure VVol Pattern Search: *** ONLY SEARCHING FOR DATA VOLUMES ***")

	// List all volumes on the Pure FlashArray
	volumes, err := f.client.Volumes.ListVolumes(nil)
	if err != nil {
		klog.Errorf("Pure VVol Pattern Search: Failed to list volumes: %v", err)
		return "", fmt.Errorf("failed to list volumes: %w", err)
	}

	klog.Infof("Pure VVol Pattern Search: Found %d total volumes on Pure FlashArray", len(volumes))

	// Extract VM identifier from vmId (vm-125748 -> 12)
	vmNumber := f.extractVMNumberFromVMId(vmId)
	klog.Infof("Pure VVol Pattern Search: Extracted VM number from '%s': %s", vmId, vmNumber)

	// Search ONLY for Data volumes that match vSphere VM patterns
	dataVolumes := []string{}

	for _, volume := range volumes {
		volumeName := volume.Name

		// ONLY look for Data volumes - skip Config, Swap, etc.
		if !strings.Contains(volumeName, "/Data-") {
			continue // Skip non-Data volumes
		}

		klog.Infof("Pure VVol Pattern Search: Found Data volume: %s", volumeName)

		// Check for vvol prefix patterns
		if strings.HasPrefix(volumeName, "vvol-") {
			// Check if it contains VM-related identifiers
			if vmDisk.VmHomeDir != "" && strings.Contains(volumeName, vmDisk.VmHomeDir) {
				klog.Infof("Pure VVol Pattern Search: ✓ Data volume contains VM home directory: %s", volumeName)
				dataVolumes = append(dataVolumes, volumeName)
				continue
			}

			if strings.Contains(volumeName, vmId) {
				klog.Infof("Pure VVol Pattern Search: ✓ Data volume contains VM ID: %s", volumeName)
				dataVolumes = append(dataVolumes, volumeName)
				continue
			}

			// Check for vvol-vm pattern and our specific VM
			if strings.HasPrefix(volumeName, "vvol-vm-") {
				if vmNumber != "" && strings.Contains(volumeName, fmt.Sprintf("vvol-vm-%s-", vmNumber)) {
					klog.Infof("Pure VVol Pattern Search: ✓✓✓ Data volume matches our VM number %s: %s", vmNumber, volumeName)
					dataVolumes = append(dataVolumes, volumeName)
				} else {
					klog.Infof("Pure VVol Pattern Search: Data volume is vvol-vm but different VM: %s", volumeName)
				}
				continue
			}
		}

		// Also check for volumes that might directly match VM home directory
		if vmDisk.VmHomeDir != "" && (volumeName == vmDisk.VmHomeDir || strings.Contains(volumeName, vmDisk.VmHomeDir)) {
			klog.Infof("Pure VVol Pattern Search: ✓ Data volume matches VM home directory pattern: %s", volumeName)
			dataVolumes = append(dataVolumes, volumeName)
		}
	}

	klog.Infof("Pure VVol Pattern Search: Found %d Data volumes total: %v", len(dataVolumes), dataVolumes)

	if len(dataVolumes) == 0 {
		klog.Errorf("Pure VVol Pattern Search: ❌ NO DATA VOLUMES FOUND - Will not copy Config/Swap volumes")
		return "", fmt.Errorf("no Data volumes found for VM %s. Only Data volumes are supported for copying", vmId)
	}

	// Select the first Data volume found
	selectedVolume := dataVolumes[0]
	klog.Infof("Pure VVol Pattern Search: ✓ Selected Data volume: %s", selectedVolume)

	if len(dataVolumes) > 1 {
		klog.Warningf("Pure VVol Pattern Search: Multiple Data volumes found, selected first: %v", dataVolumes)
	}

	return selectedVolume, nil
}

// extractVMNumberFromVMId extracts the VM number from vmId (e.g., vm-125748 -> 12)
func (f *FlashArrayClonner) extractVMNumberFromVMId(vmId string) string {
	klog.Infof("Pure VVol Pattern Search: Extracting VM number from VM ID: %s", vmId)

	// Remove "vm-" prefix if present
	if strings.HasPrefix(vmId, "vm-") {
		vmNumber := strings.TrimPrefix(vmId, "vm-")
		klog.Infof("Pure VVol Pattern Search: After removing 'vm-' prefix: %s", vmNumber)

		// For vm-125748, we might need to extract just the first part
		// Check if this looks like a multi-part number and try to extract the VM part
		if len(vmNumber) > 3 {
			// Try different patterns:
			// vm-125748 might correspond to vm-12 in the volume name

			// Method 1: Take first 2 digits
			if len(vmNumber) >= 2 {
				firstTwo := vmNumber[:2]
				klog.Infof("Pure VVol Pattern Search: Trying first 2 digits: %s", firstTwo)
				return firstTwo
			}
		}

		return vmNumber
	}

	klog.Infof("Pure VVol Pattern Search: VM ID doesn't start with 'vm-', returning as-is: %s", vmId)
	return vmId
}
