package io.konveyor.forklift.vmware

has_sriov_device {
    some i
    input.devices[i].kind == "VirtualSriovEthernetCard"
}

concerns[flag] {
    has_sriov_device
    flag := {
        "id": "vmware.device.sriov.detected",
        "category": "Warning",
        "label": "SR-IOV passthrough adapter configuration detected",
        "assessment": "SR-IOV passthrough adapter configuration is not currently supported by Migration Toolkit for Virtualization. Administrators can configure this after migration."
    }
}
