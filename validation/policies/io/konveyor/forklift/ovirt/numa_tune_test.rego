package io.konveyor.forklift.ovirt
 
test_without_numa_affinity {
    mock_vm := { "name": "test", "numaNodeAffinity": [] }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_numa_affinity {
    mock_vm := { "name": "test", "numaNodeAffinity": [0,2] }
    results = concerns with input as mock_vm
    count(results) == 1
}
