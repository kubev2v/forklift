package vsphere

import (
	"encoding/xml"
	"fmt"

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
	"centos6":  "centos6_64Guest",
	"centos7":  "centos7_64Guest",
	"centos8":  "centos8_64Guest",
	"centos9":  "centos9_64Guest",
	"rhel7":    "rhel7_64Guest",
	"rhel8":    "rhel8_64Guest",
	"rhel9":    "rhel9_64Guest",
	"rocky":    "rockylinux_64Guest",
	"sles10":   "sles10_64Guest",
	"sles11":   "sles11_64Guest",
	"sles12":   "sles12_64Guest",
	"sles15":   "sles15_64Guest",
	"sles16":   "sles16_64Guest",
	"opensuse": "opensuse64Guest",
	"debian4":  "debian4_64Guest",
	"debian5":  "debian5_64Guest",
	"debian6":  "debian6_64Guest",
	"debian7":  "debian7_64Guest",
	"debian8":  "debian8_64Guest",
	"debian9":  "debian9_64Guest",
	"debian10": "debian10_64Guest",
	"debian11": "debian11_64Guest",
	"debian12": "debian12_64Guest",
	"ubuntu":   "ubuntu64Guest",
	"fedora":   "fedora64Guest",
	"win7":     "windows7Server64Guest",
	"win8":     "windows8Server64Guest",
	"win10":    "windows9Server64Guest",
	"win11":    "windows11_64Guest",
	"win12":    "windows12_64Guest",
	"win2k19":  "windows2019srv_64Guest",
	"win2k22":  "windows2022srvNext_64Guest",
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
	if !ok {
		log.Info(fmt.Sprintf("Received %s, mapped to: %s", os, os))
		os = "otherGuest64"
	}
	return os, nil
}
