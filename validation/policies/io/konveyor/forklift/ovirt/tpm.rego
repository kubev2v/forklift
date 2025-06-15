package io.konveyor.forklift.ovirt

default has_tpm_os = false

has_tpm_os = true {
    regex.match(`windows_2022|windows_11`, input.osType)
}

concerns[flag] {
    has_tpm_os
    flag := {
        "id": "ovirt.tpm.required_by_os",
        "category": "Warning",
        "label": "TPM detected",
        "assessment": "The VM is detected with an operation system that must have a TPM device. TPM data is not transferred during the migration."
    }
}
