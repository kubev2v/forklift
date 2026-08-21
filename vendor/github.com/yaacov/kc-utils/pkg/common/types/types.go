package types

import (
	"encoding/json"
	"os"
)

func DiskSpecsFrom(disks []DiskInfo) []DiskSpec {
	out := make([]DiskSpec, 0, len(disks))
	for _, d := range disks {
		out = append(out, DiskSpec{Path: d.Path, Format: d.Format})
	}
	return out
}

func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// PipelineData is the unified envelope that accumulates stage outputs as it
// flows through the pipeline: kc-prepare → kc-convert-* → kc-finalize.
type PipelineData struct {
	Input   *PrepareInput    `json:"input,omitempty"`
	Prepare *PrepareOutput   `json:"prepare,omitempty"`
	Convert *ConverterOutput `json:"convert,omitempty"`
	Target  *TargetMeta      `json:"target,omitempty"`
}

// PrepareInput is the JSON input to kc-prepare.
type PrepareInput struct {
	Disks   []DiskSpec `json:"disks,omitempty"`
	DiskDir string     `json:"disk_dir,omitempty"` // directory of diskN.img files; used when Disks is empty
	Source  SourceSpec `json:"source"`
	// NetworkMap is provided by the orchestrator for NIC remapping at the
	// infrastructure level (e.g. Forklift/MTV). Not consumed in-guest; retained
	// so orchestrators can pass it through the pipeline JSON without losing it.
	NetworkMap []NetworkMapping `json:"network_map,omitempty"`
	LUKS       *LUKSSpec        `json:"luks,omitempty"`
	Options    PrepareOptions   `json:"options"`
}

type DiskSpec struct {
	Path   string `json:"path"`
	Format string `json:"format"`
}

type SourceSpec struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	FirmwareHint string    `json:"firmware_hint,omitempty"`
	NICs         []NICSpec `json:"nics,omitempty"`
}

type NICSpec struct {
	MAC     string `json:"mac"`
	Model   string `json:"model,omitempty"`
	Network string `json:"network,omitempty"`
}

type NetworkMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type LUKSSpec struct {
	KeyFiles map[string]string `json:"key_files,omitempty"`
	Clevis   bool              `json:"clevis,omitempty"`
}

type PrepareOptions struct {
	TmpDir                 string     `json:"tmp_dir,omitempty"`
	StaticIPs              []StaticIP `json:"static_ips,omitempty"`
	Root                   string     `json:"root,omitempty"` // "", "first" (default), "single", "/dev/..."
	Hostname               string     `json:"hostname,omitempty"`
	Timezone               string     `json:"timezone,omitempty"`
	DynamicScriptsDir      string     `json:"dynamic_scripts_dir,omitempty"`
	VMwareDriverRemoval    bool       `json:"vmware_driver_removal,omitempty"`
	WindowsRegistryNetwork bool       `json:"windows_registry_network,omitempty"`
	MultipleIPsPerNic      bool       `json:"multiple_ips_per_nic,omitempty"`
	WaitForGuestReboot     bool       `json:"wait_for_guest_reboot,omitempty"`
}

// RootCandidate is a partition or volume that appears to contain an OS root.
type RootCandidate struct {
	DevicePath  string      `json:"device_path"`
	DiskIndex   int         `json:"disk_index"`
	PartIndex   int         `json:"part_index"`
	FSType      string      `json:"fs_type,omitempty"`
	Inspect     InspectData `json:"inspect,omitempty"`
	ProductName string      `json:"product_name,omitempty"`
}

type StaticIP struct {
	MAC     string   `json:"mac"`
	IP      string   `json:"ip"`
	Gateway string   `json:"gateway,omitempty"`
	Netmask string   `json:"netmask,omitempty"`
	DNS     []string `json:"dns,omitempty"`
}

// PrepareOutput is the JSON output from kc-prepare.
type PrepareOutput struct {
	Status         string          `json:"status"`
	Error          string          `json:"error,omitempty"`
	Converter      string          `json:"converter"`
	Inspect        InspectData     `json:"inspect"`
	InspectWindows *WindowsInspect `json:"inspect_windows,omitempty"`
	Firmware       FirmwareInfo    `json:"firmware"`
	BootDevice     BootDeviceInfo  `json:"boot_device"`
	FreeSpace      []FreeSpaceInfo `json:"free_space"`
	Source         SourceSpec      `json:"source"`
	Disks          []DiskInfo      `json:"disks"`
	MountRoot      string          `json:"mount_root"`
	RootDevice     string          `json:"root_device,omitempty"`
	RootCandidates []RootCandidate `json:"root_candidates,omitempty"`
	Options        PrepareOptions  `json:"options,omitempty"`
}

type InspectData struct {
	Type           string `json:"type"`
	Distro         string `json:"distro"`
	MajorVersion   int    `json:"major_version"`
	MinorVersion   int    `json:"minor_version"`
	Arch           string `json:"arch"`
	ProductName    string `json:"product_name"`
	OsinfoID       string `json:"osinfo_id,omitempty"`
	PackageFormat  string `json:"package_format,omitempty"`
	PackageManager string `json:"package_manager,omitempty"`
	Hostname       string `json:"hostname,omitempty"`
	Apps           []App  `json:"apps,omitempty"`
	KernelVersion  string `json:"kernel_version,omitempty"`
}

