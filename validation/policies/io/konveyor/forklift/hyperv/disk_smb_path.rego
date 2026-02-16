# Disk SMB path validation for Hyper-V VMs
# Ensures disk paths can be mapped to SMB share for migration

package io.konveyor.forklift.hyperv

import rego.v1

# Find disks with missing SMB path
disks_missing_smb_path contains idx if {
	some idx
	disk := input.disks[idx]
	not has_smb_path(disk)
}

has_smb_path(disk) if {
	is_string(disk.smbPath)
	disk.smbPath != ""
}

concerns contains flag if {
	disks_missing_smb_path[idx]
	disk := input.disks[idx]
	flag := {
		"id": "hyperv.disk.smb_path.missing",
		"category": "Warning",
		"label": sprintf("Disk '%v' cannot be mapped to SMB share", [disk.name]),
		"assessment": sprintf("Cannot map Windows path '%v' to SMB share. Ensure the disk is located on a configured SMB share.", [disk.windowsPath]),
	}
}
