package vsphere

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/kubev2v/vm-migration-detective/internal/tlsconfig"
	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	vimtypes "github.com/vmware/govmomi/vim25/types"
)

// Client represents a vSphere client for querying disk information
type Client struct {
	client              *govmomi.Client
	logger              *logrus.Logger
	snapshotDeviceCache snapshotDeviceCache
}

type snapshotDeviceCache struct {
	mu      sync.Mutex
	devices map[string][]vimtypes.BaseVirtualDevice
}

func (cache *snapshotDeviceCache) get(
	ctx context.Context,
	snapshotMoref string,
	load func(context.Context, string) ([]vimtypes.BaseVirtualDevice, error),
) ([]vimtypes.BaseVirtualDevice, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if devices, ok := cache.devices[snapshotMoref]; ok {
		return devices, nil
	}

	devices, err := load(ctx, snapshotMoref)
	if err != nil {
		return nil, err
	}
	if cache.devices == nil {
		cache.devices = make(map[string][]vimtypes.BaseVirtualDevice)
	}
	cache.devices[snapshotMoref] = devices
	return devices, nil
}

// NewClient creates a new vSphere client with TLS configuration
func NewClient(ctx context.Context, vcenterURL, username, password string, tlsConfig *tlsconfig.Config, logger *logrus.Logger) (*Client, error) {
	// Parse vCenter URL
	u, err := url.Parse(vcenterURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vCenter URL: %w", err)
	}

	// Set credentials
	u.User = url.UserPassword(username, password)

	// Validate TLS config
	if tlsConfig == nil {
		return nil, fmt.Errorf("TLS configuration is required")
	}

	// Log deprecation warning if using deprecated default
	if tlsConfig.IsDeprecatedDefault && logger != nil {
		logger.Warn("Connecting to vCenter with INSECURE mode (deprecated default)")
	}

	// Configure the SOAP transport before it performs its initial service-content
	// request or authenticates. govmomi.NewClient only accepts an insecure bool,
	// so using it directly would discard CA and thumbprint configuration.
	soapClient := soap.NewClient(u, tlsConfig.ForGovmomi())
	if tlsConfig.RootCAPath != "" {
		if err := soapClient.SetRootCAs(tlsConfig.RootCAPath); err != nil {
			return nil, fmt.Errorf("failed to configure vSphere CA certificates: %w", err)
		}
	}
	if tlsConfig.Thumbprint != "" {
		soapClient.SetThumbprint(u.Host, tlsConfig.Thumbprint)
	}

	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to vSphere: %w", err)
	}
	client := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}
	if err := client.Login(ctx, u.User); err != nil {
		return nil, fmt.Errorf("failed to authenticate to vSphere: %w", err)
	}

	if logger != nil {
		tlsMode := "secure"
		if tlsConfig.ForGovmomi() {
			tlsMode = "insecure"
		}
		logger.WithFields(logrus.Fields{
			"vcenter":  vcenterURL,
			"tls_mode": tlsMode,
		}).Debug("Connected to vSphere")
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

// GetSnapshotDiskFilePath finds the snapshot-level backing file for a base disk.
// NFC matches file= to the snapshot's top-level backing filename, while VDDK
// resolves the snapshot chain itself and continues to use the base disk path.
func (c *Client) GetSnapshotDiskFilePath(ctx context.Context, snapshotMoref, baseDiskPath string) (string, error) {
	devices, err := c.snapshotDeviceCache.get(ctx, snapshotMoref, c.retrieveSnapshotDevices)
	if err != nil {
		return "", err
	}

	diskFile, err := resolveSnapshotDiskFilePath(devices, baseDiskPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve disk path %q in snapshot %s: %w", baseDiskPath, snapshotMoref, err)
	}
	return diskFile, nil
}

func (c *Client) retrieveSnapshotDevices(ctx context.Context, snapshotMoref string) ([]vimtypes.BaseVirtualDevice, error) {
	snapshotRef := vimtypes.ManagedObjectReference{
		Type:  "VirtualMachineSnapshot",
		Value: snapshotMoref,
	}

	var snapshot mo.VirtualMachineSnapshot
	pc := property.DefaultCollector(c.client.Client)
	if err := pc.RetrieveOne(ctx, snapshotRef, []string{"config.hardware.device"}, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to retrieve snapshot %s config: %w", snapshotMoref, err)
	}
	return snapshot.Config.Hardware.Device, nil
}

func resolveSnapshotDiskFilePath(devices []vimtypes.BaseVirtualDevice, baseDiskPath string) (string, error) {
	normalizedBasePath := normalizeDiskPath(baseDiskPath)
	for _, device := range devices {
		disk, ok := device.(*vimtypes.VirtualDisk)
		if !ok {
			continue
		}

		backing, ok := disk.Backing.(*vimtypes.VirtualDiskFlatVer2BackingInfo)
		if !ok {
			continue
		}

		topBackingPath := backing.FileName
		for current := backing; current != nil; current = current.Parent {
			if normalizeDiskPath(current.FileName) != normalizedBasePath {
				continue
			}
			if strings.TrimSpace(topBackingPath) == "" {
				return "", fmt.Errorf("top-level snapshot backing path is empty")
			}
			return topBackingPath, nil
		}
	}

	return "", fmt.Errorf("disk %q not found in snapshot backing chain", baseDiskPath)
}

func normalizeDiskPath(path string) string {
	path = strings.TrimSpace(path)
	before, after, ok := strings.Cut(path, "]")
	if !ok {
		return path
	}
	return before + "] " + strings.TrimSpace(after)
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
	VMName              string // VM display name, required as the libvirt domain name for virt-v2v-inspector
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

	// Get VM properties: runtime.host for compute resource path, name for the
	// libvirt domain name virt-v2v-inspector needs (it cannot look up VMs by moref)
	var vmMo mo.VirtualMachine
	pc := property.DefaultCollector(c.client.Client)
	err := pc.RetrieveOne(ctx, vm.Reference(), []string{"runtime.host", "name"}, &vmMo)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	// No need to search for snapshot - we already have the snapshotMoref

	// Create finder for ObjectReference calls
	finder := find.NewFinder(c.client.Client, true)

	// Get compute resource path (host/cluster) for vpx:// URL. vSphere's
	// InventoryPath includes the structural "host" folder, and standalone
	// hosts appear as a ComputeResource followed by a HostSystem. libvirt's vpx
	// URI omits that folder; standalone hosts need one path component, while
	// clustered hosts need both the cluster and host components.
	var computeResourcePath string
	if vmMo.Runtime.Host != nil {
		var hostMo mo.HostSystem
		err = pc.RetrieveOne(ctx, *vmMo.Runtime.Host, []string{"parent", "name"}, &hostMo)
		if err != nil {
			return nil, fmt.Errorf("failed to get host compute resource: %w", err)
		}
		if hostMo.Parent == nil {
			return nil, fmt.Errorf("host for VM '%s' has no compute resource parent", vmMoref)
		}

		parentObj, err := finder.ObjectReference(ctx, *hostMo.Parent)
		if err != nil {
			return nil, fmt.Errorf("failed to find host compute resource: %w", err)
		}

		isCluster := false
		var resourceInventoryPath string
		switch obj := parentObj.(type) {
		case *object.ClusterComputeResource:
			isCluster = true
			resourceInventoryPath = obj.InventoryPath
		case *object.ComputeResource:
			resourceInventoryPath = obj.InventoryPath
		default:
			return nil, fmt.Errorf("unexpected host compute resource type %T", parentObj)
		}

		computeResourcePath, err = vpxPathFromComputeResource(resourceInventoryPath, hostMo.Name, isCluster)
		if err != nil {
			return nil, err
		}
		if c.logger != nil {
			c.logger.WithField("compute_resource_path", computeResourcePath).Debug("Got libvirt vpx path from host compute resource")
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
		VMName:              vmMo.Name,
		SnapshotMoref:       snapshotMoref,
		ComputeResourcePath: computeResourcePath,
	}, nil
}

func vpxPathFromComputeResource(inventoryPath, hostName string, isCluster bool) (string, error) {
	const hostFolder = "/host/"
	index := strings.Index(inventoryPath, hostFolder)
	if index < 0 {
		return "", fmt.Errorf("compute resource inventory path %q does not contain the vSphere host folder", inventoryPath)
	}

	// Remove the structural host folder while preserving the resource path.
	// For a standalone host, the ComputeResource itself names the host. For a
	// cluster, append the ESXi host below the cluster path.
	path := inventoryPath[:index] + inventoryPath[index+len("/host"):]
	if isCluster {
		if hostName == "" {
			return "", fmt.Errorf("host name is required to build a clustered vpx path")
		}
		path = strings.TrimSuffix(path, "/") + "/" + hostName
	}
	return path, nil
}
