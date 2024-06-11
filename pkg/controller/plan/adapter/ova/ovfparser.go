package ova

import (
	"encoding/xml"
	"strings"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
)

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
	return xmlConf.Metadata.LibOsInfo.V2VOS.ID, nil
}
