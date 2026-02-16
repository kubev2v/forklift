# TPM check for Hyper-V VMs
# TPM (Trusted Platform Module) data cannot be migrated

package io.konveyor.forklift.hyperv

import rego.v1

default has_tpm_enabled := false

has_tpm_enabled if {
	input.tpmEnabled == true
}

concerns contains flag if {
	has_tpm_enabled
	flag := {
		"id": "hyperv.tpm.detected",
		"category": "Warning",
		"label": "TPM detected",
		"assessment": "The VM is configured with a TPM device. TPM data will not be transferred during the migration.",
	}
}
