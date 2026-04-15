package vmware

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cli/esx"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"k8s.io/klog/v2"
)

const (
	ProtocolISCSI = "iscsi"
	ProtocolFC    = "fc"
	ProtocolSCSI  = "scsi"
	ProtocolBlock = "block"
)

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/vmware_mock_client.go -package=vmware_mocks . Client
type Client interface {
	GetEsxByVm(ctx context.Context, vmName string) (*object.HostSystem, error)
	RunEsxCommand(ctx context.Context, host *object.HostSystem, command []string) ([]esx.Values, error)
	GetDatastore(ctx context.Context, dc *object.Datacenter, datastore string) (*object.Datastore, error)
	// GetVMDiskBacking returns disk backing information for detecting disk type (VVol, RDM, VMDK)
	GetVMDiskBacking(ctx context.Context, vmId string, vmdkPath string) (*DiskBacking, error)
	GetDatastoreActiveAdapters(ctx context.Context, host *object.HostSystem, datastoreName string) ([]HostAdapter, error)
}

type HostAdapter struct {
	Name string
	// Id is the initiator i.e. iqn.XXX for iSCSI, fc.WWNN:WWPN for FC
	Id string
	// Driver is the driver name (e.g., "scini" for PowerFlex, "bnx2i" for iSCSI, etc.)
	Driver string
}

// DiskBacking contains information about the disk backing type
type DiskBacking struct {
	// VVolId is set if the disk is VVol-backed
	VVolId string
	// IsRDM is true if the disk is a Raw Device Mapping
	IsRDM bool
	// DeviceName is the underlying device name
	DeviceName string
	// LunUuid is the unique LUN identifier (SCSI 83h / NAA). Use this for storage resolution; required for RDM.
	LunUuid string
}

type VSphereClient struct {
	*govmomi.Client
}

const sessionKeepAliveIdle = 5 * time.Minute

func NewClient(vcenterUrl, username, password string) (Client, error) {
	ctx := context.Background()
	u, err := soap.ParseURL(vcenterUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing vCenter URL: %w", err)
	}
	u.User = url.UserPassword(username, password)

	soapClient := soap.NewClient(u, true)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, fmt.Errorf("Failed creating vSphere client: %w", err)
	}

	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, sessionKeepAliveIdle)

	c := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}
	if err = c.Login(ctx, u.User); err != nil {
		return nil, fmt.Errorf("Failed to login to vSphere: %w", err)
	}

	return &VSphereClient{Client: c}, nil
}

// getSciniGuid queries the kernel module system to extract the ioctlIniGuidStr
// parameter from the scini module (used by PowerFlex).
// Returns the GUID string or empty string if not found/error.
func (c *VSphereClient) getSciniGuid(ctx context.Context, host *object.HostSystem) string {
	var hostConfigMgr mo.HostSystem
	if err := host.Properties(ctx, host.Reference(), []string{"configManager.kernelModuleSystem"}, &hostConfigMgr); err != nil {
		klog.V(2).Infof("Failed to get kernel module system: %v", err)
		return ""
	}

	if hostConfigMgr.ConfigManager.KernelModuleSystem == nil {
		klog.V(2).Infof("Kernel module system is not available on this host")
		return ""
	}

	res, err := methods.QueryModules(ctx, c.Client.Client, &types.QueryModules{
		This: *hostConfigMgr.ConfigManager.KernelModuleSystem,
	})
	if err != nil {
		klog.V(2).Infof("Failed to query kernel modules: %v", err)
		return ""
	}

	// Find scini module and parse ioctlIniGuidStr inline
	for _, module := range res.Returnval {
		if module.Name == "scini" {
			// Parse option string for ioctlIniGuidStr parameter
			for _, part := range strings.Fields(module.OptionString) {
				if kv := strings.SplitN(part, "=", 2); len(kv) == 2 {
					if strings.EqualFold(kv[0], "IoctlIniGuidStr") {
						klog.V(1).Infof("Found scini GUID: %s", kv[1])
						return kv[1]
					}
				}
			}
			klog.V(2).Infof("Found scini module but no ioctlIniGuidStr in options: %s", module.OptionString)
			return ""
		}
	}

	klog.V(2).Infof("scini module not found on host")
	return ""
}

