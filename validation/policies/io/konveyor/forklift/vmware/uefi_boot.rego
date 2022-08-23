package io.konveyor.forklift.vmware

has_uefi_boot {
    input.firmware == "efi"
}

concerns[flag] {
    has_uefi_boot
    flag := {
        "category": "Warning",
        "label": "UEFI secure boot detected",
        "assessment": "UEFI secure boot is enabled. NVRAM data is not copied during the migration and thus the VM might not boot on OpenShift Virtualization."
    }
}
