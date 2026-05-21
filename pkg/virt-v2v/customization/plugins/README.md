# plugins — Guest Customization Plugins

The `plugins` package implements modular, ordered customization steps that
run after virt-v2v converts a VM disk. Each plugin decides whether it applies
to a given guest and, if so, returns file and/or exec actions.

## Architecture

```
 ┌──────────────────────────────────────────────────────────┐
 │  resolve.go — AllPlugins() / Resolve(ctx)                │
 │                                                          │
 │  Returns plugins in fixed order; Resolve filters to      │
 │  those where Applicable(ctx) == true.                    │
 └──────────────────────┬───────────────────────────────────┘
                        │ []api.Plugin
 ┌──────────────────────▼───────────────────────────────────┐
 │  customize.go — Run(opts)                                │
 │                                                          │
 │  Calls Apply(ctx) on each plugin, collecting:            │
 │    • FileAction  → guestfish --rw (upload / write)       │
 │    • ExecAction  → virt-customize (--firstboot / --run)  │
 └──────────────────────────────────────────────────────────┘
```

### Plugin interface

```go
type Plugin interface {
    Name() string
    Applicable(ctx *Context) bool
    Apply(ctx *Context) (*Actions, error)
}
```

### Ordering

Plugins are evaluated in the order returned by `AllPlugins()`:

1. `network/udev` — Linux NIC renaming
2. `network/registry` — Windows static IPs (modern)
3. `network/netsh` — Windows static IPs (legacy)
4. `drivers/vmware` — VMware driver removal
5. `scripts/dynamic` — User-supplied scripts
6. `boot/windows/firstboot-runner` — .bat orchestrator
7. `boot/windows/disk-restore` — VirtIO disk restore
8. `boot/windows/conversion-done` — COM1 signal

Network config runs first so IPs are set before boot scripts execute.
The firstboot runner must come after all `.ps1` scripts are written.
`conversion-done` runs last so it signals only after everything else completes.

## Plugin file header convention

Each `plugin.go` file carries a package-level doc comment that documents
the plugin's name, applicability conditions, output actions, and a brief
description. This keeps the reference information next to the code it
describes. The format is:

```go
// <plugin-name> — Short description.
//
// Applicable: <conditions under which the plugin fires>.
// Output:     <action types and file names produced>.
//
// <Longer explanation of what the plugin does.>
package <pkgname>
```

When adding a new plugin, follow this convention so the documentation
stays co-located with the implementation.

## Embedded Scripts

Each plugin that writes files to the guest embeds its scripts via `//go:embed scripts`.
Templates (`.ps1.tmpl`) use Go `text/template` with `{{.InputString}}` for the
static IPs string. Static scripts are written verbatim.

| Plugin | Embedded scripts |
|--------|-----------------|
| network/registry | `100_network_config.ps1.tmpl`, `110_complementary_ips.ps1.tmpl`, `120_remove_duplicate_routes.ps1` |
| network/netsh | `100_network_config.ps1.tmpl`, `120_remove_duplicate_routes.ps1` |
| drivers/vmware | `100_disable_services.bat` through `104_remove_registry.bat` |
| boot/firstboot | `900_firstboot_init.bat`, `900_firstboot_init_legacy.bat` |
| boot/diskrestore | `200_restore_config_legacy.ps1` |
| boot/conversiondone | `990_signal_conversion_done.ps1` |
