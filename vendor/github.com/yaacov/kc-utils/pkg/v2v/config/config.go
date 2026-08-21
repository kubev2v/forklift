package config

const (
	EnvLibvirtURL                   = "V2V_libvirtURL"
	EnvInPlace                      = "V2V_inPlace"
	EnvExtraArgs                    = "V2V_extra_args"
	EnvVmName                       = "V2V_vmName"
	EnvNewName                      = "V2V_NewName"
	EnvRootDisk                     = "V2V_RootDisk"
	EnvStaticIPs                    = "V2V_staticIPs"
	EnvSource                       = "V2V_source"
	EnvDiskPath                     = "V2V_diskPath"
	EnvFirmware                     = "V2V_firmware"
	EnvLocalMigration               = "LOCAL_MIGRATION"
	EnvHostName                     = "V2V_HOSTNAME"
	EnvNbdeClevis                   = "V2V_NBDE_CLEVIS"
	EnvMultipleIPsPerNic            = "V2V_multipleIPsPerNic"
	EnvVsphereVmwareDriverRemoval   = "V2V_vsphereVmwareDriverRemoval"
	EnvWindowsRegistryNetworkConfig = "V2V_windowsRegistryNetworkConfig"
	EnvWaitForGuestReboot           = "V2V_waitForGuestReboot"
	EnvOverlayEnabled               = "V2V_overlayEnabled"
	EnvFingerprint                  = "V2V_fingerprint"
	EnvCopyConcurrency              = "V2V_copyConcurrency"
	EnvOffline                      = "V2V_offline"
	EnvBackend                      = "V2V_backend"

	DefaultCopyConcurrency = 4
	// Forklift conversion-pod paths (not env-configurable).
	// DefaultCaBundle is the virt-v2v symlink target used by LinkCertificates.
	// DefaultCaCert is the mounted provider PEM path used for TLS and as the symlink source.
	DefaultCaBundle = "/opt/ca-bundle.crt"
	DefaultCaCert   = "/etc/secret/cacert"

	DefaultWorkdir              = "/var/tmp/v2v"
	DefaultInspectionOutputFile = DefaultWorkdir + "/inspection.xml"
	DefaultDynamicScriptsDir    = "/mnt/dynamic_scripts"
	DefaultLuksDir              = "/etc/luks"
	DefaultMountRoot            = "/tmp/kc-guest"

	BlockGlob = "/dev/block[0-9]*"
	FSGlob    = "/mnt/disks/disk[0-9]*"
)

// Config mirrors Forklift AppConfig fields used by kc-v2v.
type Config struct {
	LibvirtURL                   string
	IsInPlace                    bool
	IsLocalMigration             bool
	ExtraArgs                    []string
	VmName                       string
	NewVmName                    string
	RootDisk                     string
	StaticIPs                    string
	Source                       string
	DiskPath                     string
	Firmware                     string
	HostName                     string
	NbdeClevis                   bool
	MultipleIPsPerNic            bool
	VsphereVmwareDriverRemoval   bool
	WindowsRegistryNetworkConfig bool
	WaitForGuestReboot           bool
	OverlayEnabled               bool
	Fingerprint                  string
	CopyConcurrency              int

	Offline              bool
	Backend              string
	LuksDir              string
	DynamicScriptsDir    string
	Workdir              string
	InspectionOutputFile string
	MountRoot            string
	LogLevel             string
	BinDir               string
}
