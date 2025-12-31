package io.konveyor.forklift.vmware

import rego.v1

# Test RHEL 6 upgrade recommendation
test_rhel6_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "Red Hat Enterprise Linux 6 (64-bit)",
    }
    results = concerns with input as mock_vm
    # Should have 2 concerns: unsupported OS + upgrade recommendation
    count(results) == 2
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 7")
    contains(concern.assessment, "Red Hat Enterprise Linux 6")
}

# Test RHEL 6 upgrade recommendation using guestName
test_rhel6_upgrade_recommendation_by_guestName if {
    mock_vm := {
        "name": "test",
        "guestName": "Red Hat Enterprise Linux 6 (32-bit)",
    }
    results = concerns with input as mock_vm
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 7")
    contains(concern.assessment, "Red Hat Enterprise Linux 6")
}

# Test CentOS 7 upgrade recommendation
test_centos7_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "CentOS 7 (64-bit)",
    }
    results = concerns with input as mock_vm
    count(results) == 2
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 7")
}

# Test CentOS 8 upgrade recommendation
test_centos8_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "CentOS 8 (64-bit)",
    }
    results = concerns with input as mock_vm
    count(results) == 2
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 8")
}

# Test CentOS 9 upgrade recommendation
test_centos9_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "CentOS 9 Stream",
    }
    results = concerns with input as mock_vm
    count(results) == 2
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 9")
}

# Test Amazon Linux 2 upgrade recommendation
test_amazon_linux2_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "Amazon Linux 2",
    }
    results = concerns with input as mock_vm
    count(results) == 2
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 8")
}

# Test supported OS has no upgrade recommendation
test_supported_rhel9_no_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "Red Hat Enterprise Linux 9 (64-bit)",
    }
    results = concerns with input as mock_vm
    # No concerns for supported OS
    count(results) == 0
}

# Test that guestNameFromVmwareTools takes precedence
test_upgrade_recommendation_guestNameFromVmwareTools_precedence if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "CentOS 7 (64-bit)",
        "guestName": "Red Hat Enterprise Linux 9 (64-bit)",
    }
    results = concerns with input as mock_vm
    # Should use guestNameFromVmwareTools which is CentOS 7 (unsupported with upgrade path)
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "CentOS 7")
    contains(concern.assessment, "Red Hat Enterprise Linux 7")
}

# Test case insensitivity
test_centos_lowercase_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestNameFromVmwareTools": "centos 8",
    }
    results = concerns with input as mock_vm
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 8")
}

# Test RHEL 6 upgrade recommendation using guestId only
test_rhel6_upgrade_recommendation_by_guestId if {
    mock_vm := {
        "name": "test",
        "guestId": "rhel6_64Guest",
    }
    results = concerns with input as mock_vm
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 6")
    contains(concern.assessment, "Red Hat Enterprise Linux 7")
}

# Test guestId fallback when guestNameFromVmwareTools is empty
test_rhel6_upgrade_recommendation_by_guestId_empty_tools if {
    mock_vm := {
        "name": "test",
        "guestId": "rhel6Guest",
        "guestNameFromVmwareTools": "",
    }
    results = concerns with input as mock_vm
    some concern in results
    concern.id == "vmware.os.upgrade.recommendation"
    contains(concern.assessment, "Red Hat Enterprise Linux 7")
}

# Test unsupported guestId without upgrade path (no upgrade recommendation)
test_photon_guestId_no_upgrade_recommendation if {
    mock_vm := {
        "name": "test",
        "guestId": "vmwarePhoton64Guest",
    }
    results = concerns with input as mock_vm
    # Should have 1 concern: unsupported OS only (no upgrade recommendation for PhotonOS)
    count(results) == 1
    some concern in results
    concern.id == "vmware.os.unsupported"
}

