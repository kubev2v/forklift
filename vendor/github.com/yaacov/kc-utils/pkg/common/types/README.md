# types -- pipeline JSON handoff structs

Shared across all pipeline stages and the kc-v2v orchestrator. Lives in `common/` because these structs are the contract between separate binaries — kc-prepare writes `PrepareOutput`, kc-convert-* reads it and writes `ConverterOutput`, kc-finalize reads both and writes `TargetMeta`. Placing the definitions in a neutral package prevents circular imports between stages.

Defines the data structures that flow between pipeline stages (kc-prepare, kc-convert-*, kc-finalize) as JSON. Every stage reads a `PipelineData` envelope, adds its output, and writes the envelope back. This package also provides helpers for serializing those structures and expanding a standalone `disk_dir` of `diskN.img` files.

The `PipelineData` envelope contains optional pointers to each stage's input/output: `PrepareInput`, `PrepareOutput`, `ConverterOutput`, and `TargetMeta`. Each of these top-level structs composes smaller types describing disks, partitions, firmware, boot devices, guest capabilities, network mappings, and inspection results. The `WriteJSON` helper marshals any value to indented JSON and writes it to a file. `DiskSpecsFrom` converts a slice of `DiskInfo` to a simpler `DiskSpec` slice for stage input.

## Key exports

| Symbol | Role |
|--------|------|
| `PipelineData` | Top-level envelope accumulating all stage outputs |
| `PrepareInput` | Input to kc-prepare: disks or disk_dir, source, network map, LUKS, options |
| `PrepareOutput` | Output from kc-prepare: inspect data, firmware, boot device, disks |
| `ConverterOutput` | Output from kc-convert: guest capabilities, warnings, errors |
| `TargetMeta` | Output from kc-finalize: target buses, NICs, firmware, boot device |
| `InspectData` | OS inspection results: distro, version, arch, apps, kernel |
| `WindowsInspect` | Windows-specific inspection: system root, hives, drive mappings |
| `GuestCaps` | Guest capability flags: bus types, virtio features, machine type |
| `DiskSpec` | Simplified disk reference (path + format) for pipeline input |
| `DiskInfo` | Full disk metadata including size and partition list |
| `PartitionInfo` | Single partition: index, size, filesystem, mount point, device path |
| `KernelInfo` | Kernel version, path, initrd, modules, virtio/xen flags |
| `BlockError` | Non-fatal error from a conversion block |
| `RootCandidate` | Partition or volume that may contain an OS root |
| `Family` | String enum for Linux distro families (rhel, suse, debian, alt) |
| `FirmwareType` | String enum for firmware type (bios, uefi) |
| `GuestDirEntry` | Directory entry from guest filesystem ReadDir operations |
| `WriteJSON(path, v)` | Marshal a value to indented JSON and write to a file |
| `DiskSpecsFrom(disks)` | Convert `[]DiskInfo` to `[]DiskSpec` |
| `ImageFileName(index)` | Standalone image name `diskN.img` |
| `ExpandDiskDir(dir)` | List `{dir}/diskN.img` regular files as raw `[]DiskSpec`, sorted by N |
