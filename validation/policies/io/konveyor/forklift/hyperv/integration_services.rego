# Integration Services check for Hyper-V VMs
# Integration Services (KVP) provides guest OS info and network data needed for static IP preservation

package io.konveyor.forklift.hyperv

import rego.v1

default is_powered_on := false

is_powered_on if {
	input.powerState == "On"
}

has_guest_os if {
	is_string(input.guestOS)
	input.guestOS != ""
}

is_missing_integration_services if {
	is_powered_on
	not has_guest_os
}

concerns contains flag if {
	is_missing_integration_services
	flag := {
		"id": "hyperv.integration_services.not_detected",
		"category": "Warning",
		"label": "Hyper-V Integration Services not detected",
		"assessment": "Guest OS info and static IP preservation may not be available. Ensure Integration Services is running in the guest VM.",
	}
}
