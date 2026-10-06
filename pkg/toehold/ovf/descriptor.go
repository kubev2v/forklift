package ovf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/kubev2v/forklift/pkg/toehold/version"
	"github.com/vmware/govmomi/vmdk"
)

// DescriptorOptions configures OVF generation.
type DescriptorOptions struct {
	VMDKPath string
	// VMDKFileName is the href for the disk file in the OVF (for streaming import).
	VMDKFileName string
	Name         string
	Network      string
	CPUs         int32
	MemoryMiB    int32
	// StreamSize is the uploaded VMDK byte size for the OVF File element.
	// When zero, size is read from VMDKPath on disk.
	StreamSize int64
	// DiskCapacity is the virtual disk capacity in bytes.
	// When zero, capacity is inferred from VMDKPath.
	DiskCapacity int64
	// DiskHash fingerprints the base disk content (e.g. toehold version.DiskHash).
	// When set, used as the OVF diskId suffix; otherwise "disk-0".
	DiskHash string
}

type descriptorData struct {
	Name      string
	Network   string
	VMDKName  string
	Size      int64
	Capacity  int64
	DiskID    string
	CPUs      int32
	MemoryMiB int32
}

// vCenter rejects OVF from govmomi's xml.Encoder because the Envelope lacks
// the DMTF/VMware namespace declarations. Match govmomi/vmdk's template shape.
const descriptorTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://schemas.dmtf.org/ovf/envelope/1"
          xmlns:ovf="http://schemas.dmtf.org/ovf/envelope/1"
          xmlns:cim="http://schemas.dmtf.org/wbem/wscim/1/common"
          xmlns:rasd="http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_ResourceAllocationSettingData"
          xmlns:vmw="http://www.vmware.com/schema/ovf"
          xmlns:vssd="http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_VirtualSystemSettingData"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <References>
    <File ovf:href="{{ .VMDKName }}" ovf:id="file1" ovf:size="{{ .Size }}"/>
  </References>
  <DiskSection>
    <Info>Virtual disk information</Info>
    <Disk ovf:capacity="{{ .Capacity }}" ovf:capacityAllocationUnits="byte" ovf:diskId="{{ .DiskID }}" ovf:fileRef="file1" ovf:format="http://www.vmware.com/interfaces/specifications/vmdk.html#streamOptimized" ovf:populatedSize="0"/>
  </DiskSection>
  <NetworkSection>
    <Info>Logical networks</Info>
    <Network ovf:name="{{ .Network }}">
      <Description>{{ .Network }}</Description>
    </Network>
  </NetworkSection>
  <VirtualSystem ovf:id="{{ .Name }}">
    <Info>{{ .Name }}</Info>
    <Name>{{ .Name }}</Name>
    <OperatingSystemSection ovf:id="109" ovf:version="9" vmw:osType="rhel9_64Guest">
      <Info>Guest OS</Info>
      <Description>Red Hat Enterprise Linux 9 (64-bit)</Description>
    </OperatingSystemSection>
    <VirtualHardwareSection>
      <Info>Virtual hardware</Info>
      <System>
        <vssd:ElementName>Virtual Hardware Family</vssd:ElementName>
        <vssd:InstanceID>0</vssd:InstanceID>
        <vssd:VirtualSystemIdentifier>{{ .Name }}</vssd:VirtualSystemIdentifier>
        <vssd:VirtualSystemType>vmx-17</vssd:VirtualSystemType>
      </System>
      <Item>
        <rasd:AllocationUnits>hertz * 10^6</rasd:AllocationUnits>
        <rasd:Description>Number of Virtual CPUs</rasd:Description>
        <rasd:ElementName>{{ .CPUs }} virtual CPU(s)</rasd:ElementName>
        <rasd:InstanceID>1</rasd:InstanceID>
        <rasd:ResourceType>3</rasd:ResourceType>
        <rasd:VirtualQuantity>{{ .CPUs }}</rasd:VirtualQuantity>
      </Item>
      <Item>
        <rasd:AllocationUnits>byte * 2^20</rasd:AllocationUnits>
        <rasd:Description>Memory Size</rasd:Description>
        <rasd:ElementName>{{ .MemoryMiB }} MB of memory</rasd:ElementName>
        <rasd:InstanceID>2</rasd:InstanceID>
        <rasd:ResourceType>4</rasd:ResourceType>
        <rasd:VirtualQuantity>{{ .MemoryMiB }}</rasd:VirtualQuantity>
      </Item>
      <Item>
        <rasd:Address>0</rasd:Address>
        <rasd:Description>SCSI Controller</rasd:Description>
        <rasd:ElementName>SCSI Controller 0</rasd:ElementName>
        <rasd:InstanceID>3</rasd:InstanceID>
        <rasd:ResourceSubType>lsilogic</rasd:ResourceSubType>
        <rasd:ResourceType>6</rasd:ResourceType>
      </Item>
      <Item>
        <rasd:AddressOnParent>0</rasd:AddressOnParent>
        <rasd:ElementName>Hard disk 1</rasd:ElementName>
        <rasd:HostResource>ovf:/disk/{{ .DiskID }}</rasd:HostResource>
        <rasd:InstanceID>4</rasd:InstanceID>
        <rasd:Parent>3</rasd:Parent>
        <rasd:ResourceType>17</rasd:ResourceType>
      </Item>
      <Item>
        <rasd:AddressOnParent>0</rasd:AddressOnParent>
        <rasd:AutomaticAllocation>true</rasd:AutomaticAllocation>
        <rasd:Connection>{{ .Network }}</rasd:Connection>
        <rasd:ElementName>Network adapter 1</rasd:ElementName>
        <rasd:InstanceID>5</rasd:InstanceID>
        <rasd:ResourceSubType>vmxnet3</rasd:ResourceSubType>
        <rasd:ResourceType>10</rasd:ResourceType>
      </Item>
    </VirtualHardwareSection>
  </VirtualSystem>
