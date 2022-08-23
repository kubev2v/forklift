package io.konveyor.forklift.vmware

test_without_ballooned_memory {
    mock_vm := { "name": "test", "balloonedMemory": 0 }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_balloned_memory {
    mock_vm := { "name": "test", "balloonedMemory": 1024 }
    results := concerns with input as mock_vm
    count(results) == 1
}

