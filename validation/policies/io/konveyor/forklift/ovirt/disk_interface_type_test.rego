package io.konveyor.forklift.ovirt

# These tests have both interface, disk.storageType & disk.status included to satisfy the disk_storage_type, disk_status and disk_interface_type tests
# diskAttachmennt.id is included to satisfy the number_of_disks rule in disk_interface_type.rego

test_with_first_valid_disk_interface_type {
    mock_vm := {
        "name": "test",
        "diskAttachments": [
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e21",
              "interface": "sata",
              "disk":
                { "storageType": "image",
                  "status": "ok"
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_second_valid_disk_interface_type {
    mock_vm := {
        "name": "test",
        "diskAttachments": [
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e21",
              "interface": "virtio_scsi",
              "disk":
                { "storageType": "image",
                  "status": "ok"
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_third_valid_disk_interface_type {
    mock_vm := {
        "name": "test",
        "diskAttachments": [
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e21",
              "interface": "virtio",
              "disk":
                { "storageType": "image",
                  "status": "ok"
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

test_with_invalid_disk_interface_type {
    mock_vm := {
        "name": "test",
        "diskAttachments": [
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e21",
              "interface": "virtio_scsi",
              "disk":
                { "storageType": "image" }
            },
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e22",
              "interface": "raw",
              "disk":
                { "storageType": "image" }
            },
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e23",
              "interface": "virtio",
              "disk":
                { "storageType": "image" }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}