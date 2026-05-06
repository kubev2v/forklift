package types

// VirtV2VInspectorXML represents the XML structure returned by virt-v2v-inspector
type VirtV2VInspectorXML struct {
	OS VirtV2VInspectorOS `xml:"operatingsystem" json:"operatingsystem"`
}

// VirtV2VInspectorOS represents an operating system entry in virt-v2v-inspector XML
type VirtV2VInspectorOS struct {
	Name              string                      `xml:"name" json:"name"`
	Distro            string                      `xml:"distro" json:"distro"`
	Osinfo            string                      `xml:"osinfo" json:"osinfo,omitempty"`
	Arch              string                      `xml:"arch" json:"architecture"`
	MajorVersion      string                      `xml:"major_version" json:"major_version"`
	MinorVersion      string                      `xml:"minor_version" json:"minor_version"`
	ProductName       string                      `xml:"product_name" json:"product,omitempty"`
	ProductVariant    string                      `xml:"product_variant" json:"product_variant,omitempty"`
	Root              string                      `xml:"root" json:"root,omitempty"`
	PackageFormat     string                      `xml:"package_format" json:"package_format,omitempty"`
	PackageManagement string                      `xml:"package_management" json:"package_management,omitempty"`
	Mountpoints       VirtV2VInspectorMountpoints `xml:"mountpoints" json:"mountpoints,omitempty"`
}

// VirtV2VInspectorMountpoints represents the mountpoints section
type VirtV2VInspectorMountpoints struct {
	Mountpoints []VirtV2VInspectorMountpoint `xml:"mountpoint" json:"mountpoints"`
}

// VirtV2VInspectorMountpoint represents a mountpoint
type VirtV2VInspectorMountpoint struct {
	Device string `xml:"dev,attr" json:"device"`
	Path   string `xml:",chardata" json:"mount_point"`
}
