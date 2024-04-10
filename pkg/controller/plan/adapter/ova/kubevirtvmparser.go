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
	OS OS `yaml:"os"`
}

type OS struct {
	Firmware string `yaml:"firmware,omitempty"`
}

func GetFirmwareFromYaml(yamlData []byte) (firmware string, err error) {
	var vmi VirtualMachineInstance
	if err = yaml.Unmarshal(yamlData, &vmi); err != nil {
		return
	}

	if vmi.Spec.Domain.OS.Firmware != "" {
		firmware = vmi.Spec.Domain.OS.Firmware
		return
	}

	err = liberr.New("Firmware type was not detected")
	return
}
