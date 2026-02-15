package io.konveyor.forklift.vmware

import rego.v1

annotation_prefix := "vsphere.forklift.konveyor.io/"

# Annotation name part: must start/end with alphanumeric, max 63 chars.
valid_annotation_name_regex := `^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?$`

max_annotation_name_length := 63
max_recommended_value_length := 262144  # 256KB

get_custom_attr_name(key) := name if {
    some def in input.customDef
    def.key == key
    name := def.name
}

valid_custom_attr_name(name) if {
    regex.match(valid_annotation_name_regex, name)
    count(name) <= max_annotation_name_length
}

valid_custom_attr_value(value) if {
    count(value) <= max_recommended_value_length
}

# Attribute name invalid for annotation key (will be sanitized)
concerns contains flag if {
    some cv in input.customValues
    name := get_custom_attr_name(cv.key)
    not valid_custom_attr_name(name)

    flag := {
        "category": "Warning",
        "label": sprintf("Custom Attribute Name Will Be Sanitized: '%s'", [name]),
        "assessment": "Custom attribute name contains characters that are not valid for Kubernetes annotation keys. Annotation keys follow the same rules as label keys: must start and end with alphanumeric characters, can only contain alphanumeric characters, hyphens (-), underscores (_), and periods (.), and be 63 characters or fewer. The name will be automatically sanitized during migration."
    }
}

# Attribute value exceeds 256KB annotation limit
concerns contains flag if {
    some cv in input.customValues
    not valid_custom_attr_value(cv.value)
    name := get_custom_attr_name(cv.key)

    flag := {
        "category": "Warning",
        "label": sprintf("Custom Attribute Value Too Long: '%s'", [name]),
        "assessment": sprintf("Custom attribute '%s' has a value exceeding 256KB. Kubernetes annotations have a size limit. The value may be truncated during migration.", [name])
    }
}
