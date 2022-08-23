package io.konveyor.forklift.ovirt
 
test_without_host_devices {
    mock_vm := { "name": "test", "hostDevices": [] }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_host_devices {
    mock_vm := { "name": "test",
                 "hostDevices": [
                    { "capability": "thing" }
                  ]
                }
    results = concerns with input as mock_vm
    count(results) == 1
}
