package io.konveyor.forklift.vmware

default has_tpm_enabled = false

has_tpm_enabled = true {
    input.tpmEnabled == true
}

concerns[flag] {
    has_tpm_enabled
    flag := {
        "id": "vmware.tpm.detected",
        "category": "Warning",
        "label": "TPM detected",
        "assessment": "The VM is configured with a TPM device. TPM data will not be transferred during the migration."
    }
}
