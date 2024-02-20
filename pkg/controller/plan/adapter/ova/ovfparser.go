package ova

import (
	"encoding/xml"
	"fmt"
	"strings"

	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/ova"
)

type OvaVmconfig struct {
	XMLName xml.Name `xml:"domain"`
	Name    string   `xml:"name"`
	OS      OS       `xml:"os"`
}

type OS struct {
	Type   OSType `xml:"type"`
	Loader Loader `xml:"loader"`
	Nvram  Nvram  `xml:"nvram"`
}

type OSType struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr"`
	Content string `xml:",chardata"`
}

type Loader struct {
	Readonly string `xml:"readonly,attr"`
	Type     string `xml:"type,attr"`
	Secure   string `xml:"secure,attr"`
	Path     string `xml:",chardata"`
}

type Nvram struct {
	Template string `xml:"template,attr"`
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

func (r *Builder) GetFirmwareFromConfig(vm *model.VM) (conf string, err error) {
	var xmlConfig string
	for _, vmConf := range r.Migration.Status.VMs {
		if vmConf.ID == vm.ID {
			xmlConfig = vmConf.OvfConfig
			fmt.Println("we are at the config here ", xmlConfig)
		}
	}
	xmlConf, err := readConfFromXML(xmlConfig)
	if err != nil {
		return
	}

	path := xmlConf.OS.Loader.Path
	if strings.Contains(path, "OVMF") {
		return UEFI, nil
	}
	return BIOS, nil
}
