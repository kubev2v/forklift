package vsphere

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	vimtypes "github.com/vmware/govmomi/vim25/types"
)

// Client represents a vSphere client for querying disk information
type Client struct {
	client *govmomi.Client
	logger *logrus.Logger
}

// NewClient creates a new vSphere client
func NewClient(ctx context.Context, vcenterURL, username, password string, insecure bool, logger *logrus.Logger) (*Client, error) {
	// Parse vCenter URL
	u, err := url.Parse(vcenterURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vCenter URL: %w", err)
	}

	// Set credentials
	u.User = url.UserPassword(username, password)

	// Connect to vSphere
	client, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vSphere: %w", err)
	}

	if logger != nil {
		logger.WithField("vcenter", vcenterURL).Debug("Connected to vSphere")
	}

	return &Client{
		client: client,
		logger: logger,
	}, nil
}

// Close closes the vSphere connection
func (c *Client) Close() {
	if c.client != nil {
		_ = c.client.Logout(context.Background())
	}
}

// GetBaseDiskPaths queries vSphere to get the base disk paths by traversing the full backing chain
// Parameters:
//   - vmMoref: VM managed object reference (e.g., "vm-145371")
//
// Returns the base disk paths (without delta disk suffixes)
func (c *Client) GetBaseDiskPaths(ctx context.Context, vmMoref string) ([]string, error) {
	// Create a reference to the VM using the moref
	vmRef := vimtypes.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: vmMoref,
	}

	// Get VM properties
	var vmMo mo.VirtualMachine
	pc := property.DefaultCollector(c.client.Client)
	err := pc.RetrieveOne(ctx, vmRef, []string{"config.hardware.device"}, &vmMo)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties for %s: %w", vmMoref, err)
	}

	var baseDiskPaths []string

	// Iterate through all virtual disks
	for _, device := range vmMo.Config.Hardware.Device {
		if disk, ok := device.(*vimtypes.VirtualDisk); ok {
			if backing, ok := disk.Backing.(*vimtypes.VirtualDiskFlatVer2BackingInfo); ok {
				currentDiskPath := backing.FileName

				// Traverse the full backing chain to find the true base disk
				baseDiskPath := c.traverseBackingChain(backing, currentDiskPath)
				baseDiskPaths = append(baseDiskPaths, baseDiskPath)

				if c.logger != nil {
					c.logger.WithFields(logrus.Fields{
						"current_disk": currentDiskPath,
						"base_disk":    baseDiskPath,
					}).Debug("Resolved base disk path")
				}
			}
		}
	}

	if len(baseDiskPaths) == 0 {
		return nil, fmt.Errorf("no disks found for VM %s", vmMoref)
	}

	return baseDiskPaths, nil
}

// traverseBackingChain traverses the full backing chain to find the base disk
// With multiple snapshots, we may have: vm-000002.vmdk -> vm-000001.vmdk -> vm.vmdk
// We need to traverse all the way to the base disk (the one with no parent)
func (c *Client) traverseBackingChain(backing *vimtypes.VirtualDiskFlatVer2BackingInfo, currentPath string) string {
	if backing.Parent == nil {
		// No parent - this could be the base disk or a disk without snapshots
		// Use the calculation fallback to ensure we get the base disk name
		return GetBaseDiskPath(currentPath)
	}

	// Traverse the full chain
	currentBacking := backing.Parent
	chainDepth := 1

	for currentBacking.Parent != nil {
		currentBacking = currentBacking.Parent
		chainDepth++
	}

	// Now currentBacking points to the true base disk (no more parents)
	baseDiskPath := currentBacking.FileName

	if baseDiskPath == "" {
		// Unexpected case - use fallback calculation
		if c.logger != nil {
			c.logger.WithFields(logrus.Fields{
				"current_disk": currentPath,
				"chain_depth":  chainDepth,
			}).Warn("Backing chain incomplete, using calculated base disk path")
		}
		return GetBaseDiskPath(currentPath)
	}

	if c.logger != nil {
		c.logger.WithFields(logrus.Fields{
			"current_disk": currentPath,
			"base_disk":    baseDiskPath,
			"chain_depth":  chainDepth,
		}).Debug("Traversed backing chain to find base disk")
	}

	return baseDiskPath
}

