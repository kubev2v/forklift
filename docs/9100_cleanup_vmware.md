# VMware Removal Script — `9100_cleanup_vmware.ps1`

Fully removes VMware Tools and its leftovers from a Windows guest in one
pass. Run **as Administrator**, typically after a physical/`p2v` migration
or a botched VMware Tools uninstall, when Add/Remove Programs and manual
driver removal aren't enough.

## Usage

```powershell
# From an elevated PowerShell prompt
.\9100_cleanup_vmware.ps1
```

- Writes a timestamped log next to the script: `cleanup_vmware.log`.
- Safe to re-run — every phase checks whether its target exists before
  acting.
- A reboot is recommended afterwards; a few locked driver files may be
  scheduled for delete-on-reboot instead of removed immediately.
- Pair with `verify_vmware_cleanup.ps1` to confirm nothing was missed, and
  `syntax_check.ps1` to lint both scripts before running them on a real box.

## Why phase order matters

Components depend on each other, so removing them out of order lets
Windows re-create or veto something already "cleaned":

1. **Services are quiesced first** (Phase 1), before drivers/devices/files
   are touched.
2. **Drivers before devices** (Phase 2 before Phase 3): `vmci` is a PnP bus
   enumerator that can re-assert its device mid-removal, and a driver still
   in the driver store can get re-selected on a PnP rescan.
3. **Service definitions are deleted last** (Phase 4, after devices are
   gone) — deleting a service too early can interfere with the
   device-removal APIs in Phase 3.
4. **`vmci` is force-deleted mid-flow** (start of Phase 3): stopped/disabled
   in Phase 1 like everything else, but only fully deleted right before
   device removal so it can't re-assert its bus device — then excluded
   from the Phase 4 list.

## Script flow (phase by phase)

### Phase 0 — VMware Tools MSI uninstall

Manual file/service/driver deletion never updates Windows Installer's own
bookkeeping, so VMware would keep reappearing in "Programs and Features" /
`Get-Package` forever. This phase uninstalls it properly first:

1. Scans the `Uninstall` registry key (and `WOW6432Node`) for any entry
   whose `DisplayName` matches `VMware`.
2. For each match, extracts the MSI product code and runs
   `msiexec /x {ProductCode} /qn /norestart`.
3. Force-deletes the Add/Remove Programs key afterwards regardless of
   whether msiexec succeeded.
4. Cleans orphaned **Windows Installer product registrations** (drives
   `Get-Package`/`Win32_Product` visibility independently of the Uninstall
   key), across every SID under `UserData`, not just SYSTEM.
5. Cleans orphaned **Windows Installer Components ownership entries**
   (`UserData\<SID>\Components\<ComponentGuid>`) — since a component can be
   shared by multiple products, only the matching value is deleted, and the
   parent key only once it has no owners left.

### Phase 1 — Stop and disable VMware services

1. Looks up every name in `$VMwareServices` (see below) via `Get-Service`.
2. Also scans all services for a `DisplayName`/`Name`/`PathName` matching
   VMware, catching anything not on the static list.
3. For each match: `sc.exe stop`, wait 2s, force-kill if still running,
   then `sc.exe config ... start= disabled`.
4. Services are **not deleted yet** — that's Phase 4.

### Phase 2 — Remove VMware driver packages

1. Prefers `Get-WindowsDriver -Online` (locale-independent — works
   regardless of display language), matching `ProviderName = VMware` or
   `OriginalFileName` against known VMware `.inf` basenames (derived from
   `$VMwareDriverFiles`, plus `vm3d.inf` added explicitly). The `.inf`-name
   match exists because post-Broadcom-acquisition packages get republished
   as `Provider Name: Broadcom Inc.` — matching "Broadcom" alone would be
   unsafe since unrelated Broadcom hardware exists.
2. Falls back to parsing `pnputil /enum-drivers` text output if
   `Get-WindowsDriver` is unavailable (English-only, so this fallback finds
   nothing on a non-English OS).
3. Removes each match with `pnputil /delete-driver <inf> /uninstall /force`.

### Phase 3 — Disable and remove VMware PnP devices

1. Force-stops and deletes the `vmci` service outright (see phase-order
   notes above), then drops it from the pending Phase 4 list.
2. Strips the VMware SVGA class coinstaller (`vm3dc003.dll`) from
   `CoDeviceInstallers` for the Display-adapters class GUID — only the
   matching entry, not the whole `REG_MULTI_SZ` value. A stale coinstaller
   can veto removal of every device in that class.
3. Enumerates PnP devices by `Manufacturer`/`FriendlyName` = VMware,
   VMware's PCI vendor ID (`VEN_15AD`), its USB vendor ID (`VID_0E0F` —
   catches the USB composite parent of the VMware USB mouse, which doesn't
   say VMware itself), or an instance ID starting `ROOT\VMWVMCI`/
   `ROOT\VMware` (ROOT-enumerated software devices that can lack a VMware
   name entirely).
4. For each device: `Disable-PnpDevice`, then `pnputil /remove-device` if
   supported by the OS.
