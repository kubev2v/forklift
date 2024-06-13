package ova

import (
	"encoding/xml"
	"fmt"
	"strings"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
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

type OvaVmconfig struct {
	XMLName  xml.Name `xml:"domain"`
	Firmware Firmware `xml:"firmware"`
	Metadata Metadata `xml:"metadata"`
}

type Metadata struct {
	LibOsInfo LibOsInfo `xml:"libosinfo"`
}

type LibOsInfo struct {
	V2VOS V2VOS `xml:"os"`
}

type V2VOS struct {
	ID string `xml:"id,attr"`
}

type Firmware struct {
	Bootloader Bootloader `xml:"bootloader"`
}

type Bootloader struct {
	Type string `xml:"type,attr"`
}

func readConfFromXML(xmlData string) (*OvaVmconfig, error) {
	var vmConfig OvaVmconfig

	reader := strings.NewReader(xmlData)
	decoder := xml.NewDecoder(reader)

	err := decoder.Decode(&vmConfig)
	if err != nil {
		return &vmConfig, err
	}
	return &vmConfig, nil
}

func GetFirmwareFromConfig(vmConfigXML string) (firmware string, err error) {
	xmlConf, err := readConfFromXML(vmConfigXML)
	if err != nil {
		return
	}

	firmware = xmlConf.Firmware.Bootloader.Type
	if firmware == "" {
		err = liberr.New("failed to get the firmware type from virt-v2v config")
	}
	return
}

func GetOperationSystemFromConfig(vmConfigXML string) (os string, err error) {
	xmlConf, err := readConfFromXML(vmConfigXML)
	if err != nil {
		return
	}
	return mapOs(xmlConf.Metadata.LibOsInfo.V2VOS.ID), nil
}

func mapOs(xmlOs string) (os string) {
	split := strings.Split(xmlOs, "/")
	distro := split[3]
	switch distro {
	case "rocky", "opensuse", "ubuntu", "fedora":
		os = distro
	default:
		os = split[3] + strings.Split(split[4], ".")[0]
	}
	os, ok := osV2VMap[os]
	if !ok {
		log.Info(fmt.Sprintf("Received %s, mapped to: %s", xmlOs, os))
		os = "otherGuest64"
	}
	return
}
