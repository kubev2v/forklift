# pkg/copy — NFC disk copy

Standalone vSphere disk copy via govmomi NFC (Network File Copy) export.
Downloads VMDK disks from vCenter/ESXi over HTTPS — no VMware VDDK library
required — and writes raw disk images to PVC targets (block devices or
filesystem images) or to `{target_dir}/diskN.img`.

This package implements the `kc-copy` binary's core logic.

## How it works

1. **Target discovery** — if `CopyInput.target_dir` is set, writes
   `{target_dir}/diskN.img` (raw format) and skips PVC discovery. Otherwise
   scans `/dev/block[0-9]*` (block devices) and `/mnt/disks/disk[0-9]*`
   (filesystem mounts) for conversion-pod PVCs. Filters to empty targets
   (block devices whose first 1 MiB is all zeros — probed by read, since
   Linux `stat` reports size 0 for block devices — or filesystem images
   smaller than 1 MiB).

2. **NFC export** — connects to vSphere using credentials from
   `/etc/secret/accessKeyId` and `/etc/secret/secretKey`, locates the VM by
   name, and starts an NFC export lease via govmomi. vCenter SDK TLS comes from
   `CopyInput.insecure` and `CopyInput.ca_cert` (`pkg/v2v/tls.CopyTLS`), with
   optional vCenter thumbprint fallback (`fingerprint`). ESXi NFC disk
   downloads reuse the govmomi client; ESXi host thumbprints from the lease
   are registered during export (see `nfc.go`).

3. **Disk matching** — filters the NFC lease URLs to the requested source
   VMDK paths (snapshot delta suffixes like `-000001.vmdk` are normalized to
   base `.vmdk` names). Empty `source_disks` selects every lease disk. PVC
   mode validates that the number of selected source disks matches the number
   of empty targets.

4. **Concurrent copy** — downloads disks in parallel (default concurrency 4,
   bounded by a semaphore). Each disk is streamed through `StreamToRaw`,
   which decompresses the stream-optimized VMDK format on the fly and writes
   sparse raw output (zero regions are skipped via seek).

5. **Progress tracking** — writes a `copy-progress.json` file after each disk
   completes. On failure of any disk, cancels all remaining copies and aborts
   the NFC lease.

## VMDK stream-to-raw conversion

`StreamToRaw` reads a stream-optimized VMDK (the format vSphere uses for NFC
export) and produces a raw disk image:

- Parses the 512-byte VMDK header to extract capacity, grain size, and
  overhead.
- Iterates grain markers: compressed grains are decompressed via zlib and
  written at their LBA offset; metadata markers (grain table, grain directory,
  footer) are skipped.
- Reuses a single zlib reader across grains to avoid per-grain allocation of
  the internal flate dictionary (~44 KiB each).
- Safety caps: grain size limited to 64 MiB, compressed buffer limited to 2x
  grain size.

## Page cache management (Linux)

On Linux, the `drainWriter` wrapper periodically calls `fdatasync` +
`fadvise(FADV_DONTNEED)` (every 32 MiB written) to release page cache back to
the kernel. Without this, the cgroup memory usage grows by the total amount
written (3-4 GiB for typical VM disks), which can cause OOM kills in
memory-constrained pods.

## File layout

| File | Role |
|------|------|
| `copy.go` | Entry point (`Run`), target validation, concurrent copy orchestration |
| `vmdk.go` | `StreamToRaw` — VMDK decompression and raw disk writing |
| `vsphere.go` | vSphere connection, NFC lease management, disk URL mapping |
| `nfc.go` | ESXi NFC HTTPS download via govmomi lease thumbprints |
| `download.go` | NFC download orchestration, `CopyDisk`, progress logging |
| `filter.go` | VMDK path normalization and NFC lease filtering |
| `target.go` | PVC target discovery (`DiscoverTargets`, `EmptyTargets`) and `TargetsFromDir` |
| `drain_linux.go` | Linux page-cache drain via fdatasync + fadvise |
| `drain_other.go` | No-op drain for non-Linux platforms |

Import path: `github.com/yaacov/kc-utils/pkg/copy`
