# commit — Guest Disk Action Commit

The `commit` package materializes the collected plugin actions onto a guest
disk image. It is the final phase of customization: plugins decide *what*
to do, and commit writes it to the guest.

## Architecture

```
 ┌──────────────────────────────────────────────────────────┐
 │  files.go — commit.Files(...)                            │
 │                                                          │
 │  Runs a single guestfish --rw session that:              │
 │    • mkdir-p parent directories                          │
 │    • upload  (host file → guest path)                    │
 │    • write   (in-memory content → guest path)            │
 │    • chmod   (optional permissions)                      │
 └──────────────────────────────────────────────────────────┘

 ┌──────────────────────────────────────────────────────────┐
 │  scripts.go — commit.Scripts(...)                        │
 │                                                          │
 │  Runs virt-customize with:                               │
 │    • --firstboot  (run script on next guest boot)        │
 │    • --run        (run script immediately in the guest)  │
 └──────────────────────────────────────────────────────────┘
```

## Functions

### Files

```go
func Files(cmdBuilder, disks, keys, rootDisk, actions) error
```

Builds a guestfish script from `[]api.FileAction` and executes it in a
single `guestfish --rw` session. Parent directories are created
automatically. LUKS keys are forwarded so encrypted volumes can be
written to.

### Scripts

```go
func Scripts(cmdBuilder, disks, keys, actions) error
```

Translates `[]api.ExecAction` into `virt-customize` flags (`--firstboot`
or `--run`) and runs the command. Skipped entirely when the action list
is empty.

## Relationship to other packages

| Package | Role |
|---------|------|
| `plugins` | Produce `[]FileAction` and `[]ExecAction` |
| `commit` | Consume those actions and write them to the guest disk |
| `probe` | Read-only guest inspection (runs before plugins) |

The orchestrator in `customize.go` connects these three phases:
probe → plugins → commit.
