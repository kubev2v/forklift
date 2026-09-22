package builder

import (
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
)

const (
	TemplateOSLabel       = "os.template.kubevirt.io/%s"
	TemplateWorkloadLabel = "workload.template.kubevirt.io/server"
	TemplateFlavorLabel   = "flavor.template.kubevirt.io/medium"
)

const (
	DefaultWindows = "win10"
	DefaultLinux   = "rhel8.1"
	Unknown        = "unknown"
)

func (r *Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vmRef)
	if err != nil {
		return nil, err
	}

	os := r.detectOS(azureVM)

	labels = make(map[string]string)
	labels[fmt.Sprintf(TemplateOSLabel, os)] = "true"
	labels[TemplateWorkloadLabel] = "true"
	labels[TemplateFlavorLabel] = "true"

	return labels, nil
}

func (r *Builder) detectOS(azureVM *inventory.VMDetails) string {
	if azureVM.Properties == nil || azureVM.Properties.StorageProfile == nil ||
		azureVM.Properties.StorageProfile.OSDisk == nil {
		return DefaultLinux
	}

	osDisk := azureVM.Properties.StorageProfile.OSDisk
	if osDisk.OSType != nil {
		switch *osDisk.OSType {
		case "Windows":
			return DefaultWindows
		case "Linux":
			return DefaultLinux
		}
	}

	if azureVM.Properties.StorageProfile.ImageReference != nil {
		imgRef := azureVM.Properties.StorageProfile.ImageReference
		offer := ""
		if imgRef.Offer != nil {
			offer = strings.ToLower(*imgRef.Offer)
		}

		if strings.Contains(offer, "windows") {
			return DefaultWindows
		}
		if strings.Contains(offer, "rhel") || strings.Contains(offer, "red hat") {
			return "rhel8.1"
		}
		if strings.Contains(offer, "ubuntu") {
			return "ubuntu20.04"
		}
		if strings.Contains(offer, "centos") {
			return "centos8"
		}
		if strings.Contains(offer, "debian") {
			return "debian10"
		}
		if strings.Contains(offer, "sles") || strings.Contains(offer, "suse") {
			return "opensuse15.0"
		}
	}

	return DefaultLinux
}
