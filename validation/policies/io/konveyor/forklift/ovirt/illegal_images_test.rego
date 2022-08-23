package io.konveyor.forklift.ovirt
 
test_without_illegal_images {
    mock_vm := { "name": "test",
                 "hasIllegalImages": false
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_illegal_images {
    mock_vm := { "name": "test",
                 "hasIllegalImages": true
                }
    results = concerns with input as mock_vm
    count(results) == 1
}