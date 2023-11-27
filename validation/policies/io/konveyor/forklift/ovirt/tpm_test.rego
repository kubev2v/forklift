package io.konveyor.forklift.ovirt
 
test_without_tpm_enabled {
    mock_vm := { "name": "test",
                 "osType": "rhel_9x64"
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_tpm_enabled_w11 {
    mock_vm := { "name": "test",
                 "osType": "windows_11"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}

test_with_tpm_enabled_w2k22 {
    mock_vm := { "name": "test",
                 "osType": "windows_2022"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}