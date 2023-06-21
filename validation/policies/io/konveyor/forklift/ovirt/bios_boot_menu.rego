package io.konveyor.forklift.ovirt

default has_boot_menu_enabled = false

has_boot_menu_enabled = value {
    value := input.bootMenuEnabled
}

concerns[flag] {
    has_boot_menu_enabled
    flag := {
        "category": "Warning",
        "label": "VM has BIOS boot menu enabled",
        "assessment": "The VM has a BIOS boot menu enabled. This is not currently supported by OpenShift Virtualization. The VM can be migrated but the BIOS boot menu will not be enabled in the target environment."
    }
}
