package io.konveyor.forklift.ova

import rego.v1

unsupported_export_source if {
	input.ovaSource != "VMware"
}

concerns contains flag if {
	unsupported_export_source
	flag := {
		"id": "ova.source.unsupported",
		"category": "Warning",
		"label": "Unsupported OVA source",
		"assessment": "This OVA may not have been exported from a VMware source, and may have issues during import.",
	}
}