func (c *VSphereClient) GetDatastoreActiveAdapters(ctx context.Context, host *object.HostSystem, datastoreName string) ([]HostAdapter, error) {
	// Get scini GUID if the module is present (for PowerFlex)
	sciniGuid := c.getSciniGuid(ctx, host)

	// 1. Find the Datastore and get its underlying device ID (NAA)
	var hostMo mo.HostSystem
	err := host.Properties(ctx, host.Reference(), []string{"datastore"}, &hostMo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch host datastores: %w", err)
	}

	pc := property.DefaultCollector(c.Client.Client)
	var dss []mo.Datastore
	err = pc.Retrieve(ctx, hostMo.Datastore, []string{"name", "info"}, &dss)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve datastore properties: %w", err)
	}

	var deviceName string
	for _, ds := range dss {
		if ds.Name == datastoreName {
			if info, ok := ds.Info.(*types.VmfsDatastoreInfo); ok {
				if info.Vmfs != nil && len(info.Vmfs.Extent) > 0 {
					deviceName = info.Vmfs.Extent[0].DiskName
				}
			}
			break
		}
	}

	if deviceName == "" {
		return nil, fmt.Errorf("could not determine underlying device for datastore %s (likely not VMFS)", datastoreName)
	}

	klog.V(2).Infof("Datastore %s maps to device %s", datastoreName, deviceName)

	// 2. Fetch Host Storage Topology (MultipathInfo and ScsiLun)
	var hostConfig mo.HostSystem
	// We need storageDevice which contains both ScsiLun list and MultipathInfo
	err = host.Properties(ctx, host.Reference(), []string{"config.storageDevice"}, &hostConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch host storage device info: %w", err)
	}

	if hostConfig.Config == nil || hostConfig.Config.StorageDevice == nil {
		return nil, fmt.Errorf("host storage device info is missing")
	}

	storageDevice := hostConfig.Config.StorageDevice

	// 3. Find the ScsiLun key using the canonical name
	var scsiLunKey string
	for _, lun := range storageDevice.ScsiLun {
		if lun.GetScsiLun().CanonicalName == deviceName {
			scsiLunKey = lun.GetScsiLun().Key
			klog.V(2).Infof("Found ScsiLun key %s for device %s", scsiLunKey, deviceName)
			break
		}
	}

	if scsiLunKey == "" {
		// Fallback: Try identifying by ID if CanonicalName didn't match (unlikely for VMFS)
		// Or maybe deviceName isn't the canonical name?
		// For now, let's log and error out, or try a direct ID match in multipath as backup?
		klog.Warningf("Could not find ScsiLun with CanonicalName %s", deviceName)
		// Let's try the direct Multipath match as a fallback, reusing previous logic is risky if it was wrong.
		// Better to fail active detection than return wrong one, but user reported "goes with all adapters", so we want to be precise.
		return nil, fmt.Errorf("scsi lun with canonical name %s not found", deviceName)
	}

	if storageDevice.MultipathInfo == nil {
		return nil, fmt.Errorf("host multipath info is missing")
	}

	// 4. Build HBA map with full adapter information
	hbaByKey := make(map[string]HostAdapter) // Maps HBA Key to HostAdapter

	for _, hba := range storageDevice.HostBusAdapter {
		h := hba.GetHostHostBusAdapter()
		if h == nil {
			continue
		}

		adapter := HostAdapter{
			Name:   h.Device,
			Driver: h.Driver,
		}

		// Extract initiator ID based on HBA type
		switch typedHba := hba.(type) {
		case *types.HostInternetScsiHba:
			adapter.Id = typedHba.IScsiName
			klog.V(1).Infof("iSCSI HBA %s has IQN: %s", h.Device, adapter.Id)
		case *types.HostFibreChannelHba:
			// For FC, use ESX format: fc.WWNN:WWPN
			// Convert int64 to uint64 to handle negative values correctly
			wwnn := uint64(typedHba.NodeWorldWideName)
			wwpn := uint64(typedHba.PortWorldWideName)
			adapter.Id = fmt.Sprintf("fc.%016x:%016x", wwnn, wwpn)
			klog.V(1).Infof("FC HBA %s has initiator ID: %s", h.Device, adapter.Id)
		case *types.HostSerialAttachedHba:
			adapter.Id = typedHba.NodeWorldWideName
			klog.V(1).Infof("SAS HBA %s has Node WWN: %s", h.Device, adapter.Id)
		case *types.HostBlockHba:
			adapter.Id = h.Device
			klog.V(1).Infof("Block HBA %s (driver: %s) using device as ID", h.Device, h.Driver)
		case *types.HostParallelScsiHba:
			adapter.Id = h.Device
			klog.V(1).Infof("Parallel SCSI HBA %s using device as ID", h.Device)
		default:
			adapter.Id = h.Device
			klog.V(1).Infof("Unknown HBA type for %s, using device name as ID", h.Device)
		}

		hbaByKey[h.Key] = adapter
	}

	// 5. Find the Multipath LogicalUnit using the ScsiLun Key
	var logicalUnit *types.HostMultipathInfoLogicalUnit
	for _, lun := range storageDevice.MultipathInfo.Lun {
		if lun.Lun == scsiLunKey {
			l := lun // pin
			logicalUnit = &l
			break
		}
	}

	if logicalUnit == nil {
		return nil, fmt.Errorf("multipath logical unit for device %s (key %s) not found", deviceName, scsiLunKey)
	}

	// 6. Collect adapters from active paths
	activeAdapters := make(map[string]HostAdapter)
	for _, path := range logicalUnit.Path {
		klog.V(5).Infof("Path %s: State=%s, AdapterKey=%s", path.Name, path.State, path.Adapter)
		if !strings.EqualFold(path.State, "active") {
			continue
		}

		if adapter, ok := hbaByKey[path.Adapter]; ok {
			activeAdapters[adapter.Name] = adapter
			klog.V(5).Infof("Found active adapter: %s", adapter.Name)
		} else {
			klog.Warningf("HBA Key %s not found in host bus adapter list", path.Adapter)
		}
	}

	var result []HostAdapter
	for _, adapter := range activeAdapters {
		// For scini driver, override the initiator ID with the GUID from kernel module
		if adapter.Driver == "scini" && sciniGuid != "" {
			adapter.Id = sciniGuid
			klog.V(1).Infof("Using scini GUID for adapter %s: %s", adapter.Name, adapter.Id)
		}

		result = append(result, adapter)
		klog.V(1).Infof("Active adapter %s with initiator ID: %s, driver: %s", adapter.Name, adapter.Id, adapter.Driver)
	}

	// Check if any result has an FC or iSCSI adapter
	hasSANAdapter := false
	for _, a := range result {
		if strings.HasPrefix(a.Id, "iqn.") || strings.HasPrefix(a.Id, "fc.") {
			hasSANAdapter = true
			break
		}
	}

	// Fallback for local datastores: if no SAN (FC/iSCSI) adapters were found
	// among active paths, pick the first FC or iSCSI adapter available on the host.
	if !hasSANAdapter {
		klog.V(1).Infof("No FC/iSCSI adapters found in active paths for datastore %s, falling back to first available SAN adapter", datastoreName)

		var firstFC, firstISCSI *HostAdapter
		for _, adapter := range hbaByKey {
			switch {
			case strings.HasPrefix(adapter.Id, "fc.") && firstFC == nil:
				a := adapter
				firstFC = &a
			case strings.HasPrefix(adapter.Id, "iqn.") && firstISCSI == nil:
				a := adapter
				firstISCSI = &a
			}
		}

		if firstFC != nil {
			klog.V(1).Infof("Falling back to FC adapter %s (ID: %s)", firstFC.Name, firstFC.Id)
			return []HostAdapter{*firstFC}, nil
		}
		if firstISCSI != nil {
			klog.V(1).Infof("Falling back to iSCSI adapter %s (ID: %s)", firstISCSI.Name, firstISCSI.Id)
			return []HostAdapter{*firstISCSI}, nil
		}

		klog.Warningf("No FC or iSCSI adapters found on host for fallback")
		if len(result) == 0 {
			return nil, fmt.Errorf("no active adapters found for datastore %s", datastoreName)
		}
	}

	return result, nil
}

