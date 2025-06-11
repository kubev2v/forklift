package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/kubev2v/forklift/pkg/lib/gob"

	"github.com/google/uuid"
)

const (
	invalidRequestMethodMsg = "Invalid request method"
	errorProcessingOvfMsg   = "Error processing OVF file"
	OvaExt                  = ".ova"
	OvfExt                  = ".ovf"
)

// xml struct
type Item struct {
	AllocationUnits string          `xml:"AllocationUnits,omitempty"`
	Description     string          `xml:"Description,omitempty"`
	ElementName     string          `xml:"ElementName"`
	InstanceID      string          `xml:"InstanceID"`
	ResourceType    string          `xml:"ResourceType"`
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

// vm struct
type VM struct {
	Name                  string
	OvaPath               string
	OvaSource             string
	OsType                string
	RevisionValidated     int64
	PolicyVersion         int
	UUID                  string
	Firmware              string
	CpuAffinity           []int32
	CpuHotAddEnabled      bool
	CpuHotRemoveEnabled   bool
	MemoryHotAddEnabled   bool
	FaultToleranceEnabled bool
	CpuCount              int32
	CoresPerSocket        int32
	MemoryMB              int32
	MemoryUnits           string
	CpuUnits              string
	BalloonedMemory       int32
	IpAddress             string
	NumaNodeAffinity      []string
	StorageUsed           int64
	ChangeTrackingEnabled bool
	Devices               []Device
	NICs                  []NIC
	Disks                 []VmDisk
	Networks              []VmNetwork
}

// Virtual Disk.
type VmDisk struct {
	ID                      string
	Name                    string
	FilePath                string
	Capacity                int64
	CapacityAllocationUnits string
	DiskId                  string
	FileRef                 string
	Format                  string
	PopulatedSize           int64
}

// Virtual Device.
type Device struct {
	Kind string `json:"kind"`
}

type Conf struct {
	//nolint:unused
	key string

	Value string
}

// Virtual ethernet card.
type NIC struct {
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	Network string
	Config  []Conf
}

type VmNetwork struct {
	Name        string
	Description string
	ID          string
}

var vmIDMap *UUIDMap
var diskIDMap *UUIDMap
var networkIDMap *UUIDMap

func main() {

	vmIDMap = NewUUIDMap()
	diskIDMap = NewUUIDMap()
	networkIDMap = NewUUIDMap()

	http.HandleFunc("/vms", vmHandler)
	http.HandleFunc("/disks", diskHandler)
	http.HandleFunc("/networks", networkHandler)
	http.HandleFunc("/watch", watchdHandler)
	http.HandleFunc("/test_connection", connHandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func connHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode("Test connection successful")
	if err != nil {
		fmt.Println("Error encoding json: ", err)
	}

	fmt.Println("Test connection handeler was called")
}

func vmHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, invalidRequestMethodMsg, http.StatusMethodNotAllowed)
		return
	}
	vmXML, ovaPath := scanOVAsOnNFS()
	vmStruct, err := convertToVmStruct(vmXML, ovaPath)
	if err != nil {
		fmt.Println(errorProcessingOvfMsg, err)
	}

	setContentTypeToJson(w)
	err = json.NewEncoder(w).Encode(vmStruct)
	if err != nil {
		fmt.Println("Error encoding json: ", err)
		http.Error(w, "Error encoding json", http.StatusInternalServerError)
		return
	}

	fmt.Println("VM handler was called")
}

func diskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, invalidRequestMethodMsg, http.StatusMethodNotAllowed)
		return
	}
	xmlStruct, ovaPath := scanOVAsOnNFS()
	diskStruct, err := convertToDiskStruct(xmlStruct, ovaPath)
	if err != nil {
		fmt.Println(errorProcessingOvfMsg, err)
		return
	}

	setContentTypeToJson(w)
	err = json.NewEncoder(w).Encode(diskStruct)
	if err != nil {
		fmt.Println("Error encoding json: ", err)
		http.Error(w, "Error encoding json", http.StatusInternalServerError)
		return
	}

	fmt.Println("Disk handler was called")
}

func networkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, invalidRequestMethodMsg, http.StatusMethodNotAllowed)
		return
	}
	xmlStruct, _ := scanOVAsOnNFS()
	netStruct, err := convertToNetworkStruct(xmlStruct)
	if err != nil {
		fmt.Println(errorProcessingOvfMsg, err)
		return
	}

	setContentTypeToJson(w)
	err = json.NewEncoder(w).Encode(netStruct)
	if err != nil {
		fmt.Println("Error encoding json: ", err)
		http.Error(w, "Error encoding json", http.StatusInternalServerError)
		return
	}

	fmt.Println("Network handler was called")
}

func setContentTypeToJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func watchdHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(w, "This is the watch page!")
	//TODO add watch
}

