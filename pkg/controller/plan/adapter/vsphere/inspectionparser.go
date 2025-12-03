package vsphere

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/lib/logging"
)

const (
	// Name.
	Name = "virt-v2v-parser"
)

// Package logger.
var log = logging.WithName(Name)

// Map of osinfo ids to vmware guest ids.
var osV2VMap = map[string]string{
	"rocky":    "rockylinux_64Guest",
	"opensuse": "opensuse64Guest",
	"ubuntu":   "ubuntu64Guest",
	"fedora":   "fedora64Guest",

	"centos6": "centos6_64Guest",
	"centos7": "centos7_64Guest",
	"centos8": "centos8_64Guest",
	"centos9": "centos9_64Guest",

	"rhel7":  "rhel7_64Guest",
	"rhel8":  "rhel8_64Guest",
	"rhel9":  "rhel9_64Guest",
	"rhel10": "rhel10_64Guest",

	"sles10": "sles10_64Guest",
	"sles11": "sles11_64Guest",
	"sles12": "sles12_64Guest",
	"sles15": "sles15_64Guest",
	"sles16": "sles16_64Guest",

	"debian4":  "debian4_64Guest",
	"debian5":  "debian5_64Guest",
	"debian6":  "debian6_64Guest",
	"debian7":  "debian7_64Guest",
	"debian8":  "debian8_64Guest",
	"debian9":  "debian9_64Guest",
	"debian10": "debian10_64Guest",
	"debian11": "debian11_64Guest",
	"debian12": "debian12_64Guest",
	"debian13": "debian13_64Guest",

	"win7":  "windows7_64Guest",
	"win8":  "windows8_64Guest",
	"win10": "windows9_64Guest", // This is not typo, VMware naming maps windows9 to windows 10
	"win11": "windows11_64Guest",
	"win12": "windows12_64Guest",

	"win2k8r2":  "windows7Server64Guest",
	"win2k12":   "windows8Server64Guest",
	"win2k12r2": "windows8Server64Guest",
	"win2k16":   "windows9Server64Guest",
	"win2k19":   "windows2019srv_64Guest",
	// VMware naming of newer windows is the previous version and `Next`
	"win2k22": "windows2019srvNext_64Guest",
	"win2k25": "windows2022srvNext_64Guest",
}

type InspectionOS struct {
	Name   string `xml:"name"`
	Distro string `xml:"distro"`
	Osinfo string `xml:"osinfo"`
	Arch   string `xml:"arch"`
}

type InspectionV2V struct {
	OS InspectionOS `xml:"operatingsystem"`
}

func ParseInspectionFromString(xmlData string) (InspectionV2V, error) {
	var xmlConf InspectionV2V
	err := xml.Unmarshal([]byte(xmlData), &xmlConf)
	if err != nil {
		return InspectionV2V{}, fmt.Errorf("Error unmarshalling XML: %v\n", err)
	}
	return xmlConf, nil
}

func GetOperationSystemFromConfig(vmConfigXML string) (string, error) {
	inspection, err := ParseInspectionFromString(vmConfigXML)
	if err != nil {
		return "", err
	}
	os, ok := osV2VMap[inspection.OS.Osinfo]
	if ok {
		return os, nil
	}
	// Some operating system can contain a minor version for that we would require large map
	// Example rhel9.5, rhel9.6 etc.
	osInfoWithVersion := inspection.OS.Osinfo
	osInfo := strings.Split(osInfoWithVersion, ".")[0]

	os, ok = osV2VMap[osInfo]
	if ok {
		return os, nil
	}
	log.Info(fmt.Sprintf("Received %s, mapped to: %s", inspection.OS.Osinfo, os))

	if inspection.OS.Name == "linux" {
		return "genericLinuxGuest", nil
	}
	return "otherGuest64", nil
}
