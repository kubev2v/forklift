package io.konveyor.forklift.ovirt

# These tests have both interface, disk.storageType & disk.status included to satisfy the disk_storage_type, disk_status and disk_interface_type tests
# diskAttachmennt.id is included to satisfy the number_of_disks rule in disk_interface_type.rego

test_with_valid_storage_type {
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

test_with_invalid_storage_type {
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
            },
            {
              "id": "b749c132-bb97-4145-b86e-a1751cf75e22",
              "interface": "virtio_scsi",
              "disk":
                { "storageType": "raw",
                  "status": "ok"
                }
            }
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}