5. Ghost devices (`Status = Unknown`) that survive both are removed via
   `CM_Uninstall_DevNode` — the same API Device Manager's "Uninstall
   device" uses.
6. If that also fails, `Clear-GhostDeviceReferences` cleans secondary
   registry locations before a raw delete of the `Enum\<InstanceId>` key:
   - Deletes the class driver binding key pointed to by the device's
     `Driver` value.
   - Deletes any `DeviceClasses` interface registration whose
     `DeviceInstance` matches this device.
   - `DeviceContainers` references are only **logged**, not deleted — a
     container can be shared by sibling functions of a composite device.

   Leftover references in these locations are a suspected cause of
   Windows resynthesizing a fresh `Unknown` Enum entry for an
   already-deleted device.

### Phase 4 — Delete VMware service definitions

Runs `sc.exe delete` on every service collected in Phase 1 (minus `vmci`,
already deleted in Phase 3).

### Phase 5 — Remove VMware registry entries

1. Deletes every key in `$VMwareRegistryKeys` (see below), plus one
   auto-generated `Services\<name>` key per entry in `$VMwareServices`.
2. Deletes the `VMware Host Open` value under `RegisteredApplications`.
3. Sweeps stray **COM registrations** under `CLSID`/`TypeLib` (native and
   `WOW6432Node`) for GUID subtrees mentioning `VMware`, up to 3 levels
   deep. Uses the .NET registry API directly instead of spawning `reg.exe`
   per GUID — these keys can each have tens of thousands of subkeys.
4. Sweeps `Installer\Folders` for value **names** containing `VMware`
   (records folders an MSI wrote to; not reliably pruned by `msiexec /x`).
5. Sweeps `Run`/`RunOnce` autostart entries for any value whose name or
   data mentions `VMware`.
6. Sweeps `Control\Video\{GUID}\Video` for adapter nodes with a VMware
   `DeviceDesc` or a `Service` starting `vm3dmp` — separate from the
   `vm3dmp_loader` service key already deleted in step 1.

All key/value deletions go through `reg.exe` (not the PowerShell registry
provider) to avoid WOW64 redirection.

### Phase 6 — Remove VMware files and folders

1. Recursively removes every directory in `$VMwareDirs` (see below).
2. Recursively removes folders matching `$VMwareTempDirPatterns`
   (`vmware-*` under `%SystemRoot%\Temp` and `%TEMP%`) — one folder per
   account that's ever run Tools, not just SYSTEM.
3. Recursively removes `AppData\{Local,Roaming,LocalLow}\VMware` under
   every user profile in `C:\Users`, including hidden ones like `Default`.
4. Deletes individual driver files (`$VMwareDriverFiles`), system
   DLLs/EXEs (`$VMwareSystemFiles`), and their 32-bit counterparts
   (`$VMwareSysWOW64Files`).
5. **Reports** (does not delete) any `DriverStore\FileRepository`
   subfolder matching the VMware name pattern (Hyper-V's own `vm*` folders
   are excluded). This store is tracked by Component-Based Servicing;
   `pnputil /delete-driver` (Phase 2) is the supported removal path, and
   force-deleting a folder directly risks CBS inconsistency that can
   surface as `sfc`/`DISM /CheckHealth` corruption later. Anything pnputil
   didn't remove is only logged (`driverStoreResidual`) for manual
   follow-up.

Every removal in this phase (except the DriverStore report) goes through a
shared helper: tries `Remove-Item`, falls back to `cmd /c rd`/`del`, and
for files still locked by a loaded driver, schedules deletion on next boot
via `MoveFileEx(..., MOVEFILE_DELAY_UNTIL_REBOOT)`.

### Phase 7 — Remove VMware scheduled tasks

Finds and `Unregister-ScheduledTask`s any task whose name or path mentions
`VMware`/`vmtools`.

### Summary

Logs and prints final counts for: MSI products uninstalled, MSI Components
removed, driver packages removed, services deleted, stray COM keys removed,
registry keys deleted (+ errors), files/dirs removed (+ errors), and
DriverStore residuals left for manual follow-up.

## Reference data (what's matched/removed)

### Services (`$VMwareServices`)

```
VGAuthService, VM3DService, VMTools, vmvss, GISvc, vmhgfs, vmmemctl,
vmrawdsk, vnetWFP, vnetflt, vsepflt, vmci, vmxnet3, vmxnet3ndis6, pvscsi,
vmusbmouse, vmmouse, vm3dmp, vm3dmp-debug, vm3dmp-stats, vm3dmp_loader,
vsock, efifw, svga_wddm, vmaudio, vgauth, cblauncher, vmwtimeprovider,
vmstatsprovider, vmupgradehelper, vmwefifw,
VMCISockets, vm3dservice, vmxnet, vmx_svga, vmkbd, vmdesched, vmdebug,
vmware, vmx86, VMwareCertService
```

