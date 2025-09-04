package io.konveyor.forklift.ovirt

import rego.v1

default secure_boot_enabled := false

secure_boot_enabled if {
	input.bios == "q35_secure_boot"
}

concerns contains flag if {
	secure_boot_enabled
	flag := {
		"id": "ovirt.secure_boot.detected",
		"category": "Warning",
		"label": "UEFI secure boot detected",
		"assessment": "UEFI secure boot is currently only partially supported by OpenShift Virtualization. Some functionality may be missing after the VM is migrated.",
	}
}
