package types

// VirtInspectorXML represents the XML structure returned by virt-inspector
type VirtInspectorXML struct {
	Operatingsystems []OS `xml:"operatingsystem" json:"operatingsystems"`
}

// OS represents an operating system entry in virt-inspector XML
type OS struct {
	Name              string       `xml:"name" json:"name"`
	Distro            string       `xml:"distro" json:"distro"`
	MajorVersion      string       `xml:"major_version" json:"major_version"`
	MinorVersion      string       `xml:"minor_version" json:"minor_version"`
	Architecture      string       `xml:"arch" json:"architecture"`
	Hostname          string       `xml:"hostname" json:"hostname,omitempty"`
	Product           string       `xml:"product_name" json:"product,omitempty"`
	Root              string       `xml:"root" json:"root,omitempty"`
	PackageFormat     string       `xml:"package_format" json:"package_format,omitempty"`
	PackageManagement string       `xml:"package_management" json:"package_management,omitempty"`
	OSInfo            string       `xml:"osinfo" json:"osinfo,omitempty"`
	Applications      Applications `xml:"applications" json:"applications,omitempty"`
	Filesystems       Filesystems  `xml:"filesystems" json:"filesystems,omitempty"`
	Mountpoints       Mountpoints  `xml:"mountpoints" json:"mountpoints,omitempty"`
	Drives            Drives       `xml:"drives" json:"drives,omitempty"`
}

// Applications represents the applications section
type Applications struct {
	Application []Application `xml:"application" json:"applications"`
}

// Application represents an installed application
type Application struct {
	Name        string `xml:"name" json:"name"`
	Version     string `xml:"version" json:"version,omitempty"`
	Epoch       int    `xml:"epoch" json:"epoch,omitempty"`
	Release     string `xml:"release" json:"release,omitempty"`
	Arch        string `xml:"arch" json:"arch,omitempty"`
	URL         string `xml:"url" json:"url,omitempty"`
	Summary     string `xml:"summary" json:"summary,omitempty"`
	Description string `xml:"description" json:"description,omitempty"`
}

// Filesystems represents the filesystems section
type Filesystems struct {
	Filesystem []Filesystem `xml:"filesystem" json:"filesystems"`
}

// Filesystem represents a filesystem
type Filesystem struct {
	Device string `xml:"dev,attr" json:"device"`
	Type   string `xml:"type" json:"type"`
	UUID   string `xml:"uuid" json:"uuid,omitempty"`
}

// Mountpoints represents the mountpoints section
type Mountpoints struct {
	Mountpoint []Mountpoint `xml:"mountpoint" json:"mountpoints"`
}

// Mountpoint represents a mountpoint
type Mountpoint struct {
	Device     string `xml:"dev,attr" json:"device"`
	MountPoint string `xml:",chardata" json:"mount_point"`
}

// Drives represents the drives section
type Drives struct {
	Drive []Drive `xml:"drive" json:"drives"`
}

// Drive represents a drive
type Drive struct {
	Name string `xml:"name,attr" json:"name"`
}
