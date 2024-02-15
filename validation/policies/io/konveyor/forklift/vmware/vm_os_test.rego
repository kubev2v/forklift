package io.konveyor.forklift.vmware
 
test_unsupported_el6_64 {
    mock_vm := { "name": "test",
                 "guestId": "rhel6_64Guest"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}

test_supported_el7 {
    mock_vm := { "name": "test",
                 "guestId": "rhel7_64Guest"
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_unsupported_el6 {
    mock_vm := { "name": "test",
                 "guestId": "rhel6Guest"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}

test_supported_windows {
    mock_vm := { "name": "test",
                 "guestId": "windows11_64Guest"
                }
    results = concerns with input as mock_vm
    count(results) == 0
}