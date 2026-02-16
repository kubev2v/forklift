package io.konveyor.forklift.hyperv

import rego.v1

# Check if any disk has RCT disabled
rct_disabled if {
	some disk in input.disks
	disk.rctEnabled == false
}

concerns contains flag if {
	rct_disabled
	flag := {
		"id": "hyperv.resilient_change_tracking.disabled",
		"category": "Warning",
		"label": "Resilient Change Tracking (RCT) not enabled",
		"assessment": "For VM warm migration, Resilient Change Tracking (RCT) must be enabled on all disks in Hyper-V.",
	}
}
