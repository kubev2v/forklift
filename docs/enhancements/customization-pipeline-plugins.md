---
title: customization-pipeline-plugins
authors:
  - "@yaacov"
reviewers:
  - TBD
approvers:
  - TBD
creation-date: 2026-05-19
last-updated: 2026-05-21
status: implemented
---

# Self-Contained GuestFS Customize with Minimal virt-customize

## Release Signoff Checklist

- [x] Enhancement is `implementable`
- [x] Design details are appropriately documented from clear requirements
- [ ] Test plan is defined
- [ ] User-facing documentation is created

## Summary

The `pkg/virt-v2v/customize` package uses a self-contained, three-phase
approach for post-conversion guest customization:

1. **Probe** (`guestfish --ro`): Detects OS family, network configurations,
   and extracts interface data directly from the guest disk -- no dependency
   on `virt-v2v-inspector`.
2. **Apply files** (`guestfish --rw`): Performs all file operations (uploads,
   generated udev rules) in a single guestfish session.
3. **Execute scripts** (`virt-customize`, only if needed): Runs only
   `--firstboot` and `--run` scripts that require in-guest execution.

Each customization concern is handled by a focused plugin that returns
declarative `Actions` (file operations + exec operations). The orchestrator
collects all actions and dispatches them to the appropriate tool.

Two independent skip flags control which phases run:

- `V2V_skipConversion` / `--skip-conversion` — skip virt-v2v and the
  inspector, run only customization on pre-existing disks.
- `V2V_skipCustomize` / `--skip-customize` — skip guest customization
  after conversion.

The flags combine freely: both false (default) runs the full pipeline, both
true is a valid no-op.

## Motivation

The previous implementation had several problems:

1. **Dependency on virt-v2v-inspector**: OS detection relied on
   `InspectionOS` data from the inspector pipeline, coupling the customize
   package to an external component.

2. **Runtime-only detection via in-guest shell script**: A 560-line shell
   script (`network_config_util.sh`) ran inside the guest via
   `virt-customize --run` to discover network data (IP addresses, interface
   names, MAC addresses) and generate udev rules. It tried all 7 methods
   sequentially (ifcfg, NM, NM-lease, dhclient, netplan, ifquery, wicked).
   This was opaque, hard to debug, and impossible to validate before
   execution -- yet the data could be extracted upfront from the disk.

3. **Everything via virt-customize**: All operations (file uploads, generated
   content, LUKS keys, script execution) went through `virt-customize` even
   when most are simple file writes that `guestfish` handles natively.

### Goals

- Full independence from `virt-v2v-inspector` for guest detection (probe
  directly via `guestfish --ro`).
- Pre-extract all guest configuration data (OS, network configs, IPs, MACs)
  before plugins run, eliminating in-guest shell scripts for data discovery.
- Minimize `virt-customize` usage to only operations that genuinely require
  it (`--firstboot` for systemd service creation, `--run` for chroot exec).
- Generate udev rules directly in Go from probe data, replacing the
  runtime shell script that tried 7 network methods sequentially.
- Plugins return declarative `Actions` (data), not side effects (command
  builder calls), improving testability.
- Single `guestfish --rw` session for all file operations (one appliance
  boot instead of per-operation).
- Fail fast on probe failure (subsequent phases also require disk access).

### Non-Goals

- Replacing `virt-customize` for `--firstboot` (creating systemd/sysvinit
  service units is complex and better left to the existing tool).
- Supporting runtime plugin loading (all plugins are compiled in).
- Changing how `AppConfig` is populated from environment variables.
- Full virt-v2v conversion replacement (`--skip-conversion` re-uses
  `customize.Run` but the conversion pipeline itself is unchanged).

## Proposal

### User Stories

#### Story 1

As a Forklift developer, I need to add support for a new Linux distribution's
network configuration. I only need to add a parser in the `probe` sub-package
for the new config format -- the unified `network/udev` plugin generates the
same udev rules output regardless of source format.

