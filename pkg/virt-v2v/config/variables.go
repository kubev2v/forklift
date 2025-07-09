package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type MountPath string

// Enviroment variables
const (
	EnvLibvirtUrlName             = "V2V_libvirtURL"
	EnvFingerprintName            = "V2V_fingerprint"
	EnvInPlaceName                = "V2V_inPlace"
	EnvExtraArgsName              = "V2V_extra_args"
	EnvNewNameName                = "V2V_NewName"
	EnvVmNameName                 = "V2V_vmName"
	EnvRootDiskName               = "V2V_RootDisk"
	EnvStaticIPsName              = "V2V_staticIPs"
	EnvSourceName                 = "V2V_source"
	EnvDiskPathName               = "V2V_diskPath"
	EnvSecretKeyName              = "V2V_secretKey"
	EnvLocalMigrationName         = "LOCAL_MIGRATION"
	EnvVirtIoWinLegacyDriversName = "VIRTIO_WIN"
	EnvHostName                   = "V2V_HOSTNAME"
	EnvNbdeClevis                 = "V2V_NBDE_CLEVIS"
	EnvMultipleIpsPerNicName      = "V2V_multipleIPsPerNic"
	EnvRemoteInspection           = "V2V_remoteInspection"
	EnvRemoteInspectionDisk       = "V2V_remoteInspectDisk_"
)

const (
	OVA     = "ova"
	VSPHERE = "vSphere"
)

// Disk globs
const (
	FS    = "/mnt/disks/disk[0-9]*"
	BLOCK = "/dev/block[0-9]*"
)

// Paths
const (
	V2vOutputDir            = "/var/tmp/v2v"
	InspectionOutputFile    = V2vOutputDir + "/inspection.xml"
	VddkLib                 = "/opt/vmware-vix-disklib-distrib"
	Luksdir                 = "/etc/luks"
	VddkConfFile            = "/mnt/vddk-conf/vddk-config-file"
	DynamicScriptsMountPath = "/mnt/dynamic_scripts"

	AccessKeyId = "/etc/secret/accessKeyId"
	SecretKey   = "/etc/secret/secretKey"

	V2vInPlaceLibvirtDomain = "/tmp/input.xml"
)

type AppConfig struct {
	// V2V_libvirtURL
	LibvirtUrl string
	// V2V_fingerprint
	Fingerprint string
	// V2V_inPlace
	IsInPlace bool
	// V2V_extra_args
	ExtraArgs []string
	// LOCAL_MIGRATION
	IsLocalMigration bool
	// V2V_NewName
	NewVmName string
	// V2V_vmName
	VmName string
	// V2V_RootDisk
	RootDisk string
	// V2V_staticIPs
	StaticIPs string
	// V2V_source
	Source string
	// V2V_diskPath
	DiskPath string
	// V2V_secretKey
	SecretKey string
	// V2V_AccessKeyId
	AccessKeyId string
	// V2V_virtIoWinDrivers
	VirtIoWinLegacyDrivers string
	// hostname
	HostName string

	// V2V_remoteInspection
	IsRemoteInspection bool
	// RemoteInspectionDisks
	RemoteInspectionDisks []string

	// V2V_multipleIPsPerNic
	MultipleIpsPerNicName string
	// Paths
	VddkConfFile         string
	InspectionOutputFile string
	Luksdir              string
	NbdeClevis           bool
	DynamicScriptsDir    string
	Workdir              string
	VddkLibDir           string
	LibvirtDomainFile    string
}

