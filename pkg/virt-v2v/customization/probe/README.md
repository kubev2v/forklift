# probe — Guest Disk Network Probe

The `probe` package runs read-only `guestfish` commands against a guest disk
image to detect the OS type, identify which networking stacks are installed,
and extract interface configuration (names, IPs, MACs).

## Architecture

Probing runs in two sequential guestfish sessions:

```
 ┌─────────────────────────────────────────────────┐
 │  Phase 1 — Detection (detect.go)                │
 │                                                 │
 │  is-dir / is-file probes → UsesIfcfg, etc.      │
 └──────────────────────┬──────────────────────────┘
                        │ GuestInfo (flags only)
 ┌──────────────────────▼──────────────────────────┐
 │  Phase 2 — Extraction (extract.go + parse.go)   │
 │                                                 │
 │  cat / glob-cat config files → parse into       │
 │  InterfaceInfo{Name, IPv4, IPv6, MAC, DHCP, …}  │
 └─────────────────────────────────────────────────┘
```

**probe.go** orchestrates both phases and builds the guestfish command line.

## Detection Probes (detect.go)

| Name              | Command                                 | Sets              |
|-------------------|-----------------------------------------|--------------------|
| `windows`         | `is-dir /Windows/System32`              | OS.Family=windows  |
| `os-release`      | `is-file /etc/os-release`               | OS.Family=linux    |
| `ifcfg`           | `is-dir /etc/sysconfig/network-scripts` | UsesIfcfg          |
| `ifcfg-suse`      | `is-dir /etc/sysconfig/network`         | UsesIfcfgSuse      |
| `network-manager` | `is-dir /etc/NetworkManager/system-connections` | UsesNetworkManager |
| `netplan`         | `is-dir /etc/netplan`                   | UsesNetplan        |
| `ifquery`         | `is-file /etc/network/interfaces`       | UsesIfquery        |
| `interfaces-d`    | `is-dir /etc/network/interfaces.d`      | UsesInterfacesD    |
| `wicked-etc`      | `is-dir /etc/wicked`                    | UsesWicked         |
| `wicked-var`      | `is-dir /var/lib/wicked`                | UsesWicked         |

Notes:
- A guest may have multiple stacks (e.g. both ifcfg and NM on RHEL 7).
- `UsesWicked` is true if **either** wicked path exists.
- SUSE uses `/etc/sysconfig/network/ifcfg-*` (no `network-scripts/`);
  `UsesIfcfgSuse` covers this path separately.

## Extraction & Parsing (extract.go, parse.go)

Each detected stack triggers a glob-cat of its config files, delimited by
start/end markers (e.g. `===IFCFG_START===` / `===IFCFG_END===`). The parsers are
intentionally simple line-scanners, not full config parsers — they extract
only the fields needed for migration (interface name, IPs, MACs).

### ifcfg — `parseIfcfgSection`

**Source paths:**
- RHEL/CentOS: `/etc/sysconfig/network-scripts/ifcfg-*`
- SUSE: `/etc/sysconfig/network/ifcfg-*`

Reads `DEVICE`, `IPADDR`/`IPADDRn` (→ `IPv4`), `IPV6ADDR` and
`IPV6ADDR_SECONDARIES` (→ `IPv6`), `HWADDR`, `NAME`, and `BOOTPROTO`
from ifcfg files. Files are separated by blank lines in the concatenated
glob output. Both the RHEL and SUSE paths are probed independently.
`BOOTPROTO=dhcp` sets `DHCP: true` on the interface.

**Known limitations vs. the ifcfg spec:**

- **Alias files (`ifcfg-eth0:1`) are included** by the glob but parsed
  correctly since they use the same key-value format.

### NetworkManager — `parseNMSection`

**Source path:** `/etc/NetworkManager/system-connections/*`

Reads `interface-name`, `mac-address`, and `addressN` (e.g. `address1=IP/prefix`)
from NM keyfile-format profiles.

**Known limitations vs. the NM keyfile spec:**

- **Multi-file boundaries use `[connection]` as separator.** Each NM keyfile
  starts with a `[connection]` section. When `glob cat` concatenates multiple
  files, the parser emits a new `InterfaceInfo` each time it sees
  `[connection]`, correctly splitting multiple profiles.

- **`method=auto` sets `DHCP: true`** on the interface. DHCP-only interfaces
  produce an entry with a name but no IPs.

- **IPv4 and IPv6 are separated** by tracking the current `[ipv4]`/`[ipv6]`
  section header. Addresses under `[ipv4]` go to `IPv4`, addresses under
  `[ipv6]` go to `IPv6`.

### Netplan — `parser.Netplan`

**Source path:** `/etc/netplan/*.yaml`

Uses `gopkg.in/yaml.v2` to unmarshal netplan YAML into typed Go structs.
Extracts interface names, addresses (CIDR stripped to IP, classified
into `IPv4`/`IPv6` by `:` presence), `match.macaddress`,
`dhcp4`/`dhcp6` (→ `DHCP: true`), and `renderer`
(→ `GuestInfo.NetplanRenderer`) from `ethernets`, `bonds`, `bridges`,
and `vlans` mappings. Multiple concatenated YAML documents (from
`glob cat`) are split on `---` separators and parsed independently.
Both `*.yaml` and `*.yml` files are globbed.

### interfaces — `parseInterfacesSection`

**Source paths:**
- `/etc/network/interfaces` (main file)
- `/etc/network/interfaces.d/*` (if the directory exists)

Reads `iface <name> inet ...` stanzas and `address` lines from the Debian
ifupdown format. Both the main file and the `interfaces.d/` directory are
probed and concatenated, so split configurations are handled.

**Known limitations vs. interfaces(5):**

- **`source` directives inside files are not followed.** The probe reads
  `interfaces.d/*` directly via guestfish glob rather than parsing `source`
  lines. Files sourced from non-standard paths will be missed.

- **IPv4 and IPv6 are separated** by the address family keyword in the
  `iface` line (`inet` → `IPv4`, `inet6` → `IPv6`).

- **`address` with CIDR notation** (`address 192.168.1.1/24`) is handled
  correctly (the `/` is stripped).

- **`gateway`, `netmask`, `dns-*` are not extracted.** Only the interface
  name and address are captured.

### Wicked — `parser.Wicked`

**Source paths:**
- `ls /var/lib/wicked/` (file listing for interface name extraction)
- `glob cat /var/lib/wicked/lease-*` (DHCP lease XML content)

Uses `encoding/xml` to parse wicked DHCP lease files. Interface names
and address families are extracted from filenames
(e.g. `lease-eth0-dhcp-ipv4.xml` → name=`eth0`, family=`ipv4`).
IP addresses are extracted from `<address>` elements in the XML and
placed into `IPv4` or `IPv6` based on the filename family.
The extraction only globs `lease-*` files, filtering out non-lease files
like `duid.xml`.

**Note:** Wicked's static configuration lives in `/etc/sysconfig/network/ifcfg-*`,
which is handled separately by the ifcfg parser via `UsesIfcfgSuse`. A SUSE
guest with both static and DHCP interfaces will have both `UsesIfcfgSuse=true`
and `UsesWicked=true`, so both parsers contribute interfaces.

## Remaining Limitations

| Limitation | Severity | Affected Distros |
|------------|----------|------------------|
| interfaces: non-standard `source` paths not followed | Low | Debian with custom source directives |
