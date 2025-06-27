package io.konveyor.forklift.vmware

has_nvme_bus {
    some i
    input.disks[i].bus == "nvme"
}

concerns[flag] {
    has_nvme_bus
    flag := {
        "id": "vmware.disk.nvme.detected",
        "category": "Critical",
        "label": "Disk NVMe was detcted",
        "assessment": "NVMe disks are not currently supported by MTV. The VM cannot be migrated"
    }
}
