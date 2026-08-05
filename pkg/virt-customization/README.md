# virt-customization

Post-conversion guest disk customization for Forklift/MTV.

This package modifies VM disk images after virt-v2v conversion using a
four-phase pipeline: **probe, resolve, commit files, commit CLI**. Plugins
declare what changes they need; the commit layer writes them to the guest.

## Quick start

```go
customization.Run(customization.Options{
    Config:     appConfig,
    Disks:      []string{"/tmp/disk.img"},
    OpenHandle: guesthandle.DefaultOpenHandle(),
})
```

## Pipeline

```
customize.Run(opts)
    |
    +-- Resolve LUKS keys
    +-- OpenHandle -> GuestHandle (libguestfs Go session)
    |
    |   Phase 1 - Probe (GuestHandle API)
    |     detect.go:  IsDir/IsFile -> OS family, network stacks, cloud-init, SSH, console
    |     extract.go: Cat/GlobExpand/Ls -> InterfaceInfo, CloudInitInfo, SSHInfo, ConsoleInfo
    |     -> GuestInfo
    |
    |   Resolve applicable plugins (Applicable filter)
    |   Collect declarative Actions (Apply)
    |     -> []FileAction + []RegAction + []ExecAction
    |
    |   Phase 2 - Files (GuestHandle API)
    |     commit/files.go: MkdirP + Write/Upload + Chmod
    |
    +-- Shutdown + Close (release disk locks)
    |
    +-- Phase 3 - Registry (CLI)
    |   commit/registry.go: virt-win-reg --merge
    |
    +-- Phase 4 - Scripts (CLI)
        commit/scripts.go: virt-customize --firstboot/--run
```

Phases 1+2 share a single libguestfs session. Phases 3+4 are CLI-based
and each boot their own appliance only when actions exist.

## Package layout

```
virt-customization/
  api/              Core types: GuestInfo, GuestHandle, Plugin, Actions
  commit/           Materialize actions onto guest disk (files, registry, scripts)
  guesthandle/      HandleFactory, DefaultOpenHandle, production GuestHandle (libguestfs CGO)
  plugins/          Modular customization steps
    example/        Reference plugin (see "Adding a plugin" below)
  probe/            Read-only guest inspection
    netcfg/         Network config parsers (ifcfg, NM, NM DHCP leases, dhclient, netplan, interfaces, wicked)
    cloudinit/      cloud.cfg YAML parser (datasource list, network config disabled)
    ssh/            sshd_config parser (PermitRootLogin, PasswordAuthentication, Port)
    console/        GRUB defaults parser (serial console params), serial-getty units
```

## Design principles

- **Probe-first**: All guest state is extracted read-only before any plugin
  runs. No in-guest shell scripts for discovery.
- **Declarative plugins**: Plugins return `Actions` (data), not side effects.
  This makes them unit-testable with mocked contexts.
- **Go API for data operations**: Probe and file operations use the
  GuestHandle interface directly. CLI tools (virt-customize, virt-win-reg)
  are used only for script execution and registry merges.
- **Compile-time registry**: Plugins are registered in `resolve.go` in fixed
  order. No dynamic loading.
- **Stack detection over distro detection**: Filesystem markers
  (`/etc/sysconfig/network-scripts`, `/etc/netplan`, etc.) determine which
  parsers run, not the distro name.

## Plugin interface

```go
type Plugin interface {
    Name() string
    Applicable(ctx *Context) bool
    Apply(ctx *Context) (*Actions, error)
}
```

`Applicable` gates execution based on `ctx.Guest` (OS, network stacks) and
`ctx.Config` (feature flags). `Apply` returns declarative actions:

| Action type | Written by | Phase |
|-------------|------------|-------|
| `FileAction` | `commit/files.go` via GuestHandle | 2 |
| `RegAction` | `commit/registry.go` via virt-win-reg | 3 |
| `ExecAction` | `commit/scripts.go` via virt-customize | 4 |

`ExecAction` supports both inline `Content` (commit layer writes to temp
file) and `ScriptPath` (pre-existing host file).

## Probe

Detection (`probe/detect.go`) uses `GuestHandle.IsDir`/`IsFile` to set
flags on `GuestInfo`:

