package settings

import "os"

const (
	CopyApplianceTemplateBuilderImage           = "COPY_APPLIANCE_TEMPLATE_BUILDER_IMAGE"
	CopyApplianceTemplateBaseDiskContainerImage = "COPY_APPLIANCE_TEMPLATE_BASE_DISK_CONTAINER_IMAGE"
	CopyApplianceTemplateBaseContainerImage     = "COPY_APPLIANCE_TEMPLATE_BASE_CONTAINER_IMAGE"
	CopyApplianceTemplateCPU                    = "COPY_APPLIANCE_TEMPLATE_CPU"
	CopyApplianceTemplateMemoryMiB              = "COPY_APPLIANCE_TEMPLATE_MEMORY_MIB"
	CopyApplianceTemplateBuildPodCPUs           = "COPY_APPLIANCE_TEMPLATE_BUILD_POD_CPUS"
	CopyApplianceTemplateBuildPodMemoryMiB      = "COPY_APPLIANCE_TEMPLATE_BUILD_POD_MEMORY_MIB"
	CopyApplianceTemplateName                   = "COPY_APPLIANCE_TEMPLATE_NAME"
	CopyApplianceTemplateVMDKPath               = "COPY_APPLIANCE_TEMPLATE_VMDK_PATH"
	CopyApplianceTemplateContentHash            = "COPY_APPLIANCE_TEMPLATE_CONTENT_HASH"
	CopyApplianceTemplateConfigHash             = "COPY_APPLIANCE_TEMPLATE_CONFIG_HASH"
	CopyApplianceDatastore                      = "COPY_APPLIANCE_TEMPLATE_DATASTORE"
	CopyApplianceFolder                         = "COPY_APPLIANCE_TEMPLATE_FOLDER"
	CopyApplianceNetwork                        = "COPY_APPLIANCE_TEMPLATE_NETWORK"
	VCenterURL                                  = "VCENTER_URL"
	VCenterUser                                 = "VCENTER_USER"
	VCenterPassword                             = "VCENTER_PASSWORD"
	VCenterInsecure                             = "VCENTER_INSECURE"
	VCenterThumbprint                           = "VCENTER_THUMBPRINT"
	DefaultBuildPodVMDKPath                     = "/work/disk-0.vmdk"
	DefaultCopyApplianceNetwork                 = "VM Network"
	DefaultBaseDiskContainerImage               = "registry.redhat.io/rhel9/rhel-guest-image:latest"
)

// CopyApplianceTemplate settings for the copy appliance template controller and build pod.
type CopyApplianceTemplate struct {
	BuilderImage           string
	BaseDiskContainerImage string
	TemplateCPU            int32
	TemplateMemoryMiB      int32
}

func (r *CopyApplianceTemplate) Load() error {
	r.BuilderImage = os.Getenv(CopyApplianceTemplateBuilderImage)
	if r.BuilderImage == "" {
		r.BuilderImage = "quay.io/kubev2v/copy-appliance-template-builder:latest"
	}
	r.BaseDiskContainerImage = os.Getenv(CopyApplianceTemplateBaseDiskContainerImage)
	if r.BaseDiskContainerImage == "" {
		r.BaseDiskContainerImage = DefaultBaseDiskContainerImage
	}
	r.TemplateCPU = int32(LookupInt(CopyApplianceTemplateCPU, 2))
	r.TemplateMemoryMiB = int32(LookupInt(CopyApplianceTemplateMemoryMiB, 4096))
	return nil
}
