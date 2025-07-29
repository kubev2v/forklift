package io.konveyor.forklift.openstack
import future.keywords.in

# Match any volume with zero or negative size
invalid_volumes[idx] {
    some idx
    input.volumes[idx].size <= 0
}

# Raise a concern for each invalid volume
concerns[flag] {
    invalid_volumes[idx]
    volume := input.volumes[idx]
    flag := {
        "id": "openstack.disk.capacity.invalid", 
        "category": "Critical",
        "label": sprintf("Volume '%v' has an invalid size of %v GB", [volume.name, volume.size]),
        "assessment": sprintf("Volume '%v' has a size of %v GB, which is not allowed. Size must be greater than zero.", [volume.name, volume.size])
    }
} 