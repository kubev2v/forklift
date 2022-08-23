package io.konveyor.forklift.ovirt
 
test_without_iothreads_enabled {
    mock_vm := { "name": "test",
                 "ioThreads": 1
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_iothreads_enabled {
    mock_vm := { "name": "test",
                 "ioThreads": 3
                }
    results = concerns with input as mock_vm
    count(results) == 1
}