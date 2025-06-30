package io.konveyor.forklift.vmware

is_vm_powered_off {
    input.powerState == "poweredOff"
}

concerns[flag] {
    is_vm_powered_off
    flag := {
        "id": "vmware.vm_powered_off.detected",
        "category": "Warning",
        "label": "VM is powered off - Static IP preservation requires the VM to be powered on",
        "assessment": "Static IP preservation requires the VM to be powered on."
    }
}