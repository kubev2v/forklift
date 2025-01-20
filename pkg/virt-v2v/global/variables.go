package global

type MountPath string

const (
	OVA                  = "ova"
	VSPHERE              = "vSphere"
	DIR                  = "/var/tmp/v2v"
	INSPECTION           = "/var/tmp/v2v/inspection.xml"
	FS         MountPath = "/mnt/disks/disk[0-9]*"
	BLOCK      MountPath = "/dev/block[0-9]*"
	VDDK                 = "/opt/vmware-vix-disklib-distrib"
	LUKSDIR              = "/etc/luks"

	WIN_FIRSTBOOT_PATH         = "/Program Files/Guestfs/Firstboot"
	WIN_FIRSTBOOT_SCRIPTS_PATH = "/Program Files/Guestfs/Firstboot/scripts"
	DYNAMIC_SCRIPTS_MOUNT_PATH = "/mnt/dynamic_scripts"
	WINDOWS_DYNAMIC_REGEX      = `^([0-9]+_win_firstboot(([\w\-]*).ps1))$`
	LINUX_DYNAMIC_REGEX        = `^([0-9]+_linux_(run|firstboot)(([\w\-]*).sh))$`
	SHELL_SUFFIX               = ".sh"

	LETTERS        = "abcdefghijklmnopqrstuvwxyz"
	LETTERS_LENGTH = len(LETTERS)
)