func scanOVAsOnNFS() (envelopes []Envelope, ovaPaths []string) {
	ovaFiles, ovfFiles, err := findOVAFiles("/ova")
	if err != nil {
		fmt.Println("Error finding OVA anf OVF files:", err)
		return
	}

	var filesPath []string

	for _, ovaFile := range ovaFiles {
		fmt.Println("Processing OVA file:", ovaFile)

		xmlStruct, err := readOVFFromOVA(ovaFile)
		if err != nil {
			fmt.Println("Error processing OVF from OVA:", err)
			continue
		}
		envelopes = append(envelopes, *xmlStruct)
		filesPath = append(filesPath, ovaFile)
	}

	for _, ovfFile := range ovfFiles {
		fmt.Println("Processing OVF file:", ovfFile)

		xmlStruct, err := readOVF(ovfFile)
		if err != nil {
			fmt.Println("Error processing OVF:", err)
			continue
		}
		envelopes = append(envelopes, *xmlStruct)
		filesPath = append(filesPath, ovfFile)

	}
	return envelopes, filesPath
}

func findOVAFiles(directory string) (ovaFiles []string, ovfFiles []string, err error) {
	var ovaMaxDepth = 2

	err = filepath.WalkDir(directory, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relativePath, _ := filepath.Rel(directory, path)
		depth := len(strings.Split(relativePath, string(filepath.Separator)))

		switch {
		case (depth <= ovaMaxDepth) && isOva(info.Name()):
			ovaFiles = append(ovaFiles, path)

		case (depth <= ovaMaxDepth+1) && isOvf(info.Name()):
			ovfFiles = append(ovfFiles, path)
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error scanning OVA and OVF files:  ", err)
		return nil, nil, err
	}
	return
}

func isOva(filename string) bool {
	return hasSuffixIgnoreCase(filename, OvaExt)
}

func isOvf(filename string) bool {
	return hasSuffixIgnoreCase(filename, OvfExt)
}

// Checks if the given file has the desired extension
func hasSuffixIgnoreCase(fileName, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(fileName), strings.ToLower(suffix))
}