func (c *VSphereClient) RunEsxCommand(ctx context.Context, host *object.HostSystem, command []string) ([]esx.Values, error) {
	executor, err := esx.NewExecutor(ctx, c.Client.Client, host.Reference())
	if err != nil {
		return nil, err
	}

	log := klog.FromContext(ctx).WithName("esxcli")
	commandStr := strings.Join(command, " ")
	log.Info("running esxcli command", "command", commandStr)
	res, err := executor.Run(ctx, command)
	if err != nil {
		log.Error(err, "esxcli command failed", "command", commandStr)
		if fault, ok := err.(*esx.Fault); ok {
			if parsedFault, parseErr := ErrToFault(fault); parseErr == nil {
				log.V(2).Info("ESX CLI fault", "type", parsedFault.Type, "messages", parsedFault.ErrMsgs)
			}
		}
		return nil, err
	}
	for _, valueMap := range res.Values {
		message, _ := valueMap["message"]
		status, statusExists := valueMap["status"]
		log.V(2).Info("esxcli result", "message", message, "status", status)
		if statusExists && strings.Join(status, "") != "0" {
			return nil, fmt.Errorf("Failed to invoke vmkfstools: %v", message)
		}
	}
	return res.Values, nil
}

func (c *VSphereClient) GetEsxByVm(ctx context.Context, vmId string) (*object.HostSystem, error) {
	finder := find.NewFinder(c.Client.Client, true)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed getting datacenters: %w", err)
	}

	var vm *object.VirtualMachine
	for _, dc := range datacenters {
		finder.SetDatacenter(dc)
		result, err := finder.VirtualMachine(ctx, vmId)
		if err != nil {
			if _, ok := err.(*find.NotFoundError); !ok {
				return nil, fmt.Errorf("error searching for VM in Datacenter '%s': %w", dc.Name(), err)
			}
		} else {
			vm = result
			klog.FromContext(ctx).WithName("esxcli").V(2).Info("found VM", "vm", vm.Reference().Value)
			break
		}
	}
	if vm == nil {
		moref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmId}
		vm = object.NewVirtualMachine(c.Client.Client, moref)
	}
	if vm == nil {
		return nil, fmt.Errorf("failed to find VM with ID %s", vmId)
	}

	var vmProps mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"runtime.host"}, &vmProps)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	hostRef := vmProps.Runtime.Host
	host := object.NewHostSystem(c.Client.Client, *hostRef)
	if host == nil {
		return nil, fmt.Errorf("failed to find host: %w", err)
	}
	return host, nil
}

