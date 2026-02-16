# Secure Boot check for Hyper-V VMs
# Secure Boot configuration is informational - KubeVirt supports Secure Boot

package io.konveyor.forklift.hyperv

import rego.v1

default has_secure_boot := false

has_secure_boot if {
	input.secureBoot == true
}

concerns contains flag if {
	has_secure_boot
	flag := {
		"id": "hyperv.secure_boot.detected",
		"category": "Information",
		"label": "Secure Boot enabled",
		"assessment": "The VM has Secure Boot enabled. Secure Boot will be configured in the migrated VM if supported by the target.",
	}
}
