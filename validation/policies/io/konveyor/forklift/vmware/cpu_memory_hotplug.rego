package io.konveyor.forklift.vmware

default has_hotplug_enabled = false

has_hotplug_enabled = true {
    input.cpuHotAddEnabled == true
}

has_hotplug_enabled = true {
    input.cpuHotRemoveEnabled == true
}

has_hotplug_enabled = true {
    input.memoryHotAddEnabled == true
}

concerns[flag] {
    has_hotplug_enabled
    flag := {
        "id": "vmware.cpu_memory.hotplug.enabled",
        "category": "Warning",
        "label": "CPU/Memory hotplug detected",
        "assessment": "Hot pluggable CPU or memory is not currently supported by Migration Toolkit for Virtualization. You can reconfigure CPU or memory after migration."
    }
}
