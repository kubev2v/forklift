package io.konveyor.forklift.vmware

import rego.v1

# Test valid custom attribute name
test_valid_custom_attr_name if {
    mock_vm := {
        "customDef": [{"key": 101, "name": "owner", "managedObjectType": "VirtualMachine"}],
        "customValues": [{"key": 101, "value": "admin@example.com"}]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test valid custom attribute with hyphen and underscore
test_valid_custom_attr_name_with_special_chars if {
    mock_vm := {
        "customDef": [{"key": 101, "name": "cost-center_2024", "managedObjectType": "VirtualMachine"}],
        "customValues": [{"key": 101, "value": "IT-123"}]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test invalid custom attribute name with spaces (will be sanitized - warning)
test_invalid_custom_attr_name_with_spaces if {
    mock_vm := {
        "customDef": [{"key": 101, "name": "Cost Center", "managedObjectType": "VirtualMachine"}],
        "customValues": [{"key": 101, "value": "IT-123"}]
    }
    results := concerns with input as mock_vm
    count(results) == 1
    results[_].category == "Warning"
}

# Test invalid custom attribute name starting with special char
test_invalid_custom_attr_name_starting_special if {
    mock_vm := {
        "customDef": [{"key": 101, "name": "-invalid-name", "managedObjectType": "VirtualMachine"}],
        "customValues": [{"key": 101, "value": "value"}]
    }
    results := concerns with input as mock_vm
    count(results) == 1
}

# Test multiple custom attributes - one valid, one invalid
test_mixed_custom_attrs if {
    mock_vm := {
        "customDef": [
            {"key": 101, "name": "valid-name", "managedObjectType": "VirtualMachine"},
            {"key": 102, "name": "Invalid Name!", "managedObjectType": "VirtualMachine"}
        ],
        "customValues": [
            {"key": 101, "value": "value1"},
            {"key": 102, "value": "value2"}
        ]
    }
    results := concerns with input as mock_vm
    count(results) == 1  # Only the invalid one should produce a warning
}

# Test empty custom values (no concerns)
test_empty_custom_values if {
    mock_vm := {
        "customDef": [{"key": 101, "name": "owner", "managedObjectType": "VirtualMachine"}],
        "customValues": []
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

# Test custom attribute with valid numeric name
test_valid_numeric_name if {
    mock_vm := {
        "customDef": [{"key": 101, "name": "attr123", "managedObjectType": "VirtualMachine"}],
        "customValues": [{"key": 101, "value": "test"}]
    }
    results := concerns with input as mock_vm
    count(results) == 0
}