Notes: `vnetflt` is the legacy predecessor of `vnetWFP` and can be left
behind by a Tools installer bug, BSODing alongside `vsepflt` if not
removed. `vgauth`/`cblauncher` are legacy alternate names (no-op if
absent). `vm3dservice` is the service behind `vm3dservice.exe` (already in
`$VMwareSystemFiles`) — without it, deleting the file alone leaves an
orphaned service definition. `vmxnet`, `vmx_svga`, `vmkbd`, `vmdesched`,
`vmdebug`, `vmware` are legacy pre-WDDM names; `vmx86`/`VMwareCertService`
are newer names checked defensively. Each also gets its own
`Services\<name>` registry key removed in Phase 5.

### Explicit registry keys (`$VMwareRegistryKeys`)

Keys not derivable from the service list — mostly shell/COM integration
("Open with VMware Host") and event log source registrations:

```
HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE
HKLM\SOFTWARE\Classes\Applications\VMwareHostOpen.exe
HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile
HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL
HKLM\SOFTWARE\VMware, Inc.
HKLM\SOFTWARE\Classes\CLSID\{C73DA087-EDDB-4a7c-B216-8EF8A3B92C7B}
HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider
HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools
HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider
HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper
HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth
HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools
HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP
HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetflt
HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt
HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vmci
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\vmtoolsd.exe
```

### Directories (`$VMwareDirs`)

```
%ProgramFiles%\VMware
%ProgramFiles%\Common Files\VMware
%ProgramFiles(x86)%\VMware
%ProgramFiles(x86)%\Common Files\VMware
%ProgramData%\VMware
```

...plus wildcarded temp folders (`$VMwareTempDirPatterns`, Phase 6 step 2):

```
%SystemRoot%\Temp\vmware-*
%TEMP%\vmware-*
```

...plus `AppData\{Local,Roaming,LocalLow}\VMware` under every user profile
(Phase 6, step 3).

### Driver files (`$VMwareDriverFiles`, in `drivers`)

```
vmci.sys, vmmouse.sys, vmrawdsk.sys, vmhgfs.sys, vmusbmouse.sys,
pvscsi.sys, vmxnet3.sys, vm3dmp.sys, vm3dmp-debug.sys, vm3dmp-stats.sys,
vm3dmp_loader.sys, vsock.sys, vmmemctl.sys, vsepflt.sys, vnetWFP.sys,
vnetflt.sys, svga_wddm.sys, efifw.sys, vmaudio.sys,
vmx_svga.sys, vmkbd.sys, vmdesched.sys
```

Each `.sys` name is also converted to a matching `.inf` basename for the
Phase 2 driver-package match (plus `vm3d.inf` added explicitly).

### System files (`$VMwareSystemFiles`, in `System32`)

```
vmGuestLib.dll, vmhgfs.dll, vm3dgl64.dll, vm3dver.dll,
vmGuestLibJava.dll, VMWSU.DLL, vm3dc003.dll,
vm3ddevapi64.dll, vm3ddevapi64-debug.dll, vm3ddevapi64-release.dll,
vm3ddevapi64-stats.dll, vm3dglhelper64.dll,
vm3dum64.dll, vm3dum64-debug.dll, vm3dum64-stats.dll,
vm3dum64_10.dll, vm3dum64_10-debug.dll, vm3dum64_10-stats.dll,
vm3dum64_loader.dll, vm3dservice.exe
```

### 32-bit system files (`$VMwareSysWOW64Files`, in `SysWOW64`)

```
vmGuestLib.dll, vmGuestLibJava.dll,
vm3ddevapi.dll, vm3ddevapi-debug.dll, vm3ddevapi-release.dll,
vm3ddevapi-stats.dll, vm3dgl.dll, vm3dglhelper.dll, vm3dservice.exe,
vm3dum.dll, vm3dum-debug.dll, vm3dum-stats.dll,
vm3dum_10.dll, vm3dum_10-debug.dll, vm3dum_10-stats.dll, vm3dum_loader.dll
```

## Other design notes

- **WOW64-safe paths throughout:** file paths resolve via `Sysnative`
  instead of `System32`, and registry reads/writes go through `reg.exe`
  rather than the PowerShell provider, to avoid WOW64 redirection hiding
  the 64-bit view.
- **Idempotent by construction:** almost every action checks whether its
  target exists before touching it, so re-running is safe.
- **Locale-independent where it matters:** Phase 2 prefers
  `Get-WindowsDriver` over `pnputil`'s English-only text output.
- **DriverStore is reported, not force-deleted:** only
  `pnputil /delete-driver` (Phase 2) is a supported removal path; leftover
  folders are logged for manual follow-up instead.
- **Ghost-device cleanup goes beyond the `Enum` key:** Phase 3 also clears
  the class driver binding and `DeviceClasses` registration, and reports
  (without deleting) any `DeviceContainers` reference — see Phase 3 step 6.

## `verify_vmware_cleanup.ps1`

Kept in sync with this script's data lists so it can confirm nothing was
missed. It also runs `DISM /Online /Cleanup-Image /CheckHealth` at the
end, since Phase 6 no longer force-deletes `DriverStore` folders — that
check catches any resulting (or unrelated) component-store corruption.
`sfc /scannow` was deliberately left out: it can take 10-20+ minutes on a
real VM, disproportionate to what this script verifies; run it manually
if `DISM /CheckHealth` flags corruption.
