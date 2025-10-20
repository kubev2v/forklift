package io.konveyor.forklift.vmware

import rego.v1

test_with_no_device if {
	mock_vm := {
		"name": "test",
		"devices": [],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_other_xyz_device if {
	mock_vm := {
		"name": "test",
		"devices": [{"kind": "VirtualXYZEthernetCard"}],
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_pci_passthrough_device if {
	mock_vm := {
		"name": "test",
		"devices": [{"kind": "VirtualPCIPassthrough"}],
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
