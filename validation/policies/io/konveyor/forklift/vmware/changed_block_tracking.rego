package io.konveyor.forklift.vmware

import rego.v1

change_tracking_disabled if {
	input.changeTrackingEnabled == false
}

concerns contains flag if {
	change_tracking_disabled
	flag := {
		"id": "vmware.changed_block_tracking.disabled",
		"category": "Warning",
		"label": "Changed Block Tracking (CBT) not enabled",
		"assessment": "For VM warm migration, Changed Block Tracking (CBT) must be enabled in VMware.",
	}
}
