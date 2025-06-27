package io.konveyor.forklift.ova

unsupported_export_source {
    input.ovaSource != "VMware"
}

concerns[flag] {
    unsupported_export_source
    flag := {
        "id": "ova.source.unsupported",
        "category": "Warning",
        "label": "Unsupported OVA source",
        "assessment": "This OVA may not have been exported from a VMware source, and may have issues during import."
    }
}
