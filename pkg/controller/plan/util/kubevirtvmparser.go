package util

import (
	"fmt"
	"strings"

	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"gopkg.in/yaml.v2"
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

type OS struct {
	Firmware string `yaml:"firmware"`
}

type Domain struct {
	OS OS `yaml:"os"`
}

type TemplateSpec struct {
	Domain Domain `yaml:"domain"`
}

type Template struct {
	Spec TemplateSpec `yaml:"spec"`
}

type VirtualMachineSpec struct {
	Template Template `yaml:"template"`
}

type VirtualMachine struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   Metadata           `yaml:"metadata"`
	Spec       VirtualMachineSpec `yaml:"spec"`
}

type Metadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

func GetFirmwareFromYaml(yamlData []byte) (string, error) {
	var vm VirtualMachine
	if err := yaml.Unmarshal(yamlData, &vm); err != nil {
		return "", err
	}

	firmware := vm.Spec.Template.Spec.Domain.OS.Firmware
	if firmware == "" {
		log.Info("Firmware type was not detected")
	}

	return firmware, nil
}

func GetOperationSystemFromYaml(yamlData []byte) (os string, err error) {
	var vm VirtualMachine
	if err = yaml.Unmarshal(yamlData, &vm); err != nil {
		return
	}

	labels := vm.Metadata.Labels
	if osinfo, ok := labels["libguestfs.org/osinfo"]; ok {
		return mapOs(osinfo), nil

	}
	return
}

func mapOs(labelOS string) (os string) {
	distro := strings.SplitN(labelOS, ".", 2)[0]

	switch {
	case strings.HasPrefix(distro, "rocky"):
		distro = "rocky"
	case strings.HasPrefix(distro, "opensuse"):
		distro = "opensuse"
	case strings.HasPrefix(distro, "ubuntu"):
		distro = "ubuntu"
	case strings.HasPrefix(distro, "fedora"):
		distro = "fedora"
	}

	os, ok := osV2VMap[distro]
	if !ok {
		log.Info(fmt.Sprintf("Received %s, mapped to: %s", labelOS, os))
		os = "otherGuest64"
	}
	return
}
