package io.konveyor.forklift.ovirt

import rego.v1

test_disk_count_at_limit if {
	attachments := [{"disk": {"provisionedSize": 17179869184}} | some i in numbers.range(1, 165)]
	test_input := {"diskAttachments": attachments}

	results := concerns with input as test_input
	not has_too_many_disks with input as test_input
}

test_disk_count_exceeds_limit if {
	attachments := [{"disk": {"provisionedSize": 17179869184}} | some i in numbers.range(1, 166)]
	test_input := {"diskAttachments": attachments}

	results := concerns with input as test_input
	has_too_many_disks with input as test_input
}

test_disk_count_well_under_limit if {
	test_input := {"diskAttachments": [
		{"disk": {"provisionedSize": 17179869184}},
		{"disk": {"provisionedSize": 17179869184}},
	]}

	results := concerns with input as test_input
	not has_too_many_disks with input as test_input
}