func readOVFFromOVA(ovaFile string) (*Envelope, error) {
	var envelope Envelope
	file, err := os.Open(ovaFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := tar.NewReader(file)
	for {
		hdr, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(hdr.Name, ".ovf") {
			decoder := xml.NewDecoder(reader)
			err = decoder.Decode(&envelope)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return &envelope, nil
}

func readOVF(ovfFile string) (*Envelope, error) {
	var envelope Envelope

	xmlFile, err := os.Open(ovfFile)
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)

	err = decoder.Decode(&envelope)
	if err != nil {
		return &envelope, err
	}
	return &envelope, nil
}

const (
	Unknown    = "Unknown"
	VMware     = "VMware"
	VirtualBox = "VirtualBox"
	Xen        = "Xen"
	Ovirt      = "oVirt"
)

// Check the OVF XML for any markers that might cause import problems later on.
// Not guaranteed to correctly guess the OVA source, but should be good enough
// to filter out some obvious problem cases.
func guessOvaSource(envelope Envelope) string {
	namespaceMap := map[string]string{
		"http://schemas.citrix.com/ovf/envelope/1": Xen,
		"http://www.citrix.com/xenclient/ovf/1":    Xen,
		"http://www.virtualbox.org/ovf/machine":    VirtualBox,
		"http://www.ovirt.org/ovf":                 Ovirt,
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
		return VMware
	}

	return Unknown
}

func convertToVmStruct(envelope []Envelope, ovaPath []string) ([]VM, error) {
	var vms []VM

	for i := 0; i < len(envelope); i++ {
		vmXml := envelope[i]
		for _, virtualSystem := range vmXml.VirtualSystem {

			// Initialize a new VM
			newVM := VM{
				OvaPath:   ovaPath[i],
				OvaSource: guessOvaSource(vmXml),
				Name:      virtualSystem.Name,
				OsType:    virtualSystem.OperatingSystemSection.OsType,
			}

			for _, item := range virtualSystem.HardwareSection.Items {
				if strings.Contains(item.ElementName, "Network adapter") {
					newVM.NICs = append(newVM.NICs, NIC{
						Name:    item.ElementName,
						MAC:     item.Address,
						Network: item.Connection,
					})
					//for _conf := range item.
				} else if strings.Contains(item.Description, "Number of Virtual CPUs") {
					newVM.CpuCount = item.VirtualQuantity
					newVM.CpuUnits = item.AllocationUnits
					if item.CoresPerSocket != "" {
						num, err := strconv.ParseInt(item.CoresPerSocket, 10, 32)
						if err != nil {
							newVM.CoresPerSocket = 1
						} else {
							newVM.CoresPerSocket = int32(num)
						}
					}
				} else if strings.Contains(item.Description, "Memory Size") {
					newVM.MemoryMB = item.VirtualQuantity
					newVM.MemoryUnits = item.AllocationUnits

				} else {
					var itemKind string
					if len(item.ElementName) > 0 {
						// if the `ElementName` element has a name such as "Hard Disk 1", strip off the
						// number suffix to try to get a more generic name for the device type
						itemKind = strings.TrimRightFunc(item.ElementName, func(r rune) bool {
							return unicode.IsDigit(r) || unicode.IsSpace(r)
						})
					} else {
						// Some .ova files do not include an `ElementName` element for each device. Fall
						// back to using the `Description` element
						itemKind = item.Description
					}
					if len(itemKind) == 0 {
						itemKind = "Unknown"
					}
					newVM.Devices = append(newVM.Devices, Device{
						Kind: itemKind,
					})
				}

			}

			for j, disk := range vmXml.DiskSection.Disks {
				name := envelope[i].References.File[j].Href
				newVM.Disks = append(newVM.Disks, VmDisk{
					FilePath:                getDiskPath(ovaPath[i]),
					Capacity:                disk.Capacity,
					CapacityAllocationUnits: disk.CapacityAllocationUnits,
					DiskId:                  disk.DiskId,
					FileRef:                 disk.FileRef,
					Format:                  disk.Format,
					PopulatedSize:           disk.PopulatedSize,
					Name:                    name,
				})
				newVM.Disks[j].ID = diskIDMap.GetUUID(newVM.Disks[j], ovaPath[i]+"/"+name)

			}

			for _, network := range vmXml.NetworkSection.Networks {
				newVM.Networks = append(newVM.Networks, VmNetwork{
					Name:        network.Name,
					Description: network.Description,
					ID:          networkIDMap.GetUUID(network.Name, network.Name),
				})
			}

			applyConfiguration(&newVM, virtualSystem.HardwareSection.Configs)
			applyExtraConfiguration(&newVM, virtualSystem.HardwareSection.ExtraConfig)

			var id string
			if isValidUUID(virtualSystem.ID) {
				id = virtualSystem.ID
			} else {
				id = vmIDMap.GetUUID(newVM, ovaPath[i])
			}
			newVM.UUID = id

			vms = append(vms, newVM)
		}
	}
	return vms, nil
}

func applyConfiguration(vm *VM, configs []VirtualConfig) {
	for _, config := range configs {
		apply(vm, config.Key, config.Value)
	}
}

func applyExtraConfiguration(vm *VM, configs []ExtraVirtualConfig) {
	for _, config := range configs {
		apply(vm, config.Key, config.Value)
	}
}

func apply(vm *VM, key, value string) {
	switch key {
	case "firmware":
		vm.Firmware = value
	case "memoryHotAddEnabled":
		vm.MemoryHotAddEnabled, _ = strconv.ParseBool(value)
	case "cpuHotAddEnabled":
		vm.CpuHotAddEnabled, _ = strconv.ParseBool(value)
	case "cpuHotRemoveEnabled":
		vm.CpuHotRemoveEnabled, _ = strconv.ParseBool(value)
	}
}

func convertToNetworkStruct(envelope []Envelope) ([]VmNetwork, error) {
	var networks []VmNetwork
	for _, ova := range envelope {
		for _, network := range ova.NetworkSection.Networks {
			newNetwork := VmNetwork{
				Name:        network.Name,
				Description: network.Description,
				ID:          networkIDMap.GetUUID(network.Name, network.Name),
			}
			networks = append(networks, newNetwork)
		}
	}

	return networks, nil
}

func convertToDiskStruct(envelope []Envelope, ovaPath []string) ([]VmDisk, error) {
	var disks []VmDisk
	for i, ova := range envelope {
		for j, disk := range ova.DiskSection.Disks {
			name := ova.References.File[j].Href
			newDisk := VmDisk{
				FilePath:                getDiskPath(ovaPath[i]),
				Capacity:                disk.Capacity,
				CapacityAllocationUnits: disk.CapacityAllocationUnits,
				DiskId:                  disk.DiskId,
				FileRef:                 disk.FileRef,
				Format:                  disk.Format,
				PopulatedSize:           disk.PopulatedSize,
				Name:                    name,
			}
			newDisk.ID = diskIDMap.GetUUID(newDisk, ovaPath[i]+"/"+name)
			disks = append(disks, newDisk)
		}
	}

	return disks, nil
}

func getDiskPath(path string) string {
	if filepath.Ext(path) != ".ovf" {
		return path
	}

	i := strings.LastIndex(path, "/")
	if i > -1 {
		return path[:i+1]
	}
	return path
}

type UUIDMap struct {
	m map[string]string
}

func NewUUIDMap() *UUIDMap {
	return &UUIDMap{
		m: make(map[string]string),
	}
}

func (um *UUIDMap) GetUUID(object interface{}, key string) string {
	var id string
	id, ok := um.m[key]

	if !ok {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)

		if err := enc.Encode(object); err != nil {
			log.Fatal(err)
		}

		hash := sha256.Sum256(buf.Bytes())
		id = hex.EncodeToString(hash[:])
		if len(id) > 36 {
			id = id[:36]
		}
		um.m[key] = id
	}
	return id
}

func isValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}