func (s *AppConfig) Load() (err error) {
	s.ExtraArgs = s.getExtraArgs()
	flag.BoolVar(&s.IsLocalMigration, "local-migration", s.getEnvBool(EnvLocalMigrationName, true), "Migration is in local or remote cluster")
	flag.BoolVar(&s.IsInPlace, "in-place", s.getEnvBool(EnvInPlaceName, false), "Run virt-v2v-in-place on already populated disks")
	flag.BoolVar(&s.NbdeClevis, "nbde-clevis", s.getEnvBool(EnvNbdeClevis, false), "virt-v2v should unencrypt the disks via clevis client")
	flag.StringVar(&s.Source, "source", os.Getenv(EnvSourceName), "Source of VM ['ova','vSphere']")
	flag.StringVar(&s.LibvirtUrl, "libvirt-url", os.Getenv(EnvLibvirtUrlName), "Libvirt domain to the vSphere")
	flag.StringVar(&s.Fingerprint, "fingerprint", os.Getenv(EnvFingerprintName), "Fingerprint for the vddk")
	flag.StringVar(&s.NewVmName, "new-vm-name", os.Getenv(EnvNewNameName), "Rename the VM in virt-v2v output")
	flag.StringVar(&s.VmName, "vm-name", os.Getenv(EnvVmNameName), "Original VM name")
	flag.StringVar(&s.RootDisk, "root-disk", os.Getenv(EnvRootDiskName), "Specify which disk should be converted (default \"first\")")
	flag.StringVar(&s.StaticIPs, "static-ips", os.Getenv(EnvStaticIPsName), "Preserve static IPs, format <mac:network|bridge|ip:out>_<mac:network|bridge|ip:out>")
	flag.StringVar(&s.DiskPath, "disk-path", os.Getenv(EnvDiskPathName), "Path to the OVA disk")
	flag.StringVar(&s.AccessKeyId, "access-key", AccessKeyId, "Path to the Username for the vSphere")
	flag.StringVar(&s.SecretKey, "secret-key", SecretKey, "Path to the secret to the vSphere")
	flag.StringVar(&s.Luksdir, "luks-dir", Luksdir, "Directory path containing the luks keys")
	flag.StringVar(&s.DynamicScriptsDir, "dynamic-scripts-dir", DynamicScriptsMountPath, "Directory path to specify dynamic scripts which will edit the guest")
	flag.StringVar(&s.Workdir, "work-dir", V2vOutputDir, "Directory path to which the virt-v2v will output the disks and data")
	flag.StringVar(&s.VddkLibDir, "vddk-lib-dir", VddkLib, "Directory path containing the vddk library")
	flag.StringVar(&s.VddkConfFile, "vddk-conf-file", VddkConfFile, "Path for additional vddk configuration")
	flag.StringVar(&s.InspectionOutputFile, "inspection-output-file", InspectionOutputFile, "Path where the virt-v2v-inspector will output the metadata")
	flag.StringVar(&s.LibvirtDomainFile, "libvirt-domain-file", V2vInPlaceLibvirtDomain, "Path to the libvirt domain used in the in-place conversion")
	flag.StringVar(&s.VirtIoWinLegacyDrivers, "virtio-win-legacy-drivers", os.Getenv(EnvVirtIoWinLegacyDriversName), "Path to the virtio-win legacy drivers ISO")
	flag.StringVar(&s.HostName, "hostname", os.Getenv(EnvHostName), "Hostname of the vm")
	flag.StringVar(&s.MultipleIpsPerNicName, "multiple-ips-per-nic", os.Getenv(EnvMultipleIpsPerNicName), "Multiple IPs per NIC")
	flag.BoolVar(&s.IsRemoteInspection, "remote-inspection", s.getEnvBool(EnvRemoteInspection, false), "Run virt-v2v-inspection on remote disks")
	s.RemoteInspectionDisks = s.getRemoteInspectionDisks()
	flag.Parse()

	return s.validate()
}

func (s *AppConfig) IsVsphereMigration() bool {
	return s.Source == VSPHERE
}

func (s *AppConfig) getExtraArgs() []string {
	var extraArgs []string
	if envExtraArgs, found := os.LookupEnv(EnvExtraArgsName); found && envExtraArgs != "" {
		if err := json.Unmarshal([]byte(envExtraArgs), &extraArgs); err != nil {
			return nil
		}
	}
	return extraArgs
}

func (s *AppConfig) getRemoteInspectionDisks() []string {
	var disks []string

	envVars := os.Environ()

	for _, envVar := range envVars {
		if strings.Contains(envVar, EnvRemoteInspectionDisk) {
			disks = append(disks, strings.Split(envVar, "=")[1])
		}
	}

	return disks
}

// Get boolean.
func (s *AppConfig) getEnvBool(name string, def bool) bool {
	if s, found := os.LookupEnv(name); found {
		parsed, err := strconv.ParseBool(s)
		if err == nil {
			return parsed
		}
	}
	return def
}

func (s *AppConfig) envMissingError(env string) error {
	return fmt.Errorf("the env variable '%s' is needed for the migration", env)
}

func (s *AppConfig) validate() error {
	if !s.IsInPlace {
		switch s.Source {
		case OVA:
			if s.DiskPath == "" {
				return s.envMissingError(EnvDiskPathName)
			}
			if s.VmName == "" {
				return s.envMissingError(EnvVmNameName)
			}
		case VSPHERE:
			if s.LibvirtUrl == "" {
				return s.envMissingError(EnvLibvirtUrlName)
			}
			if s.VmName == "" {
				return s.envMissingError(EnvVmNameName)
			}
			if s.SecretKey == "" {
				return s.envMissingError(SecretKey)
			}
			if s.VirtIoWinLegacyDrivers != "" {
				if _, err := os.Stat(s.VirtIoWinLegacyDrivers); os.IsNotExist(err) {
					if unsetErr := os.Unsetenv(EnvVirtIoWinLegacyDriversName); unsetErr != nil {
						return fmt.Errorf("legacy drivers ISO not found at %s and failed to unset %s: %v",
							s.VirtIoWinLegacyDrivers, EnvVirtIoWinLegacyDriversName, unsetErr)
					}
					fmt.Fprintf(os.Stderr, "legacy drivers ISO not found at %s; environment variable %s unset\n",
						s.VirtIoWinLegacyDrivers, EnvVirtIoWinLegacyDriversName)
				}
			} else {
				if _, err := os.Stat(s.SecretKey); os.IsNotExist(err) {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid variable '%s', the valid option is 'ova' or 'vSphere'", EnvSourceName)
		}
	}
	return nil
}
