package io.konveyor.forklift.ova

import rego.v1

# Inventory exposes exportSource (camelCase JSON from the web API).
# Keep ovaSource as a fallback for older fixtures. Empty when both are absent
# so other OVA policies are not forced to set a source.
export_source := object.get(input, "exportSource", object.get(input, "ovaSource", ""))

supported_export_source if {
	export_source == "VMware"
}

supported_export_source if {
	export_source == "Nutanix"
}

unsupported_export_source if {
	export_source != ""
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
