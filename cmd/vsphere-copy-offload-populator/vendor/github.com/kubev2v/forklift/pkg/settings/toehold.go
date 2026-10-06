package settings

import "os"

const (
	ToeholdBuilderImage           = "TOEHOLD_BUILDER_IMAGE"
	ToeholdBaseDiskContainerImage = "TOEHOLD_BASE_DISK_CONTAINER_IMAGE"
	ToeholdBaseContainerImage     = "TOEHOLD_BASE_CONTAINER_IMAGE"
	ToeholdTemplateCPU            = "TOEHOLD_TEMPLATE_CPU"
	ToeholdTemplateMemoryMiB      = "TOEHOLD_TEMPLATE_MEMORY_MIB"
	ToeholdBuildPodCPUs           = "TOEHOLD_CPUS"
	ToeholdBuildPodMemoryMiB      = "TOEHOLD_MEMORY_MIB"
	ToeholdTemplateName           = "TOEHOLD_TEMPLATE_NAME"
	ToeholdVMDKPath               = "TOEHOLD_VMDK_PATH"
	ToeholdTemplateContentHash    = "TOEHOLD_TEMPLATE_CONTENT_HASH"
	ToeholdTemplateConfigHash     = "TOEHOLD_TEMPLATE_CONFIG_HASH"
	ToeholdDatastore              = "TOEHOLD_DATASTORE"
	ToeholdFolder                 = "TOEHOLD_FOLDER"
	ToeholdNetwork                = "TOEHOLD_NETWORK"
	VCenterURL                    = "VCENTER_URL"
	VCenterUser                   = "VCENTER_USER"
	VCenterPassword               = "VCENTER_PASSWORD"
	VCenterInsecure               = "VCENTER_INSECURE"
	VCenterThumbprint             = "VCENTER_THUMBPRINT"
	DefaultBuildPodVMDKPath       = "/work/disk-0.vmdk"
	DefaultToeholdNetwork         = "VM Network"
	DefaultBaseDiskContainerImage = "registry.redhat.io/rhel9/rhel-guest-image:latest"
)

// Toehold settings for the toehold template controller and build pod.
type Toehold struct {
	BuilderImage           string
	BaseDiskContainerImage string
	TemplateCPU            int32
	TemplateMemoryMiB      int32
}

func (r *Toehold) Load() error {
	r.BuilderImage = os.Getenv(ToeholdBuilderImage)
	if r.BuilderImage == "" {
		r.BuilderImage = "quay.io/kubev2v/toehold-builder:latest"
	}
	r.BaseDiskContainerImage = os.Getenv(ToeholdBaseDiskContainerImage)
	if r.BaseDiskContainerImage == "" {
		r.BaseDiskContainerImage = DefaultBaseDiskContainerImage
	}
	r.TemplateCPU = int32(LookupInt(ToeholdTemplateCPU, 2))
	r.TemplateMemoryMiB = int32(LookupInt(ToeholdTemplateMemoryMiB, 4096))
	return nil
}
