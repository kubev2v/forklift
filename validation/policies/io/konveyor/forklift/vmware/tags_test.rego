package io.konveyor.forklift.vmware

import rego.v1

test_valid_tag_name_with_prefix if {
    mock_vm := { "tags": [{"name": "valid-prefix/tagname", "description": "valid"}] }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test for invalid tag names with prefix (should fail)
test_invalid_tag_name_with_prefix if {
    mock_vm := { "tags": [{"name": "invalid prefix/tagname", "description": "valid"}] }
    results := concerns with input as mock_vm
    count(results) == 1
}

# Test for valid tag names without prefix
test_valid_tag_name_no_prefix if {
    mock_vm := { "tags": [{"name": "validtagname", "description": "valid"}] }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test for invalid tag names without prefix (should fail)
test_invalid_tag_name_no_prefix if {
    mock_vm := { "tags": [{"name": "invalid tagname", "description": "valid"}] }
    results := concerns with input as mock_vm
    count(results) == 1
}

# Test for valid tag descriptions
test_valid_tag_description if {
    mock_vm := { "tags": [{"name": "validname", "description": "validdescription"}] }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test for invalid tag descriptions (should fail)
test_invalid_tag_description if {
    mock_vm := { "tags": [{"name": "validname", "description": "invalid description!"}] }
    results := concerns with input as mock_vm
    count(results) == 1
}

# Test for empty tags array
test_empty_tags if {
    mock_vm := { "tags": [] }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test for empty description (should be valid)
test_empty_description if {
    mock_vm := { "tags": [{"name": "validname", "description": ""}] }
    results := concerns with input as mock_vm
    count(results) == 0
}
