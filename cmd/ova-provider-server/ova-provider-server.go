package main

import (
	"archive/tar"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
}

type VirtualConfig struct {
	XMLName  xml.Name `xml:"http://www.vmware.com/schema/ovf Config"`
	Required string   `xml:"required,attr"`
	Key      string   `xml:"key,attr"`
	Value    string   `xml:"value,attr"`
}

type VirtualHardwareSection struct {
	Info    string          `xml:"Info"`
	Items   []Item          `xml:"Item"`
	Configs []VirtualConfig `xml:"Config"`
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
	} `xml:"OperatingSystemSection"`
	HardwareSection VirtualHardwareSection `xml:"VirtualHardwareSection"`
}

type Envelope struct {
	XMLName        xml.Name        `xml:"Envelope"`
	VirtualSystem  []VirtualSystem `xml:"VirtualSystem"`
	DiskSection    DiskSection     `xml:"DiskSection"`
	NetworkSection NetworkSection  `xml:"NetworkSection"`
	References     References      `xml:"References"`
}

// vm struct
type VM struct {
	Name                  string
	OvaPath               string
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
	key   string
	Value string
}

// Virtual ethernet card.
type NIC struct {
	Name   string `json:"name"`
	MAC    string `json:"mac"`
	Config []Conf
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

	http.ListenAndServe(":8080", nil)

}

func connHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("Test connection successful")
	fmt.Println("Test connection handeler was called")
}

func vmHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	vmXML, ovaPath := scanOVAsOnNFS()
	vmStruct, err := convertToVmStruct(vmXML, ovaPath)
	if err != nil {
		fmt.Println("Error processing OVF file:", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vmStruct)
	fmt.Println("VM handeler was called")
}

func diskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	xmlStruct, ovaPath := scanOVAsOnNFS()
	diskStruct, err := convertToDiskStruct(xmlStruct, ovaPath)
	if err != nil {
		fmt.Println("Error processing OVF file:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diskStruct)
	fmt.Println("Disk handeler was called")
}

func networkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	xmlStruct, _ := scanOVAsOnNFS()
	netStruct, err := convertToNetworkStruct(xmlStruct)
	if err != nil {
		fmt.Println("Error processing OVF file:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(netStruct)
	fmt.Println("Network handeler was called")
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

func findOVAFiles(directory string) ([]string, []string, error) {
	var ovaFiles, ovfFiles []string

	dirs, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, nil, err
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			newDir := directory + "/" + dir.Name()
			files, err := ioutil.ReadDir(newDir)
			if err != nil {
				return nil, nil, err
			}
			for _, file := range files {
				if strings.HasSuffix(strings.ToLower(file.Name()), ".ova") {
					ovaFiles = append(ovaFiles, filepath.Join(directory, dir.Name(), file.Name()))
				}

				if strings.HasSuffix(strings.ToLower(file.Name()), ".ovf") {
					ovfFiles = append(ovfFiles, filepath.Join(directory, dir.Name(), file.Name()))
				}
			}
		}
	}
	return ovaFiles, ovfFiles, nil
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

func convertToVmStruct(envelope []Envelope, ovaPath []string) ([]VM, error) {
	var vms []VM

	for i := 0; i < len(envelope); i++ {
		vmXml := envelope[i]
		for _, virtualSystem := range vmXml.VirtualSystem {

			// Initialize a new VM
			newVM := VM{
				OvaPath: ovaPath[i],
				Name:    virtualSystem.Name,
				UUID:    vmIDMap.GetUUID(ovaPath[i]),
			}

			for _, item := range virtualSystem.HardwareSection.Items {
				if strings.Contains(item.ElementName, "Network adapter") {
					newVM.NICs = append(newVM.NICs, NIC{
						Name: item.ElementName,
						MAC:  item.Address,
					})
					//for _conf := range item.
				} else if strings.Contains(item.Description, "Number of Virtual CPUs") {
					newVM.CpuCount = item.VirtualQuantity

				} else if strings.Contains(item.Description, "Memory Size") {
					newVM.MemoryMB = item.VirtualQuantity

				} else {
					newVM.Devices = append(newVM.Devices, Device{
						Kind: item.ElementName[:len(item.ElementName)-2],
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
					ID:                      diskIDMap.GetUUID(ovaPath[i] + "/" + name),
				})
			}

			for _, network := range vmXml.NetworkSection.Networks {
				newVM.Networks = append(newVM.Networks, VmNetwork{
					Name:        network.Name,
					Description: network.Description,
					ID:          networkIDMap.GetUUID(network.Name),
				})
			}

			for _, conf := range virtualSystem.HardwareSection.Configs {
				if conf.Key == "firmware" {
					newVM.Firmware = conf.Value
				} else if conf.Key == "memoryHotAddEnabled" {
					newVM.MemoryHotAddEnabled, _ = strconv.ParseBool(conf.Value)
				} else if conf.Key == "cpuHotAddEnabled" {
					newVM.CpuHotAddEnabled, _ = strconv.ParseBool(conf.Value)
				} else if conf.Key == "cpuHotRemoveEnabled" {
					newVM.CpuHotRemoveEnabled, _ = strconv.ParseBool(conf.Value)
				}
			}
			vms = append(vms, newVM)
		}
	}
	return vms, nil
}

func convertToNetworkStruct(envelope []Envelope) ([]VmNetwork, error) {
	var networks []VmNetwork
	for _, ova := range envelope {
		for _, network := range ova.NetworkSection.Networks {
			newNetwork := VmNetwork{
				Name:        network.Name,
				Description: network.Description,
				ID:          networkIDMap.GetUUID(network.Name),
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
				ID:                      diskIDMap.GetUUID(ovaPath[i] + "/" + name),
			}

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

func (um *UUIDMap) GetUUID(filename string) string {
	var id string
	id, ok := um.m[filename]

	if !ok {
		uuid := make([]byte, 16)
		n, err := io.ReadFull(rand.Reader, uuid)
		if n != len(uuid) || err != nil {
			return ""
		}
		uuid[6] = (uuid[6] & 0x0f) | 0x40
		uuid[8] = (uuid[8] & 0x3f) | 0x80
		id = fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])

		um.m[filename] = id
	}
	return id
}
