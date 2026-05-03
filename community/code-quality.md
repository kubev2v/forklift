# Code Quality

How to format, lint, test, and validate your changes locally before pushing.

## Quick Start

Run the full CI pipeline locally to catch issues before pushing:

```bash
make ci
```

This runs all of the steps below in order: unit tests, binary build, module tidying, vendoring, generated code verification, linting, and CRD schema validation.

## Formatting

Format Go source files according to standard Go style:

```bash
make fmt
```

## Static Analysis

Run `go vet` to catch common mistakes (unused variables, incorrect format strings, unreachable code):

```bash
make vet
```

## Linting

Run `golangci-lint` with the project's configuration. The correct version is pinned in the Makefile and auto-installed on first run -- no manual setup needed:

```bash
make lint
```

## Unit Tests

Run Go unit tests with coverage for the `pkg/` and `cmd/` packages:

```bash
make test
```

This also triggers `fmt`, `vet`, `generate`, `manifests`, and `validation-test` as prerequisites.

## Validation Policy Tests

Run OPA (Open Policy Agent) tests against the Rego migration validation policies in `validation/policies/`. These cover VM eligibility rules for VMware, oVirt, OpenStack, OVA, and Hyper-V:

```bash
make validation-test
```

## Code Generation

Generate deepcopy functions and CRD code, then verify that generated files are up to date:

```bash
make generate
make generate-verify
```

## Manifest Generation

Regenerate all code and upstream/downstream operator manifests after changing CRD APIs or operator configuration:

```bash
make update-manifests
```

## Module Management

Keep `go.mod` and the vendor directory clean:

```bash
make tidy
make vendor
```

## CRD Schema Validation

Validate the ForkliftController CRD schema (requires Python 3):

```bash
make validate-forklift-controller-crd
```

## Commit Message Validation

Validate that your commit messages match the required format:

```bash
make validate-commits
```

## Building and Pushing Images

Build and push with custom registry settings:

```bash
make REGISTRY=quay.io REGISTRY_ORG=myuser REGISTRY_TAG=test1 push-controller-image
```

See the [README](../README.md) for the full set of image targets and environment variables.

## References

- [Kubernetes Coding Conventions](https://github.com/kubernetes/community/blob/master/contributors/guide/coding-conventions.md)
