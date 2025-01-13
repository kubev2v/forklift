---
title: OVA Provider Storage Improvement
authors:
  - "@mansam"
reviewers:
  - "@mnecas"
approvers:
  - "@mnecas"
creation-date: 2024-12-20
last-updated: 2025-01-13
status: implementable
---

# OVA Provider Storage Improvement

## Release Signoff Checklist

- [ ] Enhancement is `implementable`
- [ ] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Allow specifying any PVC as the appliance catalogue backing for an OVA provider (rather than requiring 
the coordinates of an NFS share) and provide a way for the end user to upload appliances to
the catalogue.

## Motivation

The current implementation of the OVA provider in Forklift is difficult to use. Users
must specify the coordinates to an NFS share that contains appliances, and adding appliances
requires the user to add them to the NFS share outside the Forklift user experience.
Kubernetes/OpenShift users generally expect to think of storage in terms of PVs/PVCs, so 
having to provide an NFS share rather than a PVC (which could be backed by NFS) is
not aligned with user expectations. Allowing the user to specify a PVC to back the OVA
provider would be much more flexible and better match user expectations, and providing
UI to allow appliance upload would remove friction.

### Goals

1. Allow specifying any PVC as the storage backing for an OVA provider, rather than requiring
   the coordinates of an NFS share.
2. Expose an endpoint on the OVA provider server that permits uploading appliances to the catalogue.
3. Expose an endpoint on the OVA provider server that permits retrieving an appliance from a URL.
4. Expose the upload functionality in the UI.

## Proposal

### User Stories

#### Story 1

As a Forklift administrator, I want to configure an OVA provider to use an arbitrary volume as
the backing for the appliance catalogue.

#### Story 2

As a Forklift user, I want to upload an appliance to the appliance catalogue.

#### Story 3

As a Forklift user, I want to see which appliances have been uploaded to the appliance catalogue.

#### Story 4

As a Forklift user, I want to remove an appliance from the appliance catalogue.

#### Story 5

As a Forklift administrator, I want to disable the appliance catalogue management endpoints.

### Implementation Details/Notes/Constraints [optional]

Implementation can be approached in two stages.

* 1: Supporting arbitrary PVCs as storage backing for OVA providers and corresponding UX change.
* 2: Supporting upload and delete of appliances in the provider server and corresponding UX change.

#### Supporting Arbitrary PVCs

The provider controller currently expects an OVA provider resource to contain
the path to an NFS share in its `url` field. This URL is used to construct an NFS PV
which is mounted by an OVA provider server that is created when the OVA provider resource is reconciled.
This field will be made optional for OVA providers, and the provider controller will be updated to look for a
`pvc` key in the `settings` field which may contain the namespaced name of a PVC that will be mounted
by the provider server.

The current provider controller implementation does not update the deployment for the OVA provider server
when an OVA provider resource is reconciled; the deployment is only created if it does not exist and deleted
when an OVA provider resource is removed. The provider controller needs to be adjusted to keep OVA provider server
deployments in sync with the storage configuration of the provider resource.

This will require corresponding UX changes to relax the requirement on the URL field and expose an interface for
setting the desired PVC.

#### Uploading OVAs

The OVA provider server is currently very simple, consisting of a single file using the built-in HTTP server to expose
endpoints for consumption by the Forklift inventory. The OVA provider server needs to be extended with endpoints for
uploading, listing, and removing appliances. 

* `POST /appliances` Accepts a single OVA file to be stored in the appliance catalogue, returning a 409 if an appliance with that filename already exists.
* `GET /appliances` Return JSON array containing metadata about the appliances in the catalog (size, upload date, etc)
* `DELETE /appliances/:appliance` Remove an appliance from the catalogue, using OVA filename as the parameter.

Due to this increase in complexity, the provider server should be refactored to use a more feature rich server
(preferably Gin which is used in Forklift for the inventory) and reorganized into multiple files. Special consideration
needs to be given to the upload endpoint to ensure that uploaded appliances are streamed to disk efficiently. Configuration
options should be provided to set the maximum accepted file size.

### Security, Risks, and Mitigations

The upload component of this enhancement permits uploading and deleting arbitrary files from a volume in the cluster,
and those arbitrary uploads will be used in migration to construct VMs that will run in the cluster. This is an inherently
risky operation that is acceptable given the risk already inherent in migrating VMs into the cluster.

The provider server endpoints should be protected by auth, as is done for the inventory endpoints, and a configuration 
option should be provided so that the administrator can disable the OVA upload/delete endpoints.

## Design Details

### Test Plan

Integration tests should verify that the OVA provider server deployment is reconfigured when the OVA provider's
storage configuration is changed, that providers configured with NFS shares continue to function, that
the catalogue management endpoints work, and that uploaded OVAs can be imported.

### Upgrade / Downgrade Strategy

OVA providers that rely on NFS shares can continue to be supported until it becomes reasonable
to deprecate and remove direct consumption of NFS shares. The provider controller shall update
any existing OVA provider server deployments to ensure they refer to the correct version of
the image.

An OVA Provider resource that has been created with a reference to a PVC instead of an NFS share
will not be able to be inventoried or used in a plan if Forklift is downgraded to a prior version. The existing
version of Forklift does not update the OVA provider server deployment if the provider is changed, so
downgrading will require any OVA provider server deployments to be removed so that the provider controller
can recreate them with the correct version of the image.

## Implementation History

* 1/13/2025: Enhancement pull request opened

## Drawbacks

Allowing the end user to provide a PVC of their choice to use for the appliance catalogue rather than requiring an
NFS share is a purely beneficial improvement in usability and aligns better with Kube/OpenShift patterns.
Likewise, allowing upload of images into the store is a significant improvement in usability that comes at the minor
cost of a slightly more complex provider server to handle the file upload. There are no meaningful drawbacks to these
usability improvements.

## Alternatives

### Inventorying remote appliances

An alternative to storing appliances locally is to store the URLs of appliances, and then use nbdkit to remotely examine
the appliance so that it can be added to the inventory. Then the remote appliance disks could be imported directly to 
VM PVCs without the intermediate step of storing the appliance in the cluster. This avoids the long term storage
overhead of having the full appliance in the provider catalogue, but it has several drawbacks that make it unsuitable.

* The overhead of transferring the appliance image across the network into the cluster is incurred
  each time a VM is created from the appliance instead of once when it is added to the catalogue.
* Creating a VM from the remote image requires uninterrupted connectivity to the remote host and is unsuitable for disconnected clusters.
* Remote images could move or go missing, requiring manual intervention to update the inventory.

For these reasons it is more practical to upload the appliance into the provider catalogue once and store it for future
use.