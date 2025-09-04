package io.konveyor.forklift.vmware

import rego.v1

test_has_nvme_bus if {
	mock_vm := {
		"name": "test",
		"disks": [
			{"bus": "scsi"},
			{"bus": "nvme"},
		],
	}

	results := concerns with input as mock_vm
	count(results) == 1
}

test_has_no_nvme_bus if {
	mock_vm := {
		"name": "test",
		"disks": [{"bus": "scsi"}],
	}

	results := concerns with input as mock_vm
	count(results) == 0
}