type App struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type WindowsInspect struct {
	SystemRoot        string            `json:"system_root"`
	CurrentControlSet int               `json:"current_control_set"`
	SystemHive        string            `json:"system_hive"`
	SoftwareHive      string            `json:"software_hive"`
	DriveMappings     map[string]string `json:"drive_mappings"`
}

type FirmwareInfo struct {
	Type       string   `json:"type"`
	ESPDevices []string `json:"esp_devices,omitempty"`
}

type BootDeviceInfo struct {
	DiskIndex      int    `json:"disk_index"`
	PartIndex      int    `json:"part_index,omitempty"`
	BootloaderType string `json:"bootloader_type,omitempty"`
}

type FreeSpaceInfo struct {
	Path       string `json:"path"`
	FreeBytes  int64  `json:"free_bytes"`
	FreeInodes int64  `json:"free_inodes"`
}

type DiskInfo struct {
	Path       string          `json:"path"`
	SizeBytes  int64           `json:"size_bytes"`
	Format     string          `json:"format"`
	Partitions []PartitionInfo `json:"partitions,omitempty"`
}

type PartitionInfo struct {
	Index      int    `json:"index"`
	SizeBytes  int64  `json:"size_bytes"`
	FSType     string `json:"fs_type"`
	MountPoint string `json:"mount_point,omitempty"`
	DevicePath string `json:"device_path"`
}

// BlockError represents a non-fatal failure in a conversion pipeline block.
type BlockError struct {
	Block    string `json:"block"`
	Message  string `json:"message"`
	CausedBy string `json:"causedBy,omitempty"`
}

// ConverterOutput is the JSON output from kc-convert-linux or kc-convert-windows.
type ConverterOutput struct {
	GuestCaps        GuestCaps    `json:"guestcaps"`
	SELinuxRelabeled bool         `json:"selinux_relabeled,omitempty"`
	Warnings         []string     `json:"warnings,omitempty"`
	Errors           []BlockError `json:"errors,omitempty"`
}

type GuestCaps struct {
	BlockBus       string `json:"block_bus"`
	NetBus         string `json:"net_bus"`
	VirtioRNG      bool   `json:"virtio_rng"`
	VirtioBalloon  bool   `json:"virtio_balloon"`
	VirtioSocket   bool   `json:"virtio_socket"`
	ISAPVPanic     bool   `json:"isa_pvpanic"`
	MachineType    string `json:"machine_type"`
	Arch           string `json:"arch"`
	ArchMinVersion int    `json:"arch_min_version,omitempty"`
	Virtio10       bool   `json:"virtio_1_0"`
	RTCUTC         bool   `json:"rtc_utc"`
}

// TargetMeta is the JSON output from kc-finalize.
type TargetMeta struct {
	GuestCaps      GuestCaps      `json:"guestcaps"`
	TargetBuses    TargetBuses    `json:"target_buses"`
	TargetNICs     []TargetNIC    `json:"target_nics"`
	TargetFirmware string         `json:"target_firmware"`
	BootDevice     BootDeviceInfo `json:"boot_device"`
	Disks          []DiskInfo     `json:"disks"`
	Inspect        InspectData    `json:"inspect"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type TargetBuses struct {
	VirtioBlk  []BusSlot `json:"virtio_blk,omitempty"`
	SCSI       []BusSlot `json:"scsi,omitempty"`
	IDE        []BusSlot `json:"ide,omitempty"`
	Floppy     []BusSlot `json:"floppy,omitempty"`
	Removables []BusSlot `json:"removables,omitempty"`
}

type BusSlot struct {
	Index      int    `json:"index"`
	SourceDisk int    `json:"source_disk"`
	Type       string `json:"type,omitempty"`
}

type TargetNIC struct {
	MAC     string `json:"mac"`
	Model   string `json:"model"`
	Network string `json:"network,omitempty"`
}

type Family string

const (
	FamilyRHEL    Family = "rhel"
	FamilySUSE    Family = "suse"
	FamilyDebian  Family = "debian"
	FamilyALT     Family = "alt"
	FamilyUnknown Family = "unknown"
)

type FirmwareType string

const (
	FirmwareBIOS FirmwareType = "bios"
	FirmwareUEFI FirmwareType = "uefi"
)

type KernelInfo struct {
	Version    string   `json:"version"`
	Path       string   `json:"path"`
	InitrdPath string   `json:"initrd_path"`
	Modules    []string `json:"modules"`
	HasVirtio  bool     `json:"has_virtio"`
	IsXenPV    bool     `json:"is_xen_pv"`
}

// GuestDirEntry is a directory entry from guest filesystem ReadDir operations.
type GuestDirEntry struct {
	Name  string
	IsDir bool
	Mode  os.FileMode
}
