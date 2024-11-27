---
title: ova-ovfenv
authors:
  - "@arturshadnik"
reviewers:
  - "@mnecas"
approvers:
  - "@mnecas"
creation-date: 2024-11-27
last-updated: 2024-11-27
status: implementable
---

# OVA/OVF environment

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Open Questions

1. How should `vmtoolsd-shim` be made available to the guest? Create the binary as a release artifact and allow users to download it as needed? Bundle it with the forklift-virt-v2v image and copy it during guest conversion?

## Summary

In the current implementation, Forklift supports importing OVAs exported from vSphere, to run on KubeVirt. Most OVAs expect to be able to obtain runtime configuration options - also known as OVF environment XML - on boot via `vmtoolsd --cmd 'info-get guestinfo.ovfEnv'`. Right now, Forklift does not provide a clear way to configure these OVF environment XML during the migration. This is a shortcoming which may prevent users from being able to run VMs imported from OVAs. This proposal is to add several features to the migration process from OVA providers to address this issue. By making the necessary OVF environment XML queriably via REST API prior to migration, and providing tools to inject them into the guest, Forklift will be able to support a much wider variety of OVAs. The newly added functionality will be optional and backwards compatible with existing Plans, allowing users to continue using the existing functionality without any changes.

## Motivation

The motivation for this proposal is to allow Forklift to import a wider variety of OVAs, expanding the utility of the project.

### Goals

* Forklift will allow users to specify OVF OVF environment XML to be set in the guest during migration from OVA providers.
* Forklift will provide all the tools necessary to inject the OVF environment XML into the guest.

### Non-Goals

* Forklift will not enforce that an ovfenv is set.
* Forklift will not perform any validation of the user-defined ovfenv.
* Forklift will not automatically populate the ovfenv based on the OVA metadata. It will only give users the tools and information needed to set the OVF environment XML manually. (stretch goal - the UI plugin could be extended to provide an enhanced UX for creating the ovfenv)

## Proposal

The proposed change consists of 3 distinct parts:

1. Expanding the inventory REST API to support querying the required OVF environment XML of an OVA. This change will also include expanding the `ova-provider-server` to support providing the required data to the inventory service. No new endpoints will be added, but the existing GET endpoints will be expanded to return the new data. This data is the [Product Section](https://pkg.go.dev/github.com/vmware/govmomi@v0.43.0/ovf#ProductSection) of the OVF envelope. By adding this data to the inventory, requests to GET `/forklift-inventory/providers/ova/<provider-id>/vms` and `/forklift-inventory/providers/ova/<provider-id>/vms/<vm-id>` will return the ovfenv data for all or the specified VM, respectively. The data can then be queried by the user to understand what OVF environment XML are needed for the OVA. UI changes to accommodate this are outside the scope of this proposal.

2. Expanding the Plan CRD to support specifying a ConfigMap containing the OVF environment XML to be set in the guest, for each VM in the Plan. This ConfigMap must be created by the user in the target namespace prior to starting the migration. When the OVA builder constructs the KubeVirt VM template, it will check for a ConfigMap reference in the VM entry in the plan, then check for the existence of the ConfigMap in the target namespace, and, if found, attach the ConfigMap as a disk to the VM template, as outlined in [this KubeVirt doc](https://kubevirt.io/user-guide/storage/disks_and_volumes/#configmap). 

    Sample of a ConfigMap with an ovfenv specified:

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: abc123-ovfenv
    data:
      ovfenv.xml: |
        <?xml version="1.0" encoding="UTF-8"?>
        <Environment
            xmlns="http://schemas.dmtf.org/ovf/environment/1"
            xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
            xmlns:ovf="http://schemas.dmtf.org/ovf/environment/1"
            ovf:version="1.0"
            ovf:id="MyVirtualMachine">
            <PropertySection>
                <Property ovf:key="hostname" ovf:value="my-vm"/>
                <Property ovf:key="ip_address" ovf:value="192.168.1.100"/>
                <Property ovf:key="admin_password" ovf:value="securepassword"/>
            </PropertySection>
            <PlatformSection>
            </PlatformSection>
        </Environment>
    ```

    Sample of a Plan CRD with 2 VMs, one with a reference to a ConfigMap containing an ovfenv, one without:

    ```yaml
    apiVersion: forklift.konveyor.io/v1beta1
    kind: Plan
    metadata:
      name: example-plan
      namespace: ${NAMESPACE}
    spec:
      provider:
        source:
        namespace: ''
        name: ''
      destination:
        namespace: ''
        name: ''
      vms:
        - id: abc123
          name: my-vm
          ovaEnvConfigMap:
            name: abc123-ovfenv
        - id: def456
          name: my-other-vm
    ``` 

3. Provide a `vmtoolsd-shim` binary that can be used to inject the OVF environment XML into the guest. The binary will emulate the functionality of the `vmtoolsd` daemon related to the ovfenv, and will also be able to mount the ovfenv from the attached ConfigMap disk. When running `vmtoolsd --cmd 'info-get guestinfo.ovfEnv'`, the tool will attempt to mount the ovfenv from the ConfigMap disk, and if successful, will return the OVF environment XML to the caller.

### User Stories

#### Story 1

- Given that I have an OVA with specific configuration requirements,
- When I want to initiate a migration of an OVA appliance using the Forklift tool,
- Then I should be able to query the necessary OVF environment XML using the forklift-inventory REST API,
- And create a ConfigMap based on this data in the target namespace,
- And include a reference to this ConfigMap in the Plan CRD for the migration,
- And have a way to inject the `vmtoolsd-shim` binary into the guest,
- So that the migration process makes the OVF environment XML available as a disk to be mounted in the guest,
- Ensuring a smooth and consistent deployment on KubeVirt.

### Security, Risks, and Mitigations

* The Plan CRD will need to be updated to allow the `ovaEnvConfigMap` field to be set. This field will be optional, and will not be set by default, so existing Plans will not be affected.

* This enhancement will rely on the end user to provide a valid ovfenv in the ConfigMap. Since Forklift will not be validating the ovfenv, it is possible that the ovfenv will be invalid, and the appliance will not function as expected. 

* Depending on how `vmtoolsd-shim` is injected into the guest, it may be necessary for the user to create a script to download the binary (passed to virt-customize via ConfigMap) during VM first boot. Users should be directed to download the binary from forklift release artifacts to avoid potential security issues.

## Design Details

### Test Plan

Expand existing ova e2e tests to create Plans with and without the `ovaEnvConfigMap` field.

An open question is if/how to automatically validate that `vmtoolsd-shim` is working as expected. This would require accessing the guest of a migrated VM, and executing a command to check that the ovfenv was properly set. Since the shim is not exepcted to change, it may be sufficient to manually test it once only. 

### Upgrade / Downgrade Strategy

To make use of the functionality, an updated Plan CRD will be required. Previous Plans will be forward compatible, and will not require any changes. New Plans may or may not be backward compatible, depending on whether or not the `ovaEnvConfigMap` field is set.

## Implementation History

* 11/27/2024 - Enhancement submitted.

## Drawbacks

Several steps rely on user action without clear guidance. This approach can be error prone and lead to a poor UX.
