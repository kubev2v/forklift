package io.konveyor.forklift.ovirt
 
test_with_first_legal_placement_policy_affinity {
    mock_vm := { "name": "test",
                 "placementPolicyAffinity": "user_migratable"
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_second_legal_placement_policy_affinity {
    mock_vm := { "name": "test",
                 "placementPolicyAffinity": "pinned"
                }
    results = concerns with input as mock_vm
    count(results) == 0
}

test_with_illegal_placement_policy_affinity {
    mock_vm := { "name": "test",
                 "placementPolicyAffinity": "migratable"
                }
    results = concerns with input as mock_vm
    count(results) == 1
}