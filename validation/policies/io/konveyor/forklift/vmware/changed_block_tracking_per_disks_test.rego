package io.konveyor.forklift.vmware

test_with_cbt_enabled_disk {
     mock_vm := {
        "name": "test",
        "disks": [
            {
                "key": 2000,
                "file": "[datastore1] vm-folder/vm.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": true
            }
        ]
    }

    result := oncerns with input as mock_vm
    count(results) == 0
    
}

test_with_cbt_disabled_disk {
     mock_vm := {
        "name": "test",
        "disks": [
            {
                "key": 2000,
                "file": "[datastore1] vm-folder/vm.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": false
            }
        ]
    }

    result := oncerns with input as mock_vm
    count(results) == 1

}