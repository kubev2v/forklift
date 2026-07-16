package io.konveyor.forklift.openstack

import rego.v1

test_disk_count_at_limit if {
	volumes := [{"name": sprintf("vol%d", [i]), "size": 16} | some i in numbers.range(1, 165)]
	test_input := {"volumes": volumes}

	results := concerns with input as test_input
	not has_too_many_disks with input as test_input
}

test_disk_count_exceeds_limit if {
	volumes := [{"name": sprintf("vol%d", [i]), "size": 16} | some i in numbers.range(1, 166)]
	test_input := {"volumes": volumes}

	results := concerns with input as test_input
	has_too_many_disks with input as test_input
}

test_disk_count_well_under_limit if {
	test_input := {"volumes": [
		{"name": "vol1", "size": 16},
		{"name": "vol2", "size": 16},
	]}

	results := concerns with input as test_input
	not has_too_many_disks with input as test_input
}
