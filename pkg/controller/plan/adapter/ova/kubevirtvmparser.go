package ova

import (
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"gopkg.in/yaml.v2"
)

type VirtualMachineInstance struct {
	ApiVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Spec struct {
	Domain Domain `yaml:"domain"`
}

type Domain struct {
	Firmware Firmware `yaml:"firmware,omitempty"`
}

type Firmware struct {
	Bootloader Bootloader `yaml:"bootloader,omitempty"`
}

type Bootloader struct {
	Bios *Bios `yaml:"bios,omitempty"`
	EFI  *EFI  `yaml:"efi,omitempty"`
}

type Bios struct{}

type EFI struct {
	SecureBoot bool `yaml:"secureBoot"`
}

func GetFirmwareFromYaml(yamlData []byte) (firmware string, err error) {
	var vmi VirtualMachineInstance
	if err = yaml.Unmarshal(yamlData, &vmi); err != nil {
		return
	}

	if vmi.Spec.Domain.Firmware.Bootloader.Bios != nil {
		firmware = "bios"
		return
	}
	if vmi.Spec.Domain.Firmware.Bootloader.EFI != nil {
		firmware = "efi"
		return
	}
	err = liberr.New("Firmware type was not detected")
	return
}
