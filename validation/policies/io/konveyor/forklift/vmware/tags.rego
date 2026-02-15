package io.konveyor.forklift.vmware

import rego.v1

valid_key_no_prefix_regex := `^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?$`
valid_key_with_prefix_regex := `^([a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?)/([a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?)$`
valid_value_regex := `^$|^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,61}[a-zA-Z0-9])?$`

max_name_length_no_prefix := 63
max_name_length_with_prefix := 253
max_value_length := max_name_length_no_prefix

valid_tag_description(description) if {
    regex.match(valid_value_regex, description)
    count(description) <= max_value_length
}

has_prefix(name) if {
    regex.match(valid_key_with_prefix_regex, name)
}

valid_tag_name_with_prefix(name) if {
    regex.match(valid_key_with_prefix_regex, name)
    count(name) <= max_name_length_with_prefix
}

valid_tag_name_no_prefix(name) if {
    regex.match(valid_key_no_prefix_regex, name)
    count(name) <= max_name_length_no_prefix
}

valid_tag_name(name) if {
    has_prefix(name)
    valid_tag_name_with_prefix(name)
}

valid_tag_name(name) if {
    not has_prefix(name)
    valid_tag_name_no_prefix(name)
}

# Tag name invalid for K8s labels (will be sanitized)
concerns contains flag if {
    some t in input.tags
    not valid_tag_name(t.name)

    flag := {
        "category": "Warning",
        "label": sprintf("Tag Name Will Be Sanitized: '%s'", [t.name]),
        "assessment": "Tag name contains characters that are not valid for Kubernetes labels. The name will be automatically sanitized during migration. Valid names must start and end with alphanumeric characters, can only contain alphanumeric characters, hyphens (-), underscores (_), and periods (.), and be 63 characters or fewer."
    }
}

# Tag description invalid for K8s label values (will be sanitized)
concerns contains flag if {
    some t in input.tags
    not valid_tag_description(t.description)

    flag := {
        "category": "Warning",
        "label": sprintf("Tag Description Will Be Sanitized: '%s'", [t.description]),
        "assessment": "Tag description contains characters that are not valid for Kubernetes label values. The description will be automatically sanitized during migration. Valid values must start and end with alphanumeric characters, can only contain alphanumeric characters, hyphens (-), underscores (_), and periods (.), and be 63 characters or fewer."
    }
}
