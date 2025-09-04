package io.konveyor.forklift.vmware

import rego.v1

default has_tpm_enabled := false

has_tpm_enabled if {
	input.tpmEnabled == true
}

concerns contains flag if {
	has_tpm_enabled
	flag := {
		"id": "vmware.tpm.detected",
		"category": "Warning",
		"label": "TPM detected",
		"assessment": "The VM is configured with a TPM device. TPM data will not be transferred during the migration.",
	}
}
