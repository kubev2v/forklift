package api

import (
	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// OSFamily distinguishes the top-level OS type.
type OSFamily string

const (
	OSFamilyLinux   OSFamily = "linux"
	OSFamilyWindows OSFamily = "windows"
	OSFamilyUnknown OSFamily = "unknown"
)

// GuestOS holds OS-level information detected from the guest disk.
type GuestOS struct {
	Family  OSFamily // linux or windows
	Distro  string   // "rhel", "ubuntu", "debian", "sles", etc. (empty for Windows)
	Version string   // "9.2", "22.04" (best-effort from os-release)
}

func (g *GuestOS) IsWindows() bool { return g.Family == OSFamilyWindows }
func (g *GuestOS) IsLinux() bool   { return g.Family == OSFamilyLinux }

// InterfaceInfo is populated by probe parsers (ifcfg, NM, netplan, interfaces, wicked)
// and consumed by network plugins to map MAC→interface name for static IP assignment.
type InterfaceInfo struct {
	Name   string   // Interface name ("eth0", "ens192")
	IPv4   []string // IPv4 addresses from config
	IPv6   []string // IPv6 addresses from config
	MAC    string   // MAC address if in config (may be empty)
	DHCP   bool     // True if configured via DHCP (BOOTPROTO=dhcp, method=auto, dhcp4: true)
	Source string   // "ifcfg", "nm-connection", "netplan", "interfaces", "wicked"
}

// GuestInfo is the complete probe result -- everything customize needs.
type GuestInfo struct {
	OS GuestOS

	// Network stacks detected on disk (multiple can coexist)
	UsesIfcfg          bool
	UsesIfcfgSuse      bool // SUSE path: /etc/sysconfig/network/ifcfg-*
	UsesNetworkManager bool
	UsesNetplan        bool
	UsesIfquery        bool
	UsesInterfacesD    bool // /etc/network/interfaces.d/ directory exists
	UsesWicked         bool

	// Netplan renderer ("networkd" or "NetworkManager"), if detected
	NetplanRenderer string

	// Pre-extracted interface configs
	Interfaces []InterfaceInfo
}

// InterfaceForIP returns the interface name associated with the given IP,
// searching both IPv4 and IPv6 addresses.
func (g *GuestInfo) InterfaceForIP(ip string) string {
	for _, iface := range g.Interfaces {
		for _, v := range iface.IPv4 {
			if v == ip {
				return iface.Name
			}
		}
		for _, v := range iface.IPv6 {
			if v == ip {
				return iface.Name
			}
		}
	}
	return ""
}

// HasIPs returns true if the interface has any IPv4 or IPv6 address.
func (i *InterfaceInfo) HasIPs() bool {
	return len(i.IPv4) > 0 || len(i.IPv6) > 0
}

// WinFirstbootScriptsPath is the guest-side directory where Windows firstboot
// PowerShell and batch scripts are placed for execution after conversion.
const WinFirstbootScriptsPath = "/Program Files/Guestfs/Firstboot/scripts"

// FileActionType identifies the kind of file operation.
type FileActionType string

const (
	ActionUpload FileActionType = "upload"
	ActionWrite  FileActionType = "write"
)

// FileAction is a file operation performed by guestfish (upload or write).
type FileAction struct {
	Type        FileActionType
	LocalPath   string // Source path on host (for Upload)
	GuestPath   string // Destination path inside guest
	Content     []byte // Content to write (for Write)
	Permissions string // e.g., "0644" (optional)
}

// ExecActionType identifies the kind of virt-customize operation.
type ExecActionType string

const (
	ActionFirstboot ExecActionType = "firstboot"
	ActionRun       ExecActionType = "run"
)

// ExecAction is a virt-customize operation (script execution or key injection).
type ExecAction struct {
	Type  ExecActionType
	Value string // script path for firstboot/run, key spec for key (e.g. "all:clevis")
}

// Actions collects file and exec operations returned by a plugin.
type Actions struct {
	Files []FileAction
	Execs []ExecAction
}

// Context carries all shared state that stack plugins need.
type Context struct {
	Guest      *GuestInfo
	Config     *config.AppConfig
	Disks      []string
	FileSystem utils.FileSystem
}

// Plugin is a modular unit handling one aspect of guest customization.
type Plugin interface {
	Name() string
	Applicable(ctx *Context) bool
	Apply(ctx *Context) (*Actions, error)
}
