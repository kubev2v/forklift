package io.konveyor.forklift.ovirt

test_with_i440fx_sea_bios {
    mock_vm := {
        "name": "test",
        "bios": "i440fx_sea_bios"
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_q35_secure_boot_bios {
    mock_vm := {
        "name": "test",
        "bios": "q35_secure_boot"
    }
    results := concerns with input as mock_vm
    count(results) == 1
}
