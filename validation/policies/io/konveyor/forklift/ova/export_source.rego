package io.konveyor.forklift.ova

import rego.v1

# Inventory exposes exportSource (camelCase JSON from the web API).
# Keep ovaSource as a fallback for older test fixtures.
export_source := input.exportSource if {
	input.exportSource
} else := input.ovaSource if {
	input.ovaSource
} else := "Unknown"

supported_export_source if {
	export_source == "VMware"
}

supported_export_source if {
	export_source == "Nutanix"
}

unsupported_export_source if {
	not supported_export_source
}

concerns contains flag if {
	unsupported_export_source
	flag := {
		"id": "ova.source.unsupported",
		"category": "Warning",
		"label": "Unsupported OVA source",
		"assessment": "This OVA may not have been exported from a supported source (VMware or Nutanix), and may have issues during import.",
	}
}
