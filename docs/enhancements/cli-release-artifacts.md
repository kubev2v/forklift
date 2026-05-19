---
title: cli-release-artifacts
authors:
  - "@yzamir"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-05-19
last-updated: 2026-05-19
status: implementable
see-also:
  - "/enhancements/mcp-server-integration.md"
---

# CLI Release Artifacts

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [x] Test plan is defined
- [ ] User-facing documentation is created

## Summary

Automate `kubectl-mtv` CLI binary distribution as GitHub Release artifacts,
complementing the existing container-based distribution via the
`forklift-cli-download` image. When a GitHub Release is created on the
`kubev2v/forklift` repository, a CI workflow cross-compiles versioned archives
for all supported platforms (Linux amd64/arm64, macOS amd64/arm64, Windows
amd64), generates SHA-256 checksums, and attaches them to the release.

## Motivation

Users who want `kubectl-mtv` today must either install from the upstream
`yaacov/kubectl-mtv` GitHub releases or extract binaries from the
`forklift-cli-download` container image. Neither option maps directly to a
specific Forklift release version. Forklift upstream releases (GitHub tags)
should carry the same CLI archives so users can download a CLI binary that
matches the exact version of Forklift they have deployed.

### Goals

- Attach versioned CLI archives to every Forklift GitHub Release automatically.
- Provide SHA-256 checksum files for every archive.
- Use the same naming convention as upstream (`kubectl-mtv-{VERSION}-{OS}-{ARCH}.{ext}`) for consistency.
- Provide a `make dist-cli` target to produce release-ready archives locally without CI.
- Make the workflow idempotent so re-running a release event does not overwrite existing assets.

### Non-Goals

- Krew index updates (handled by the upstream `yaacov/kubectl-mtv` repo).
- Downstream packaging (Brew, RPM, etc.).
- Changing the existing container-based distribution via the `forklift-cli-download` image.

## Proposal

### User Stories

#### Story 1

As a cluster administrator, I want to download `kubectl-mtv` directly from the
Forklift GitHub release page so that the CLI version matches my deployed
Forklift version, without needing to cross-reference the upstream kubectl-mtv
project.

#### Story 2

As a Forklift developer, I want a single `make dist-cli` command to produce
release-ready archives and checksums locally, so I can verify the packaging
before a release is cut.

### Implementation Details

Three deliverables make up this enhancement:

1. **`dist-*` Makefile targets** (`cmd/kubectl-mtv/Makefile`) -- per-platform
   targets (`dist-linux-amd64`, `dist-linux-arm64`, `dist-darwin-amd64`,
   `dist-darwin-arm64`, `dist-windows-amd64`) that package binaries into
   versioned archives with the LICENSE file. A `dist-all` target orchestrates
   all platforms, checksums, and a listing. A `dist-cli` target in the root
   Makefile delegates to `make -C cmd/kubectl-mtv dist-all`.

2. **`.github/workflows/release-cli.yml`** -- GitHub Actions workflow triggered
   on `release` events that builds, packages, and uploads CLI archives to the
   release.

3. **`cmd/kubectl-mtv/.gitignore`** -- ignores `bin/` and `dist/` build outputs.

### Artifact Matrix

| Platform       | Binary name                     | Archive format | Archive name                                       |
|----------------|---------------------------------|----------------|----------------------------------------------------|
| Linux amd64    | `kubectl-mtv-linux-amd64`       | tar.gz         | `kubectl-mtv-{VERSION}-linux-amd64.tar.gz`         |
| Linux arm64    | `kubectl-mtv-linux-arm64`       | tar.gz         | `kubectl-mtv-{VERSION}-linux-arm64.tar.gz`         |
| macOS amd64    | `kubectl-mtv-darwin-amd64`      | tar.gz         | `kubectl-mtv-{VERSION}-darwin-amd64.tar.gz`        |
| macOS arm64    | `kubectl-mtv-darwin-arm64`      | tar.gz         | `kubectl-mtv-{VERSION}-darwin-arm64.tar.gz`        |
| Windows amd64  | `kubectl-mtv-windows-amd64.exe` | zip            | `kubectl-mtv-{VERSION}-windows-amd64.zip`          |

Each archive includes the binary and the repository LICENSE file. A
corresponding `.sha256sum` file is generated for each archive.

### Security, Risks, and Mitigations

- **Integrity verification**: SHA-256 checksum files are generated alongside
  every archive, allowing users to verify downloads.
- **Scoped permissions**: The release workflow requests only
  `permissions: contents: write`, limited to attaching assets to an existing
  release. It cannot create releases, push code, or modify other repository
  settings.
- **Idempotency**: The workflow checks whether assets already exist on the
  release before building. If assets are present, the entire build is skipped.
  This prevents accidental overwrites on workflow re-runs.
- **Vendored builds**: Builds use `GOFLAGS=-mod=vendor` to ensure
  reproducibility without network access during compilation.

## Design Details

### VERSION Resolution

| Context    | Source                                         |
|------------|------------------------------------------------|
| CI         | `github.event.release.tag_name`                |
| Local      | `MTV_VERSION` from `build/release.conf` + `v` postfix |
| Override   | `MTV_VERSION=vX.Y.Z make dist-cli`             |

## Alternatives

1. **GoReleaser**: Use GoReleaser to automate cross-compilation and archive
   creation. Rejected because it adds a new tool dependency and configuration
   file for what is a straightforward `go build` + `tar`/`zip` operation.

2. **Repackage upstream releases**: Download pre-built binaries from the
   upstream `yaacov/kubectl-mtv` GitHub releases and re-attach them to the
   Forklift release. Rejected because the forklift repo vendors a specific
   version of the CLI source, and building from vendored code ensures the binary
   matches exactly what the `forklift-cli-download` container image ships.

3. **Container-only distribution (status quo)**: Continue distributing CLI
   binaries only via the `forklift-cli-download` container image. Rejected
   because extracting a binary from a running container or pulling an image just
   to get a CLI tool is not user-friendly, especially for users who do not have
   a container runtime installed locally.