func (c *VSphereClient) GetDatastore(ctx context.Context, dc *object.Datacenter, datastore string) (*object.Datastore, error) {
	finder := find.NewFinder(c.Client.Client, false)
	finder.SetDatacenter(dc)

	ds, err := finder.Datastore(ctx, datastore)
	if err != nil {
		return nil, fmt.Errorf("Failed to find datastore %s: %w", datastore, err)
	}

	return ds, nil
}

// GetVMDiskBacking retrieves disk backing information to determine disk type
func (c *VSphereClient) GetVMDiskBacking(ctx context.Context, vmId string, vmdkPath string) (*DiskBacking, error) {
	log := klog.FromContext(ctx).WithName("esxcli")
	finder := find.NewFinder(c.Client.Client, true)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed getting datacenters: %w", err)
	}

	var vm *object.VirtualMachine
	for _, dc := range datacenters {
		finder.SetDatacenter(dc)
		result, err := finder.VirtualMachine(ctx, vmId)
		if err != nil {
			if _, ok := err.(*find.NotFoundError); !ok {
				return nil, fmt.Errorf("error searching for VM in Datacenter '%s': %w", dc.Name(), err)
			}
		} else {
			vm = result
			break
		}
	}
	if vm == nil {
		moref := types.ManagedObjectReference{Type: "VirtualMachine", Value: vmId}
		vm = object.NewVirtualMachine(c.Client.Client, moref)
	}
	if vm == nil {
		return nil, fmt.Errorf("failed to find VM with ID %s", vmId)
	}

	// Get VM configuration to inspect disk devices
	var vmProps mo.VirtualMachine
	err = vm.Properties(ctx, vm.Reference(), []string{"config.hardware.device"}, &vmProps)
	if err != nil {
		return nil, fmt.Errorf("failed to get VM properties: %w", err)
	}

	// Normalize vmdkPath for comparison (remove brackets and spaces)
	normalizedPath := strings.ToLower(vmdkPath)

	// Find the disk matching the vmdkPath
	for _, device := range vmProps.Config.Hardware.Device {
		disk, ok := device.(*types.VirtualDisk)
		if !ok {
			continue
		}

		// Check different backing types
		switch backing := disk.Backing.(type) {
		case *types.VirtualDiskFlatVer2BackingInfo:
			// Check if this disk matches the requested path
			if !strings.Contains(strings.ToLower(backing.FileName), normalizedPath) &&
				!strings.Contains(normalizedPath, strings.ToLower(backing.FileName)) {
				// Try to match by extracting datastore and path
				if !diskPathMatches(backing.FileName, vmdkPath) {
					continue
				}
			}

			// Check for VVol backing
			if backing.BackingObjectId != "" {
				log.V(2).Info("disk is VVol-backed", "vmdk", vmdkPath, "backing_object_id", backing.BackingObjectId)
				return &DiskBacking{
					VVolId:     backing.BackingObjectId,
					IsRDM:      false,
					DeviceName: backing.FileName,
				}, nil
			}

			// Regular VMDK
			log.V(2).Info("disk is VMDK-backed", "vmdk", vmdkPath)
			return &DiskBacking{
				VVolId:     "",
				IsRDM:      false,
				DeviceName: backing.FileName,
			}, nil

		case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
			// Check if this disk matches
			if !strings.Contains(strings.ToLower(backing.FileName), normalizedPath) &&
				!strings.Contains(normalizedPath, strings.ToLower(backing.FileName)) {
				if !diskPathMatches(backing.FileName, vmdkPath) {
					continue
				}
			}

			log.V(2).Info("disk is RDM-backed", "vmdk", vmdkPath, "device", backing.DeviceName, "lunUuid", backing.LunUuid)
			return &DiskBacking{
				VVolId:     "",
				IsRDM:      true,
				DeviceName: backing.DeviceName,
				LunUuid:    backing.LunUuid,
			}, nil
		}
	}

	// If we couldn't find the disk, return default VMDK type
	log.V(2).Info("disk not found, assuming VMDK type", "vmdk", vmdkPath)
	return &DiskBacking{
		VVolId:     "",
		IsRDM:      false,
		DeviceName: "",
	}, nil
}

