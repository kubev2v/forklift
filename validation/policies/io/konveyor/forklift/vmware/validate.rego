package io.konveyor.forklift.vmware

import rego.v1

validate := {
	"rules_version": RULES_VERSION,
	"errors": errors,
	"concerns": concerns,
}

errors contains message if {
	not valid_vm
	message := "No VM name found in input body"
}
