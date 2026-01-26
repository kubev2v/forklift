package hypervovf

import "encoding/xml"

const xmlHeader = "<?xml version='1.0' encoding='UTF-8'?>\n"

type Envelope struct {
	XMLName        xml.Name       `xml:"Envelope"`
	Xmlns          string         `xml:"xmlns,attr"`
	Cim            string         `xml:"xmlns:cim,attr"`
	Ovf            string         `xml:"xmlns:ovf,attr"`
	Rasd           string         `xml:"xmlns:rasd,attr"`
	Vmw            string         `xml:"xmlns:vmw,attr"`
	Vssd           string         `xml:"xmlns:vssd,attr"`
	Xsi            string         `xml:"xmlns:xsi,attr"`
	References     References     `xml:"References"`
	DiskSection    DiskSection    `xml:"DiskSection"`
	NetworkSection NetworkSection `xml:"NetworkSection"`
	VirtualSystem  VirtualSystem  `xml:"VirtualSystem"`
}

type References struct {
	Files []File `xml:"File"`
}

type File struct {
	ID   string `xml:"ovf:id,attr"`
	Href string `xml:"ovf:href,attr"`
	Size int64  `xml:"ovf:size,attr"`
}

type DiskSection struct {
	Info  string `xml:"Info"`
	Disks []Disk `xml:"Disk"`
}

type Disk struct {
	Capacity                int64  `xml:"ovf:capacity,attr"`
	CapacityAllocationUnits string `xml:"ovf:capacityAllocationUnits,attr"`
	DiskID                  string `xml:"ovf:diskId,attr"`
	FileRef                 string `xml:"ovf:fileRef,attr"`
	Format                  string `xml:"ovf:format,attr"`
}

type NetworkSection struct {
	Info     string    `xml:"Info"`
	Networks []Network `xml:"Network"`
}

type Network struct {
	Name        string `xml:"ovf:name,attr"`
	Description string `xml:"Description"`
}

type VirtualSystem struct {
	ID              string                 `xml:"ovf:id,attr"`
	Info            string                 `xml:"Info"`
	Name            string                 `xml:"Name"`
	OperatingSystem OperatingSystemSection `xml:"OperatingSystemSection"`
	VirtualHardware VirtualHardwareSection `xml:"VirtualHardwareSection"`
}

type OperatingSystemSection struct {
	ID          int    `xml:"ovf:id,attr"`
	OsType      string `xml:"vmw:osType,attr"`
	Info        string `xml:"Info"`
	Description string `xml:"Description"`
}

type VirtualHardwareSection struct {
	Info   string `xml:"Info"`
	System System `xml:"System"`
	Items  []Item `xml:"Item"`
}

type System struct {
	ElementName             string `xml:"vssd:ElementName"`
	InstanceID              int    `xml:"vssd:InstanceID"`
	VirtualSystemIdentifier string `xml:"vssd:VirtualSystemIdentifier"`
	VirtualSystemType       string `xml:"vssd:VirtualSystemType"`
}

type Item struct {
	AllocationUnits     string       `xml:"rasd:AllocationUnits,omitempty"`
	Description         string       `xml:"rasd:Description"`
	ElementName         string       `xml:"rasd:ElementName"`
	InstanceID          string       `xml:"rasd:InstanceID"`
	ResourceType        ResourceType `xml:"rasd:ResourceType"`
	VirtualQuantity     int64        `xml:"rasd:VirtualQuantity,omitempty"`
	Address             string       `xml:"rasd:Address,omitempty"`
	AddressOnParent     string       `xml:"rasd:AddressOnParent,omitempty"`
	Parent              string       `xml:"rasd:Parent,omitempty"`
	HostResource        string       `xml:"rasd:HostResource,omitempty"`
	Required            *bool        `xml:"ovf:required,attr,omitempty"`
	AutomaticAllocation *bool        `xml:"rasd:AutomaticAllocation,omitempty"`
	Connection          string       `xml:"rasd:Connection,omitempty"`
	ResourceSubType     string       `xml:"rasd:ResourceSubType,omitempty"`
}
