package io.konveyor.forklift.ovirt

nic_set_to_pci_passthrough [i] {
    some i
    regex.match(`pci_passthrough`, input.nics[i].interface)
}

concerns[flag] {
    count(nic_set_to_pci_passthrough) > 0
    flag := {
        "category": "Warning",
        "label": "NIC with host device passthrough detected",
        "assessment": "The VM is using a vNIC profile configured for host device passthrough, which is not currently supported by OpenShift Virtualization. The VM will be configured with an SRIOV NIC, but the destination network will need to be set up correctly."
    }
}

