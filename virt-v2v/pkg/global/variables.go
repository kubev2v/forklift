package global

type MountPath string

const (
	OVA               = "ova"
	VSPHERE           = "vSphere"
	DIR               = "/var/tmp/v2v"
	FS      MountPath = "/mnt/disks/disk[0-9]*"
	BLOCK   MountPath = "/dev/block[0-9]*"
	VDDK              = "/opt/vmware-vix-disklib-distrib"
	LUKSDIR           = "/etc/luks"

	LETTERS        = "abcdefghijklmnopqrstuvwxyz"
	LETTERS_LENGTH = len(LETTERS)
)
