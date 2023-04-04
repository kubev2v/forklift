package io.konveyor.forklift.openstack

default secure_boot_enabled = false

secure_boot_enabled {
	input.image.properties.os_secure_boot == "required"
}

secure_boot_enabled {
	input.flavor.extraSpecs["os:secure_boot"] == "required"
}

concerns[flag] {
	secure_boot_enabled
	flag := {
		"category": "Warning",
		"label": "UEFI secure boot detected",
		"assessment": "UEFI secure boot is currently only partially supported by OpenShift Virtualization. Some functionality may be missing after the VM is migrated.",
	}
}
