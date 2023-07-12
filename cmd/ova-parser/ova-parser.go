package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
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

type DiskSection struct {
	XMLName xml.Name `xml:"DiskSection"`
	Info    string   `xml:"Info"`
	Disks   []Disk   `xml:"Disk"`
}

type Disk struct {
	XMLName                 xml.Name `xml:"Disk"`
	Capacity                string   `xml:"capacity,attr"`
	CapacityAllocationUnits string   `xml:"capacityAllocationUnits,attr"`
	DiskId                  string   `xml:"diskId,attr"`
	FileRef                 string   `xml:"fileRef,attr"`
	Format                  string   `xml:"format,attr"`
	PopulatedSize           string   `xml:"populatedSize,attr"`
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
	FilePath                string
	Capacity                string
	CapacityAllocationUnits string
	DiskId                  string
	FileRef                 string
	Format                  string
	PopulatedSize           string
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
}

func main() {

	http.HandleFunc("/vms", vmHandler)
	http.HandleFunc("/disks", diskHandler)
	http.HandleFunc("/networks", networkHandler)
	http.HandleFunc("/watch", watchdHandler)
	http.HandleFunc("/test_connection", connHendler)

	http.ListenAndServe(":8080", nil)

}

func connHendler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("")
}

func vmHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	vmXML, ovfPath := scanOVAsOnNFS()
	vmStruct, err := convertToVmStruct(vmXML, ovfPath)
	if err != nil {
		fmt.Println("Error processing OVF file:", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vmStruct)
}

func diskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	xmlStruct, ovfPath := scanOVAsOnNFS()
	diskStruct, err := convertToDiskStruct(xmlStruct, ovfPath)
	if err != nil {
		fmt.Println("Error processing OVF file:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diskStruct)
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
}

func watchdHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(w, "This is the watch page!")
	//TODO add watch
}

func scanOVAsOnNFS() (envelopes []Envelope, ovaPaths []string) {
	ovaFiles, err := findOVAFiles("/ova")
	if err != nil {
		fmt.Println("Error finding OVA files:", err)
		return
	}

	for _, ovaFile := range ovaFiles {
		fmt.Println("Processing OVA file:", ovaFile)

		ovfPath, tmpDir, err := extractOVFFromOVA(ovaFile)
		if err != nil {
			fmt.Println("Error extracting OVF from OVA:", err)
			continue
		}

		xmlStruct, err := processOVF(ovfPath)
		if err != nil {
			fmt.Println("Error processing OVF file:", err)
			os.RemoveAll(tmpDir)
		}

		os.RemoveAll(tmpDir)
		envelopes = append(envelopes, *xmlStruct)
		ovaPaths = append(ovaPaths, ovfPath)

	}
	return *&envelopes, ovaPaths
}

func findOVAFiles(directory string) ([]string, error) {
	var ovaFiles []string

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".ova") {
			ovaFiles = append(ovaFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ovaFiles, nil
}

func extractOVFFromOVA(ovaFile string) (string, string, error) {
	tmpDir, err := ioutil.TempDir("", "ova_extraction")
	print(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", err
	}

	cmd := exec.Command("tar", "-xf", ovaFile, "-C", tmpDir, "*.ovf")
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", err
	}

	ovfFiles, err := filepath.Glob(filepath.Join(tmpDir, "*.ovf"))
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", "", err
	}

	if len(ovfFiles) == 0 {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("no OVF file found in the OVA")
	}

	return ovfFiles[0], tmpDir, nil
}

func processOVF(ovfPath string) (*Envelope, error) {
	var envelope Envelope

	xmlFile, err := os.Open(ovfPath)
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()

	decoder := xml.NewDecoder(xmlFile)

	err = decoder.Decode(&envelope)
	if err != nil {
		return &envelope, err
	}

	fmt.Println("Virtual Systems:")
	for _, virtualSystem := range envelope.VirtualSystem {
		fmt.Println("Virtual System ID:", virtualSystem.ID)
		fmt.Println("Virtual System Name:", virtualSystem.Name)
		fmt.Println("Operating System Description:", virtualSystem.OperatingSystemSection.Description)
		fmt.Println("Virtual Hardware Info:", virtualSystem.HardwareSection.Info)

		fmt.Println("Virtual System Items:")
		for _, item := range virtualSystem.HardwareSection.Items {
			fmt.Printf("ElementName: %s, InstanceID: %s, ResourceType: %s\n", item.ElementName, item.InstanceID, item.ResourceType)
			for _, conf := range item.Configs {
				fmt.Printf("conf req: %s, key: %s, value: %s", conf.Required, conf.Key, conf.Value)
			}
		}

		fmt.Println("Disk Settings:")
		for _, disk := range envelope.DiskSection.Disks {
			fmt.Printf("DiskSize: %s, DiskId: %s\n", disk.Capacity, disk.DiskId)
		}

		fmt.Println("Network Settings:")
		for _, network := range envelope.NetworkSection.Networks {
			fmt.Printf("NetworkName: %s, NetworkId: %s\n", network.Name, network.Description)
		}

		fmt.Println("Config:")
		for _, conf := range virtualSystem.HardwareSection.Configs {
			fmt.Printf("conf req: %s, key: %s, value: %s", conf.Required, conf.Key, conf.Value)
		}
		fmt.Println()
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

			for _, disk := range vmXml.DiskSection.Disks {
				newVM.Disks = append(newVM.Disks, VmDisk{
					FilePath:                ovaPath[i],
					Capacity:                disk.Capacity,
					CapacityAllocationUnits: disk.CapacityAllocationUnits,
					DiskId:                  disk.DiskId,
					FileRef:                 disk.FileRef,
					Format:                  disk.Format,
					PopulatedSize:           disk.PopulatedSize,
				})
			}

			for _, network := range vmXml.NetworkSection.Networks {
				newVM.Networks = append(newVM.Networks, VmNetwork{
					Name:        network.Name,
					Description: network.Description,
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
			}
			networks = append(networks, newNetwork)
		}
	}

	return networks, nil
}

func convertToDiskStruct(envelope []Envelope, ovaPath []string) ([]VmDisk, error) {
	var disks []VmDisk
	for i := 0; i < len(envelope); i++ {
		ova := envelope[i]
		for _, disk := range ova.DiskSection.Disks {
			newDisk := VmDisk{
				FilePath:                ovaPath[i],
				Capacity:                disk.Capacity,
				CapacityAllocationUnits: disk.CapacityAllocationUnits,
				DiskId:                  disk.DiskId,
				FileRef:                 disk.FileRef,
				Format:                  disk.Format,
				PopulatedSize:           disk.PopulatedSize,
			}

			disks = append(disks, newDisk)
		}
	}

	return disks, nil
}
