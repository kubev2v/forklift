package io.konveyor.forklift.vmware

has_uefi_boot {
    input.firmware == "efi"
}

concerns[flag] {
    has_uefi_boot
    flag := {
        "category": "Warning",
        "label": "UEFI detected",
        "assessment": "UEFI secure boot will be disabled on Openshift Virtualization. If the VM was set with UEFI secure boot, manual steps within the guest would be needed for the guest operating system to boot."
    }
}
