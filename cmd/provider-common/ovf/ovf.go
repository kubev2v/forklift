package ovf

import (
	"archive/tar"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"strings"
)

const (
	SourceUnknown    = "Unknown"
	SourceVMware     = "VMware"
	SourceVirtualBox = "VirtualBox"
	SourceXen        = "Xen"
	SourceOvirt      = "oVirt"
)

const (
	ExtOVF = ".ovf"
	ExtOVA = ".ova"
)

// ExtractEnvelope from an appliance archive (*.ova file)
func ExtractEnvelope(ovaPath string) (envelope *Envelope, err error) {
	file, err := os.Open(ovaPath)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()
	envelope = &Envelope{}
	reader := tar.NewReader(file)
	for {
		header, rErr := reader.Next()
		if rErr != nil {
			if errors.Is(rErr, io.EOF) {
				err = errors.New("unexpected end of file while looking for .ovf")
			} else {
				err = rErr
			}
			return
		}
		if strings.HasSuffix(strings.ToLower(header.Name), ExtOVF) {
			decoder := xml.NewDecoder(reader)
			err = decoder.Decode(envelope)
			if err != nil {
				return
			}
			break
		}
	}
	return
}

// ReadEnvelope from an *.ovf file.
func ReadEnvelope(ovfPath string) (envelope *Envelope, err error) {
	file, err := os.Open(ovfPath)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()
	envelope = &Envelope{}
	decoder := xml.NewDecoder(file)
	err = decoder.Decode(envelope)
	if err != nil {
		return
	}
	return
}

// GuessSource checks the OVF XML for any markers that might cause import problems later on.
// Not guaranteed to correctly guess the OVA source, but should be good enough
// to filter out some obvious problem cases.
func GuessSource(envelope Envelope) string {
	namespaceMap := map[string]string{
		"http://schemas.citrix.com/ovf/envelope/1": SourceXen,
		"http://www.citrix.com/xenclient/ovf/1":    SourceXen,
		"http://www.virtualbox.org/ovf/machine":    SourceVirtualBox,
		"http://www.ovirt.org/ovf":                 SourceOvirt,
	}

	foundVMware := false

	for _, attribute := range envelope.Attributes {
		if source, present := namespaceMap[attribute.Value]; present {
			return source
		}

		// Other products may contain a VMware namespace, use it as a default if present
		// and if no others are found.
		if strings.Contains(attribute.Value, "http://www.vmware.com/schema/ovf") {
			foundVMware = true
		}
	}

	if foundVMware {
		return SourceVMware
	}

	return SourceUnknown
}

// xml struct
type Item struct {
	AllocationUnits string          `xml:"AllocationUnits,omitempty"`
	Description     string          `xml:"Description,omitempty"`
	ElementName     string          `xml:"ElementName"`
	InstanceID      string          `xml:"InstanceID"`
	ResourceType    int             `xml:"ResourceType"`
	VirtualQuantity int32           `xml:"VirtualQuantity"`
	Address         string          `xml:"Address,omitempty"`
	ResourceSubType string          `xml:"ResourceSubType,omitempty"`
	Parent          string          `xml:"Parent,omitempty"`
	HostResource    string          `xml:"HostResource,omitempty"`
	Connection      string          `xml:"Connection,omitempty"`
	Configs         []VirtualConfig `xml:"Config"`
	CoresPerSocket  string          `xml:"CoresPerSocket"`
}

type VirtualConfig struct {
	XMLName  xml.Name `xml:"http://www.vmware.com/schema/ovf Config"`
	Required string   `xml:"required,attr"`
	Key      string   `xml:"key,attr"`
	Value    string   `xml:"value,attr"`
}

type ExtraVirtualConfig struct {
	XMLName  xml.Name `xml:"http://www.vmware.com/schema/ovf ExtraConfig"`
	Required string   `xml:"required,attr"`
	Key      string   `xml:"key,attr"`
	Value    string   `xml:"value,attr"`
}

type VirtualHardwareSection struct {
	Info        string               `xml:"Info"`
	Items       []Item               `xml:"Item"`
	Configs     []VirtualConfig      `xml:"Config"`
	ExtraConfig []ExtraVirtualConfig `xml:"ExtraConfig"`
}

type References struct {
	File []struct {
		Href string `xml:"href,attr"`
	} `xml:"File"`
}

type DiskSection struct {
	XMLName xml.Name `xml:"DiskSection"`
	Info    string   `xml:"Info"`
	Disks   []Disk   `xml:"Disk"`
}

type Disk struct {
	XMLName                 xml.Name `xml:"Disk"`
	Capacity                int64    `xml:"capacity,attr"`
	CapacityAllocationUnits string   `xml:"capacityAllocationUnits,attr"`
	DiskId                  string   `xml:"diskId,attr"`
	FileRef                 string   `xml:"fileRef,attr"`
	Format                  string   `xml:"format,attr"`
	PopulatedSize           int64    `xml:"populatedSize,attr"`
}

type NetworkSection struct {
	XMLName  xml.Name  `xml:"NetworkSection"`
	Info     string    `xml:"Info"`
	Networks []Network `xml:"Network"`
}

type Network struct {
	XMLName     xml.Name `xml:"Network"`
	Name        string   `xml:"name,attr"`
	Description string   `xml:"Description"`
}

type VirtualSystem struct {
	ID                     string `xml:"id,attr"`
	Name                   string `xml:"Name"`
	OperatingSystemSection struct {
		Info        string `xml:"Info"`
		Description string `xml:"Description"`
		OsType      string `xml:"osType,attr"`
	} `xml:"OperatingSystemSection"`
	HardwareSection VirtualHardwareSection `xml:"VirtualHardwareSection"`
}

type Envelope struct {
	XMLName        xml.Name        `xml:"Envelope"`
	Attributes     []xml.Attr      `xml:",any,attr"`
	VirtualSystem  []VirtualSystem `xml:"VirtualSystem"`
	DiskSection    DiskSection     `xml:"DiskSection"`
	NetworkSection NetworkSection  `xml:"NetworkSection"`
	References     References      `xml:"References"`
}
