package io.konveyor.forklift.vmware

has_sriov_device {
    some i
    input.devices[i].kind == "VirtualSriovEthernetCard"
}

concerns[flag] {
    has_sriov_device
    flag := {
        "category": "Critical",
        "label": "SR-IOV passthrough adapter configuration detected",
        "assessment": "SR-IOV passthrough adapter configuration is not currently supported by OpenShift Virtualization."
    }
}
