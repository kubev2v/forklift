package io.konveyor.forklift.ovirt
 
test_with_auto_resume {
    mock_vm := { "name": "test",
                 "storageErrorResumeBehaviour": "auto_resume"
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_without_auto_resume {
    mock_vm := { "name": "test",
                 "storageErrorResumeBehaviour": "pause"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}