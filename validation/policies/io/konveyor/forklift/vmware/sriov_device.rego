package io.konveyor.forklift.vmware

import rego.v1

has_sriov_device if {
	some i
	input.devices[i].kind == "VirtualSriovEthernetCard"
}

concerns contains flag if {
	has_sriov_device
	flag := {
		"id": "vmware.device.sriov.detected",
		"category": "Warning",
		"label": "SR-IOV passthrough adapter configuration detected",
		"assessment": "SR-IOV passthrough adapter configuration is not currently supported by Migration Toolkit for Virtualization. Administrators can configure this after migration.",
	}
}
