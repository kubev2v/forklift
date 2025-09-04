package io.konveyor.forklift.ovirt

import rego.v1

default has_boot_menu_enabled := false

has_boot_menu_enabled := value if {
	value := input.bootMenuEnabled
}

concerns contains flag if {
	has_boot_menu_enabled
	flag := {
		"id": "ovirt.bios.boot_menu.enabled",
		"category": "Warning",
		"label": "VM has BIOS boot menu enabled",
		"assessment": "The VM has a BIOS boot menu enabled. This is not currently supported by OpenShift Virtualization. The VM can be migrated but the BIOS boot menu will not be enabled in the target environment.",
	}
}
