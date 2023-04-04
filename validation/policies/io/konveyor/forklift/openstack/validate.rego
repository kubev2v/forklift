package io.konveyor.forklift.openstack

validate = {
	"rules_version": RULES_VERSION,
	"errors": errors,
	"concerns": concerns,
}

errors[message] {
	not valid_vm_string
	message := "No VM name found in input body"
}
