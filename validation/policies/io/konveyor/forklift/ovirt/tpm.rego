package io.konveyor.forklift.ovirt

import rego.v1

default has_tpm_os := false

has_tpm_os if {
	regex.match(`windows_2022|windows_11`, input.osType)
}

concerns contains flag if {
	has_tpm_os
	flag := {
		"id": "ovirt.tpm.required_by_os",
		"category": "Warning",
		"label": "TPM detected",
		"assessment": "The VM is detected with an operation system that must have a TPM device. TPM data is not transferred during the migration.",
	}
}