| Probe | Path | Flag |
|-------|------|------|
| Windows | `/Windows/System32` | `OS.Family=windows` |
| os-release | `/etc/os-release` | `OS.Family=linux` |
| ifcfg | `/etc/sysconfig/network-scripts` | `UsesIfcfg` |
| ifcfg (SUSE) | `/etc/sysconfig/network` | `UsesIfcfgSuse` |
| NetworkManager | `/etc/NetworkManager/system-connections` | `UsesNetworkManager` |
| netplan | `/etc/netplan` | `UsesNetplan` |
| ifquery | `/etc/network/interfaces` | `UsesIfquery` |
| interfaces.d | `/etc/network/interfaces.d` | `UsesInterfacesD` |
| wicked | `/etc/wicked` or `/var/lib/wicked` | `UsesWicked` |
| NM DHCP lease | `/var/lib/NetworkManager` | `UsesNMDhcpLease` |
| dhclient | `/var/lib/dhclient` | `UsesDhclient` |
| cloud-init | `/etc/cloud/` or `/usr/bin/cloud-init` | `CloudInit.Present` |
| cloud.cfg | `/etc/cloud/cloud.cfg` | `CloudInit.HasCloudCfg` |
| cloud.cfg.d | `/etc/cloud/cloud.cfg.d/` | `CloudInit.HasCloudCfgD` |
| cloud instance | `/var/lib/cloud/instance/` | `CloudInit.HasInstanceData` |
| cloud seed | `/var/lib/cloud/seed/` | `CloudInit.HasSeedData` |
| sshd | `/usr/sbin/sshd` | `SSH.Present` |
| sshd_config | `/etc/ssh/sshd_config` | `SSH.HasConfig` |
| SSH host keys | `/etc/ssh/ssh_host_*_key` | `SSH.HasHostKeys` |
| root authkeys | `/root/.ssh/authorized_keys` | `SSH.HasRootAuthorizedKeys` |
| GRUB defaults | `/etc/default/grub` | `Console.HasGrubDefaults` |

Extraction (`probe/extract.go`) reads config files via `Cat`/`GlobExpand`
and feeds them to parsers:

### Network (`probe/netcfg/`)

| Parser | Source paths |
|--------|-------------|
| ifcfg | `/etc/sysconfig/network-scripts/ifcfg-*`, `/etc/sysconfig/network/ifcfg-*` |
| NM | `/etc/NetworkManager/system-connections/*` |
| NM DHCP lease | `/var/lib/NetworkManager/*.lease` + `timestamps` |
| dhclient | `/var/lib/dhclient/dhclient-*`, `/var/lib/NetworkManager/dhclient-*` |
| netplan | `/etc/netplan/*.yaml`, `/etc/netplan/*.yml` |
| interfaces | `/etc/network/interfaces`, `/etc/network/interfaces.d/*` |
| wicked | `/var/lib/wicked/lease-*` (XML) |

Each parser produces `[]InterfaceInfo` with interface name, IPs, MAC, and
DHCP flag. A guest may have multiple stacks active simultaneously.

### Cloud-init (`probe/cloudinit/`)

| Field | Source |
|-------|--------|
| `DatasourceList` | `cloud.cfg` + `cloud.cfg.d/*.cfg` YAML |
| `NetworkConfigDisabled` | `network: {config: disabled}` in cloud.cfg |
| `ActiveDatasource` | `/var/lib/cloud/instance/datasource` |
| `InstanceID` | `/var/lib/cloud/instance/instance-id` |

`CloudInitInfo.ManagesNetwork()` returns true when cloud-init is present
and networking is not explicitly disabled -- plugins use this to decide
whether to disable cloud-init networking before migration.

### SSH (`probe/ssh/`)

| Field | Source |
|-------|--------|
| `PermitRootLogin` | `sshd_config` + `sshd_config.d/*.conf` |
| `PasswordAuthentication` | same (first-match-wins) |
| `Port` | same |
| `HostKeyTypes` | filenames from `/etc/ssh/ssh_host_*_key` glob |
| `ServiceEnabled` | `sshd.service` or `ssh.service` in `multi-user.target.wants/` |

### Console (`probe/console/`)

| Field | Source |
|-------|--------|
| `SerialConsoles` | `console=` params from `GRUB_CMDLINE_LINUX*` in `/etc/default/grub` |
| `SerialGettyDevices` | `serial-getty@*.service` in `getty.target.wants/` |

`ConsoleInfo.HasSerialConsole()` returns true if either source detected serial access.

## Adding a plugin

The `plugins/example/` directory is a complete reference implementation.
To create a new plugin:

1. Copy `plugins/example/` to `plugins/<category>/<name>/`.
2. Rename the package.
3. Implement `Applicable` with real conditions (OS family, config flags).
4. Implement `Apply` to return the actions your plugin needs.
5. Place scripts in `scripts/` and embed with `//go:embed scripts`.
6. Register in `resolve.go` -> `AllPlugins()` at the correct position.
7. Add tests in `plugin_test.go`.

Plugin ordering matters: network config before drivers before boot scripts.

Each `plugin.go` should start with a doc comment:

```go
// <plugin-name> -- Short description.
//
// Applicable: <conditions>.
// Output:     <action types and files>.
//
// <Longer explanation.>
package <pkgname>
```

## Adding a probe subsystem

**Network parsers:** Create a parser in `probe/netcfg/` (one Go file per
format), add detection markers in `probe/detect.go`, add extraction logic
in `probe/extract.go`. Network plugins consume `GuestInfo.Interfaces`
uniformly.

**Other subsystems** (cloud-init, SSH, console, etc.): Create a sub-package
under `probe/` with a parser and tests. Add detection flags to the
corresponding struct in `api/types.go`, wire detection in `detect.go`, and
extraction in `extract.go`. All extraction is non-fatal (warnings on read
errors). Windows guests skip Linux-only subsystems.
