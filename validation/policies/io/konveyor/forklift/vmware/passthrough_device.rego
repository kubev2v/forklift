package io.konveyor.forklift.vmware

has_passthrough_device {
    some i
    input.devices[i].kind == "VirtualPCIPassthrough"
}

concerns[flag] {
    has_passthrough_device
    flag := {
        "id": "vmware.passthrough_device.detected",
        "category": "Critical",
        "label": "Passthrough device detected",
        "assessment": "SCSI or PCI passthrough devices are not currently supported by Migration Toolkit for Virtualization. The VM cannot be migrated unless the passthrough device is removed."
    }
}