// GetBaseDiskPath removes the -XXXXXX delta disk suffix to get the base VMDK path
// Example: "[datastore] vm/vm-000002.vmdk" -> "[datastore] vm/vm.vmdk"
// This is a fallback when backing chain is not available
func GetBaseDiskPath(diskPath string) string {
	// Find the last occurrence of .vmdk
	vmdkIndex := len(diskPath) - len(".vmdk")
	if vmdkIndex < 0 || diskPath[vmdkIndex:] != ".vmdk" {
		// Not a .vmdk file, return as-is
		return diskPath
	}

	// Find the part before .vmdk
	prefix := diskPath[:vmdkIndex]

	// Look for -XXXXXX pattern (6 digits) before .vmdk
	// Example: "vm-000002" -> "vm"
	if len(prefix) >= 7 && prefix[len(prefix)-7] == '-' {
		// Check if last 6 characters are digits
		isAllDigits := true
		for i := len(prefix) - 6; i < len(prefix); i++ {
			if prefix[i] < '0' || prefix[i] > '9' {
				isAllDigits = false
				break
			}
		}
		if isAllDigits {
			// Remove -XXXXXX suffix
			return prefix[:len(prefix)-7] + ".vmdk"
		}
	}

	// No delta disk suffix found, return original path
	return diskPath
}

// FindVMByName finds a VM by name and returns its moref
func (c *Client) FindVMByName(ctx context.Context, datacenter, vmName string) (string, error) {
	finder := find.NewFinder(c.client.Client, true)

	// Find datacenter
	dc, err := finder.Datacenter(ctx, datacenter)
	if err != nil {
		return "", fmt.Errorf("failed to find datacenter %s: %w", datacenter, err)
	}

	finder.SetDatacenter(dc)

	// Find VM
	vm, err := finder.VirtualMachine(ctx, vmName)
	if err != nil {
		return "", fmt.Errorf("failed to find VM %s: %w", vmName, err)
	}

	return vm.Reference().Value, nil
}

// SnapshotDiskInfo contains snapshot disk information for inspection
type SnapshotDiskInfo struct {
	VMMoref             string
	SnapshotMoref       string
	ComputeResourcePath string
}

// GetSnapshotDiskInfo gets snapshot disk information needed for inspection
func (c *Client) GetSnapshotDiskInfo(ctx context.Context, vmMoref, snapshotMoref string) (*SnapshotDiskInfo, error) {
	// Create VM object directly from VMMoref
	// VMMoref is globally unique, no need for datacenter lookup
	vmRef := vimtypes.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: vmMoref,
	}
	vm := object.NewVirtualMachine(c.client.Client, vmRef)

	// Get VM properties (we only need runtime.host for compute resource path)
	var vmMo mo.VirtualMachine
	pc := property.DefaultCollector(c.client.Client)
	err := pc.RetrieveOne(ctx, vm.Reference(), []string{"runtime.host"}, &vmMo)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	// No need to search for snapshot - we already have the snapshotMoref

	// Create finder for ObjectReference calls
	finder := find.NewFinder(c.client.Client, true)

	// Get compute resource path (host/cluster) for vpx:// URL
	var computeResourcePath string
	if vmMo.Runtime.Host != nil {
		host, err := finder.ObjectReference(ctx, *vmMo.Runtime.Host)
		if err == nil {
			if hostObj, ok := host.(*object.HostSystem); ok {
				computeResourcePath = hostObj.InventoryPath
				if c.logger != nil {
					c.logger.WithField("compute_resource_path", computeResourcePath).Debug("Got compute resource path from host")
				}
			}
		}

		// If we couldn't get the host path, try to get it from the host's parent (cluster)
		if computeResourcePath == "" {
			var hostMo mo.HostSystem
			err = pc.RetrieveOne(ctx, *vmMo.Runtime.Host, []string{"parent"}, &hostMo)
			if err == nil && hostMo.Parent != nil {
				parentObj, err := finder.ObjectReference(ctx, *hostMo.Parent)
				if err == nil {
					switch obj := parentObj.(type) {
					case *object.ClusterComputeResource:
						computeResourcePath = obj.InventoryPath
					case *object.ComputeResource:
						computeResourcePath = obj.InventoryPath
					}
					if c.logger != nil && computeResourcePath != "" {
						c.logger.WithField("compute_resource_path", computeResourcePath).Debug("Got compute resource path from parent")
					}
				}
			}
		}
	}

	if computeResourcePath == "" {
		return nil, fmt.Errorf("failed to get compute resource path for VM '%s'", vmMoref)
	}

	if c.logger != nil {
		c.logger.WithFields(logrus.Fields{
			"vm_moref":              vmMoref,
			"snapshot_moref":        snapshotMoref,
			"compute_resource_path": computeResourcePath,
		}).Debug("Retrieved snapshot disk info")
	}

	return &SnapshotDiskInfo{
		VMMoref:             vmMoref,
		SnapshotMoref:       snapshotMoref,
		ComputeResourcePath: computeResourcePath,
	}, nil
}
