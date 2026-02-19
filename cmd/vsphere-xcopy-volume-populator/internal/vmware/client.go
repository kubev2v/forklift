package vmware

import (
	"context"
	"encoding/xml"
	"net/url"
	"strings"

	"fmt"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cli/esx"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"k8s.io/klog/v2"
)

//go:generate go run go.uber.org/mock/mockgen -destination=mocks/vmware_mock_client.go -package=vmware_mocks . Client
type Client interface {
	GetEsxByVm(ctx context.Context, vmName string) (*object.HostSystem, error)
	RunEsxCommand(ctx context.Context, host *object.HostSystem, command []string) ([]esx.Values, error)
	GetDatastore(ctx context.Context, dc *object.Datacenter, datastore string) (*object.Datastore, error)
	// GetVMDiskBacking returns disk backing information for detecting disk type (VVol, RDM, VMDK)
	GetVMDiskBacking(ctx context.Context, vmId string, vmdkPath string) (*DiskBacking, error)
}

// DiskBacking contains information about the disk backing type
type DiskBacking struct {
	// VVolId is set if the disk is VVol-backed
	VVolId string
	// IsRDM is true if the disk is a Raw Device Mapping
	IsRDM bool
	// DeviceName is the underlying device name
	DeviceName string
}

type VSphereClient struct {
	*govmomi.Client
}

func NewClient(vcenterUrl, username, password string) (Client, error) {
	ctx := context.Background()
	u, err := soap.ParseURL(vcenterUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing vCenter URL: %w", err)
	}
	u.User = url.UserPassword(username, password)

	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, fmt.Errorf("Failed creating vSphere client: %w", err)
	}

	return &VSphereClient{Client: c}, nil
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

			log.V(2).Info("disk is RDM-backed", "vmdk", vmdkPath, "device", backing.DeviceName)
			return &DiskBacking{
				VVolId:     "",
				IsRDM:      true,
				DeviceName: backing.DeviceName,
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
