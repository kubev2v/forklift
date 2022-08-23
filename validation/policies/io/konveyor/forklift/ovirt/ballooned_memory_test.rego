package io.konveyor.forklift.ovirt
 
test_without_ballooned_memory {
    mock_vm := { "name": "test",
                 "balloonedMemory": false
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_ballooned_memory {
    mock_vm := { "name": "test",
                 "balloonedMemory": true
                }
    results = concerns with input as mock_vm
    count(results) == 1
}