package io.konveyor.forklift.openstack

import rego.v1

validate := {
	"rules_version": RULES_VERSION,
	"errors": errors,
	"concerns": concerns,
}

errors contains message if {
	not valid_vm_string
	message := "No VM name found in input body"
}