#### Story 2

As a Forklift developer, I need to debug why static IP configuration fails on
a specific RHEL migration. I can inspect the `GuestInfo` probe output to see
exactly what interfaces, IPs, and MACs were detected, then check whether the
generated udev rules match expectations -- all without running anything inside
the guest.

#### Story 3

As a Forklift developer, I need to add a Windows-only customization that
uploads a configuration file. I create a plugin returning `FileAction`s and
know that `virt-customize` will be skipped entirely for this migration --
only `guestfish --rw` runs.

### Implementation Details

#### Architecture

```
customize.Run(opts)
    │
    ├── Resolve LUKS keys from config (infrastructure, not a plugin)
    │
    ├── Phase 1: guestfish --ro --key --root (Probe)
    │   ├── Detect OS (Windows/Linux/Unknown)
    │   ├── Detect network configurations (ifcfg, NM, netplan, etc.)
    │   └── Extract interface configs (IPs, MACs, names)
    │        → GuestInfo struct
    │
    ├── Resolve applicable plugins (resolve.go)
    │   └── Each plugin's Applicable(ctx) uses ctx.Guest
    │
    ├── Collect Actions from all plugins
    │   └── Each plugin's Apply(ctx) returns *Actions
    │        → []FileAction + []ExecAction
    │
    ├── Phase 2: guestfish --rw --key --root (Apply Files)
    │   └── Single session: mkdir-p + upload + write + chmod
    │
    └── Phase 3: virt-customize --key (Exec, only if needed)
        └── --firstboot + --run
```

`--root` selects the OS root partition when multi-boot or multi-disk images are
present. The value comes from `opts.Config.RootDisk`; when empty it defaults to
`"first"` (guestfish auto-detection). `--key` arguments for LUKS-encrypted
volumes are resolved once at the start of `Run` and passed to every phase
(probe, apply, exec) so encrypted disks are accessible throughout.

#### Probe Sub-Package

The `probe` sub-package runs two guestfish passes -- detection (filesystem
markers) then extraction (config file parsing) -- and returns a `GuestInfo`
struct with OS family, detected network configurations, and pre-extracted interface
data (IPv4/IPv6 addresses, MACs, DHCP status).

Each network config format (ifcfg, NM keyfile, netplan, interfaces, wicked)
has a dedicated parser in `probe/parser/`, using syntax-aware libraries where
available (YAML for netplan, XML for wicked).

See [probe/README.md](../../pkg/virt-v2v/customization/probe/README.md) for
detection markers, extraction details, and parser specifics.

#### Plugins Sub-Package

The `plugins` sub-package contains 8 plugins evaluated in declaration order.
Each plugin implements `Applicable(ctx)` to decide whether it should run, and
`Apply(ctx)` to return declarative `Actions`.

Ordering matters: network config runs first so IPs are set before boot
scripts. The firstboot runner must come after all `.ps1` scripts are written.
`conversion-done` runs last so it signals only after everything completes.

See [plugins/README.md](../../pkg/virt-v2v/customization/plugins/README.md)
for the full plugin list, applicability rules, and implementation details.

#### What Goes Where

| Operation | Tool | Why |
|-----------|------|-----|
| OS + network detection | guestfish --ro --key --root | Read-only probing |
| Create parent directories | guestfish --rw --key --root | `mkdir-p` before writes |
| Upload files to guest | guestfish --rw --key --root | Native `upload` command |
| Write generated udev rules | guestfish --rw --key --root | Native `write` command |
| Windows firstboot scripts | guestfish --rw --key --root | Just file uploads |
| VMware driver removal scripts | guestfish --rw --key --root | Just file uploads |
| `--firstboot` (Linux) | virt-customize --key | Creates systemd/sysvinit service |
| `--run` (dynamic user scripts) | virt-customize --key | Executes inside guest chroot |

