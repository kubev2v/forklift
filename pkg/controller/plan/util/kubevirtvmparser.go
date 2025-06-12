package util

import (
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/kubev2v/forklift/pkg/lib/logging"
)

const (
	// Name.
	Name = "kubevirt-vm-parser"
)

// Package logger.
var log = logging.WithName(Name)

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
	if firmware != "" {
		return firmware, nil
	}

	// FIXME: In newer version of virt-v2v the output will change to support CNV VM format.
	// With this we will support both so the migrations should not fail during the update to newer virt-v2v.
	// But we still need to remove the custom templating.
	// https://issues.redhat.com/browse/RHEL-58065
	var cnvVm *cnv.VirtualMachine
	if err := yaml.Unmarshal(yamlData, &cnvVm); err != nil {
		return "", err
	}

	if cnvVm.Spec.Template != nil &&
		cnvVm.Spec.Template.Spec.Domain.Firmware != nil &&
		cnvVm.Spec.Template.Spec.Domain.Firmware.Bootloader != nil {

		if cnvVm.Spec.Template.Spec.Domain.Firmware.Bootloader.BIOS != nil {
			return "bios", nil
		}
		if cnvVm.Spec.Template.Spec.Domain.Firmware.Bootloader.EFI != nil {
			return "uefi", nil
		}
	}

	log.Info("Firmware type was not detected")
	return "", nil
}
