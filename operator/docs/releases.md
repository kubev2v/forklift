# Konveyor Forklift Upstream Release Instructions

## Prerequisites

- Podman 1.6.4+
- [Operator SDK v1.3.0+](https://github.com/operator-framework/operator-sdk)
- [Opm](https://github.com/operator-framework/operator-registry) for index image manipulation
- [Quay.io](https://quay.io/organization/konveyor) access to Konveyor Forklift repos

## Overview
The Konveyor Forklift new release procedure consist of a few steps summarized below: 
- Create a new release branch on Konveyor Forklift Operator repo
- Create and submit PR preparing bundle manifests for the new release branch
- Once merged, bundle images for new release will be automatically built on Quay.io
- Build new index images and push new metadata to Quay.io

## Konveyor Forklift Stable
We use semantic versioning convention (semver) for stable releases, release branches should be in the form of release-v<semver>

1. Create a new release branch, for example `release-v2.3.0`
1. Create a PR for the new release branch
   1. Run `VERSION=2.3.0 make bundle`
   1. Bump the `forklift_operator_version` in `roles/forkliftcontroller/defaults/main.yml`
   1. Add relatedImages to the CSV with the SHA digests.
   1. Review changes, commmit, and submit the PR for review
1. Once the release PR is ready and merged, add it to the index image and push to quay.io
   1. `CATALOG_BASE_IMG=quay.io/konveyor/forklift-operator-index:release-v2.2.0 VERSION=2.3.0 make catalog-build bundle-push`
   1. `CATALOG_BASE_IMG=quay.io/konveyor/forklift-operator-index:release-v2.3.0 TAG=latest make catalog-build catalog-push`
   1. Create or refresh existing konveyor-forklift catalog source and validate `oc create -f forklift-operator-catalog.yaml`