</Envelope>`

var parsedDescriptorTemplate = template.Must(template.New("toehold-ovf").Parse(descriptorTemplate))

// Descriptor generates a minimal OVF envelope for a stream-optimized VMDK.
func Descriptor(opts DescriptorOptions) (string, error) {
	if opts.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if opts.VMDKPath == "" && opts.DiskCapacity == 0 && opts.StreamSize == 0 {
		return "", fmt.Errorf("vmdk path or stream size/capacity is required")
	}
	if opts.Network == "" {
		opts.Network = "VM Network"
	}
	if opts.CPUs == 0 {
		opts.CPUs = version.DefaultCPU
	}
	if opts.MemoryMiB == 0 {
		opts.MemoryMiB = version.DefaultMemoryMiB
	}

	var size int64
	var err error
	if opts.VMDKPath != "" {
		size, err = fileSize(opts.VMDKPath)
		if err != nil && opts.StreamSize == 0 {
			return "", err
		}
	}
	if opts.StreamSize > 0 {
		size = opts.StreamSize
	}
	capacity := opts.DiskCapacity
	if capacity == 0 && opts.VMDKPath != "" {
		capacity, err = DiskCapacity(opts.VMDKPath)
		if err != nil {
			capacity = size
		}
	}
	if capacity == 0 {
		capacity = size
	}
	if size == 0 {
		return "", fmt.Errorf("stream file size is required (set StreamSize or VMDKPath)")
	}

	vmdkName := opts.VMDKFileName
	if vmdkName == "" && opts.VMDKPath != "" {
		vmdkName = filepath.Base(opts.VMDKPath)
	}
	if vmdkName == "" {
		vmdkName = "disk-0.vmdk"
	}

	data := descriptorData{
		Name:      opts.Name,
		Network:   opts.Network,
		VMDKName:  vmdkName,
		Size:      size,
		Capacity:  capacity,
		DiskID:    diskID(opts),
		CPUs:      opts.CPUs,
		MemoryMiB: opts.MemoryMiB,
	}
	var buf bytes.Buffer
	if err := parsedDescriptorTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func diskID(opts DescriptorOptions) string {
	if opts.DiskHash == "" {
		return "disk-0"
	}
	h := opts.DiskHash
	if len(h) > 8 {
		h = h[:8]
	}
	return "vdisk-" + h
}

// DiskCapacity returns the virtual size of a stream-optimized VMDK.
func DiskCapacity(path string) (int64, error) {
	info, err := vmdk.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("vmdk stat %s: %w", path, err)
	}
	if info.Capacity <= 0 {
		return 0, fmt.Errorf("invalid virtual size for %s", path)
	}
	return int64(info.Capacity), nil
}
