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

// InterfaceInfo is populated by probe parsers (ifcfg, NM, NM DHCP leases,
// dhclient, netplan, interfaces, wicked) and consumed by network plugins to
// map MAC→interface name for static IP assignment.
type InterfaceInfo struct {
	Name   string   // Interface name ("eth0", "ens192")
	IPv4   []string // IPv4 addresses from config
	IPv6   []string // IPv6 addresses from config
	MAC    string   // MAC address if in config (may be empty)
	DHCP   bool     // True if configured via DHCP (BOOTPROTO=dhcp, method=auto, dhcp4: true)
	Source string   // "ifcfg", "nm-connection", "nm-dhcp-lease", "dhclient", "netplan", "interfaces", "wicked"
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
	UsesNMDhcpLease    bool // /var/lib/NetworkManager/*.lease files exist
	UsesDhclient       bool // /var/lib/dhclient/ directory exists

	// Netplan renderer ("networkd" or "NetworkManager"), if detected
	NetplanRenderer string

	// Pre-extracted interface configs
	Interfaces []InterfaceInfo

	// Subsystem probes (Linux only)
	CloudInit CloudInitInfo
	SSH       SSHInfo
	Console   ConsoleInfo
}

// CloudInitInfo holds cloud-init detection and configuration state.
// Plugins use this to prevent cloud-init from overriding network config
// after migration (the primary post-migration failure mode).
type CloudInitInfo struct {
	Present         bool // /etc/cloud/ dir or /usr/bin/cloud-init exists
	HasCloudCfg     bool // /etc/cloud/cloud.cfg exists
	HasCloudCfgD    bool // /etc/cloud/cloud.cfg.d/ dir exists
	HasInstanceData bool // /var/lib/cloud/instance/ dir exists
	HasSeedData     bool // /var/lib/cloud/seed/ dir exists

	DatasourceList        []string // from cloud.cfg: datasource_list
	ActiveDatasource      string   // from /var/lib/cloud/instance/datasource
	NetworkConfigDisabled bool     // network: {config: disabled} in cloud.cfg
	InstanceID            string   // from /var/lib/cloud/instance/instance-id
}

// ManagesNetwork returns true if cloud-init is present and has not been
// told to leave networking alone. Plugins can use this to decide whether
// cloud-init networking needs to be disabled before migration.
func (c *CloudInitInfo) ManagesNetwork() bool {
	return c.Present && !c.NetworkConfigDisabled
}

// SSHInfo holds SSH service detection for post-migration login capability.
type SSHInfo struct {
	Present               bool // /usr/sbin/sshd exists
	HasConfig             bool // /etc/ssh/sshd_config exists
	ServiceEnabled        bool // sshd.service or ssh.service in multi-user.target.wants
	HasHostKeys           bool // /etc/ssh/ssh_host_*_key glob matched
	HasRootAuthorizedKeys bool // /root/.ssh/authorized_keys exists

	PermitRootLogin        string // "yes", "no", "prohibit-password", etc.
	PasswordAuthentication *bool
	Port                   int      // 0 = unset (default 22)
	HostKeyTypes           []string // "rsa", "ed25519", etc. (from filenames)
}

// ConsoleDevice represents a serial console parameter from the kernel cmdline.
type ConsoleDevice struct {
	Device string // "ttyS0"
	Baud   string // "115200" (empty if not specified)
}

// ConsoleInfo holds serial console detection for virtctl console readiness.
type ConsoleInfo struct {
	HasGrubDefaults bool // /etc/default/grub exists
	HasSerialGetty  bool // serial-getty@*.service in getty.target.wants

	SerialConsoles     []ConsoleDevice // from GRUB_CMDLINE_LINUX console= params
	SerialGettyDevices []string        // "ttyS0" parsed from unit names
}

// HasSerialConsole returns true if the guest has serial console configured
// either via GRUB kernel cmdline or systemd serial-getty units.
func (c *ConsoleInfo) HasSerialConsole() bool {
	return len(c.SerialConsoles) > 0 || len(c.SerialGettyDevices) > 0
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

// FileAction is a file operation performed by virt-customize (upload or write).
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

// ExecAction is a virt-customize operation (script execution).
//
// Provide either Content (inline script bytes) or ScriptPath (host-side file).
// When Content is set the commit layer writes it to a temp file automatically,
// keeping Apply free of side effects. ScriptPath is for pre-existing host files
// (e.g., user-supplied scripts from a ConfigMap mount).
type ExecAction struct {
	Type       ExecActionType
	ScriptPath string // host path passed to virt-customize --firstboot or --run
	Content    []byte // inline script content (commit layer writes to temp file)
}

// RegAction is a Windows Registry merge operation performed by virt-win-reg.
type RegAction struct {
	Content []byte // .reg file content in Windows REGEDIT format
}

// Actions collects file, registry, and exec operations returned by a plugin.
type Actions struct {
	Files []FileAction
	Regs  []RegAction
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

// GuestHandle abstracts the libguestfs operations used by probe and commit.
// Production code uses the CGO-backed implementation; tests use a mock.
type GuestHandle interface {
	IsDir(path string) (bool, error)
	IsFile(path string) (bool, error)
	Cat(path string) (string, error)
	GlobExpand(pattern string) ([]string, error)
	Ls(dir string) ([]string, error)
	ReadFile(path string) ([]byte, error)
	MkdirP(path string) error
	Upload(local, guest string) error
	Write(path string, content []byte) error
	Chmod(mode int, path string) error
	Shutdown() error
	Close()
}
