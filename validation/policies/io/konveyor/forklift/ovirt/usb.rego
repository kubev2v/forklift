package io.konveyor.forklift.ovirt

default has_usb_enabled = false

has_usb_enabled = value {
   value :=  input.usbEnabled
}

concerns[flag] {
    has_usb_enabled
    flag := {
        "category": "Warning",
        "label": "USB support enabled",
        "assessment": "The VM has USB support enabled, but USB device attachment is not currently supported by OpenShift Virtualization."
    }
}
