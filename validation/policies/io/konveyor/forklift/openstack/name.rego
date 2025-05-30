package io.konveyor.forklift.openstack

default valid_input = true

valid_input = false {
	is_null(input)
}

default valid_vm_string = false

valid_vm_string {
	is_string(input.name)
}

default valid_vm_name = false

valid_vm_name {
	regex.match("^(([A-Za-z0-9][-A-Za-z0-9.]*)?[A-Za-z0-9])?$", input.name)
	count(input.name) < 64
}

concerns[flag] {
	valid_input
	valid_vm_string
	not valid_vm_name
	flag := {
		"category": "Warning",
		"label": "Invalid VM Name",
		"assessment": "The VM name does not comply with the DNS subdomain name format. Edit the name or it will be renamed automatically during the migration to meet RFC 1123. The VM name must be a maximum of 63 characters containing lowercase letters (a-z), numbers (0-9), periods (.), and hyphens (-). The first and last character must be a letter or number. The name cannot contain uppercase letters, spaces or special characters.",
	}
}
