package io.konveyor.forklift.ovirt
 
test_without_customcpu {
    mock_vm := { "name": "test" }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_customcpu {
    mock_vm := { "name": "test",
                 "customCpuModel": "Icelake-Server-noTSX,-mpx"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}