// diskPathMatches compares two VMDK paths accounting for different formats
func diskPathMatches(path1, path2 string) bool {
	// Extract datastore and filename from both paths
	// Format: "[datastore] folder/file.vmdk"
	normalize := func(p string) string {
		p = strings.TrimSpace(p)
		p = strings.ToLower(p)
		// Remove brackets from datastore
		p = strings.ReplaceAll(p, "[", "")
		p = strings.ReplaceAll(p, "]", "")
		return p
	}

	return normalize(path1) == normalize(path2)
}

type Obj struct {
	XMLName          xml.Name `xml:"urn:vim25 obj"`
	VersionID        string   `xml:"versionId,attr"`
	Type             string   `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Fault            Fault    `xml:"fault"`
	LocalizedMessage string   `xml:"localizedMessage"`
}

type Fault struct {
	Type    string   `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	ErrMsgs []string `xml:"errMsg"`
}

func ErrToFault(err error) (*Fault, error) {
	f, ok := err.(*esx.Fault)
	if ok {
		var obj Obj
		decoder := xml.NewDecoder(strings.NewReader(f.Detail))
		err := decoder.Decode(&obj)
		if err != nil {
			return nil, fmt.Errorf("failed to decode from xml to fault: %w", err)
		}
		return &obj.Fault, nil
	}
	return nil, fmt.Errorf("error is not of type esx.Fault")
}
