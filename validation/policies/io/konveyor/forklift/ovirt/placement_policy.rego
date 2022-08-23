package io.konveyor.forklift.ovirt

default warn_placement_policy = false

warn_placement_policy = true {
    regex.match(`\bmigratable\b`, input.placementPolicyAffinity)
}

concerns[flag] {
    warn_placement_policy
    flag := {
        "category": "Warning",
        "label": "Placement policy affinity",
        "assessment": "The VM has a placement policy affinity setting that requires live migration to be enabled in OpenShift Virtualization for compatibility. The target storage classes must also support RWX access mode."
    }
}