`--key` arguments are passed to every tool invocation when LUKS encryption
is configured, ensuring encrypted volumes are accessible in all phases.
`--root` selects the OS root partition (from `Config.RootDisk`, defaulting to
`"first"`) so multi-boot and multi-disk images are handled correctly.

#### Conversion Flags

Two independent boolean flags on `AppConfig` control which top-level phases
the entrypoint executes:

| Flag | Env var | CLI flag | Effect |
|------|---------|----------|--------|
| `SkipConversion` | `V2V_skipConversion` | `--skip-conversion` | Skip virt-v2v and inspector |
| `SkipCustomize` | `V2V_skipCustomize` | `--skip-customize` | Skip the three-phase guest customization |

The flags are independent and combine freely:

| SkipConversion | SkipCustomize | Result |
|:-:|:-:|--------|
| false | false | Full pipeline (default) |
| true | false | Customization only on pre-existing disks |
| false | true | Conversion + inspection, no customization |
| true | true | No-op (setup only) |

When `SkipConversion` is true, `AppConfig.validate()` skips source/provider
checks (no source provider is needed). Certificate linking always runs
because the controller mounts the secret volume unconditionally.

```go
// entrypoint.go
linkCertificates(env)

if !env.SkipConversion {
    // ... run virt-v2v, inspector ...
}

if !env.SkipCustomize {
    convert.RunCustomize()
}
```

#### Shared API

Core types are defined in the `api` sub-package:

- `GuestInfo` -- probe result with OS family, detected network configurations,
  and extracted interfaces.
- `InterfaceInfo` -- per-interface data with separate `IPv4`/`IPv6` address
  slices and a `DHCP` flag.
- `Plugin` -- interface that all plugins implement (`Name`, `Applicable`,
  `Apply`).
- `Actions` -- declarative return type from plugins containing `FileAction`s
  and `ExecAction`s.

The apply phase creates parent directories (`mkdir-p`) automatically before
writing files to the guest, so plugins don't need to worry about directory
existence.

### Security, Risks, and Mitigations

**Reduced attack surface**: `virt-customize` is now called only when
`--firstboot` or `--run` scripts are needed. For Windows-only migrations
(file uploads only), it is skipped entirely.

**Script injection**: The same protections apply -- scripts are embedded at
compile time and user-provided dynamic scripts follow naming convention
validation.

**Probe failure**: If `guestfish --ro` fails (e.g. unsupported filesystem),
the error propagates immediately. Subsequent phases also require disk
access, so a fallback would only delay the inevitable failure.

## Alternatives

1. **Keep everything in virt-customize**: The previous approach. Rejected
   because it required running complex shell scripts inside the guest for
   data re-discovery, provided no independence from the inspector, and
   booted `virt-customize` even for simple file uploads.

2. **Replace virt-customize entirely with guestfish**: Would require
   implementing systemd service creation and sysvinit/upstart fallback
   logic for `--firstboot`. Rejected as unnecessary complexity given that
   `virt-customize` handles this well.

3. **Distro-based plugins** (e.g. `linux/rhel`, `linux/ubuntu`): Groups
   concerns by distro rather than by subsystem. Rejected because the probe
   now detects actual filesystem markers, making distro-based grouping
   redundant.

4. **Separate plugin per network config format**: Each format-specific plugin
   generates its own udev rules independently. Rejected because the probe
   now provides all data needed for a single unified rule generator in Go.

## Extensibility

New post-conversion concerns slot in as new plugin categories without touching
existing code:

```
plugins/
  bootloader/       -- FUTURE: grub/ vs systemd-boot/
  selinux/          -- FUTURE: relabel handling
  cloud-init/       -- FUTURE: cloud-init cleanup
  agents/           -- FUTURE: guest agent installation
```

New network config formats require only adding a parser in `probe/parser/`
(one Go file per format) and updating the extraction script builder in
`probe/extract.go` -- the `network/udev` plugin handles all Linux network
configurations uniformly.
