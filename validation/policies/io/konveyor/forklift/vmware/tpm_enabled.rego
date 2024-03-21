package io.konveyor.forklift.vmware

default has_tpm_enabled = false

has_tpm_enabled = true {
    input.tpmEnabled == true
}

concerns[flag] {
    has_tpm_enabled
    flag := {
        "category": "Warning",
        "label": "TPM detected",
        "assessment": "The VM is configured with a TPM device. TPM data is not transferred during the migration."
    }
}
