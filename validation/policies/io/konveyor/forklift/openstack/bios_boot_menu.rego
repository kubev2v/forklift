package io.konveyor.forklift.openstack


default has_boot_menu_enabled = false

has_boot_menu_enabled {
	input.image.properties.hw_boot_menu == "true"
}

concerns[flag] {
	has_boot_menu_enabled
	flag := {
		"category": "Information",
		"label": "VM has BIOS boot menu enabled",
		"assessment": "The VM has a BIOS boot menu enabled. This is not currently supported by OpenShift Virtualization.",
	}
}
