package io.konveyor.forklift.ovirt
 
test_without_cpushares_enabled {
    mock_vm := { "name": "test",
                 "cpuShares": 0
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_cpushares_enabled {
    mock_vm := { "name": "test",
                 "cpuShares": 3
                }
    results = concerns with input as mock_vm
    count(results) == 1
}