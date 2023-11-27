package io.konveyor.forklift.vmware

test_with_tpm_disabled {
    mock_vm := {
        "name": "test",
        "tpmEnabled": false,
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_cpu_hot_add_enabled {
    mock_vm := {
        "name": "test",
        "tpmEnabled": true
    }
    results := concerns with input as mock_vm
    count(results) == 1
}