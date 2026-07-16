package io.konveyor.forklift.vmware

import rego.v1

test_disk_count_at_limit if {
	disks := [{"file": sprintf("disk%d.vmdk", [i]), "capacity": 17179869184} | some i in numbers.range(1, 165)]
	test_input := {"disks": disks}

	results := concerns with input as test_input
	not has_too_many_disks with input as test_input
}

test_disk_count_exceeds_limit if {
	disks := [{"file": sprintf("disk%d.vmdk", [i]), "capacity": 17179869184} | some i in numbers.range(1, 166)]
	test_input := {"disks": disks}

	results := concerns with input as test_input
	has_too_many_disks with input as test_input
}

test_disk_count_well_under_limit if {
	test_input := {"disks": [
		{"file": "disk1.vmdk", "capacity": 17179869184},
		{"file": "disk2.vmdk", "capacity": 17179869184},
	]}

	results := concerns with input as test_input
	not has_too_many_disks with input as test_input
}
