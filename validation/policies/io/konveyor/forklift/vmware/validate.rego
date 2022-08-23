package io.konveyor.forklift.vmware

validate = {
    "rules_version": RULES_VERSION,
    "errors": errors,
    "concerns": concerns
}

errors[message] {
    not valid_vm
    message := "No VM name found in input body"
}