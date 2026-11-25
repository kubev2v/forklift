# Forklift virtualization validation runner

This image is owned and released by Forklift. It executes only the validation
profiles supported by the Forklift `VirtualizationValidation` API:

- `basic-v1`: API authentication, OpenShift Virtualization installation,
  validation-namespace quota, and a canary VM workflow.
- `basic-all-nodes-v1`: the same cluster checks and the canary workflow once
  for each eligible target node.

The controller supplies target credentials and invokes the runner through its
`run-validation` command. Results are published as CTRF data for the Forklift
validation controller to project into the CR status.

This directory intentionally contains only the checks referenced by the two
supported profiles. Adding a profile requires adding its selected checks here
and updating the controller's profile contract.
