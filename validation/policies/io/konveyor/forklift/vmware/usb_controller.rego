package io.konveyor.forklift.vmware

has_usb_controller {
    some i
    input.devices[i].kind == "VirtualUSBController"
}

concerns[flag] {
    has_usb_controller
    flag := {
        "category": "Warning",
        "label": "USB controller detected",
        "assessment": "USB controllers are not currently supported by OpenShift Virtualization. The VM can be migrated but the devices attached to the USB controller will not be migrated."
    }
}
