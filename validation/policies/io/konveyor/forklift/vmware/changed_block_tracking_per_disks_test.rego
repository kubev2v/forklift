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
                "changeTrackingEnabled": true,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 0,
            }
        ]
    }

    results := concerns with input as mock_vm
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
                "changeTrackingEnabled": false,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 0
            }
        ]
    }

    results := concerns with input as mock_vm
    count(results) == 1

}

test_with_all_disks_cbt_enabled {
    mock_vm := {
        "name": "test-multi-all-enabled",
        "disks": [
            {
                "key": 2000,
                "file": "[datastore1] vm-folder/vm1.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": true,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 0
            },
            {
                "key": 2001,
                "file": "[datastore1] vm-folder/vm2.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": true,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 1
            }
        ]
    }

    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_all_disks_cbt_disabled {
    mock_vm := {
        "name": "test-multi-all-disabled",
        "disks": [
            {
                "key": 2000,
                "file": "[datastore1] vm-folder/vm1.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": false,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 0
            },
            {
                "key": 2001,
                "file": "[datastore1] vm-folder/vm2.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": false,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 1
            }
        ]
    }

    results := concerns with input as mock_vm
    count(results) == 2
}

test_with_mixed_cbt_disks {
    mock_vm := {
        "name": "test-multi-mixed",
        "disks": [
            {
                "key": 2000,
                "file": "[datastore1] vm-folder/vm1.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": false,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 0
            },
            {
                "key": 2001,
                "file": "[datastore1] vm-folder/vm2.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": true,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 1
            },
            {
                "key": 2002,
                "file": "[datastore1] vm-folder/vm3.vmdk",
                "datastore": {
                    "id": "datastore-101",
                    "kind": "Datastore"
                },
                "changeTrackingEnabled": false,
                "controllerKey": 1000,
                "bus": "scsi",
                "unitNumber": 2
            }
        ]
    }

    results := concerns with input as mock_vm
    count(results) == 2
}