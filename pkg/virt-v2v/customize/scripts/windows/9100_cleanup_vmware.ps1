# 9100_cleanup_vmware.ps1
# Complete VMware cleanup: PnP devices, driver packages, services,
# registry entries, files/folders, and scheduled tasks.
# Run as Administrator.

$ErrorActionPreference = 'Continue'

# Schedules a locked file for deletion on next reboot (last resort).
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class LockedFileDeleter {
    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    public static extern bool MoveFileEx(
        string lpExistingFileName, string lpNewFileName, int dwFlags);
    public const int MOVEFILE_DELAY_UNTIL_REBOOT = 0x4;
}

// CM_Uninstall_DevNode: same API Device Manager uses to remove a
// device. Safer than deleting the Enum registry key by hand.
public static class CfgMgr {
    public const uint CM_LOCATE_DEVNODE_PHANTOM = 0x00000001;
    public const uint CR_SUCCESS = 0x00000000;

    [DllImport("cfgmgr32.dll", CharSet = CharSet.Unicode, SetLastError = true,
        EntryPoint = "CM_Locate_DevNodeW")]
    public static extern uint CM_Locate_DevNode(
        out uint pdnDevInst, string pDeviceID, uint ulFlags);

    [DllImport("cfgmgr32.dll", SetLastError = true,
        EntryPoint = "CM_Uninstall_DevNode")]
    public static extern uint CM_Uninstall_DevNode(uint dnDevInst, uint ulFlags);
}
"@

# Removes a ghost/non-present device instance. Returns $true on success.
function Remove-GhostDevNode {
    param([string]$InstanceId)
    $devInst = 0
    $cr = [CfgMgr]::CM_Locate_DevNode([ref]$devInst, $InstanceId, [CfgMgr]::CM_LOCATE_DEVNODE_PHANTOM)
    if ($cr -ne [CfgMgr]::CR_SUCCESS) { return $false }
    $cr2 = [CfgMgr]::CM_Uninstall_DevNode($devInst, 0)
    return ($cr2 -eq [CfgMgr]::CR_SUCCESS)
}

# ===================================================================
# WOW64-safe path resolution
# ===================================================================
# Sysnative avoids System32 being redirected to SysWOW64 if this runs
# as a 32-bit process.
$Sysnative = Join-Path $env:SystemRoot 'Sysnative'
if (Test-Path $Sysnative) {
    $Sys32   = $Sysnative
    $Drivers = Join-Path $Sysnative 'drivers'
} else {
    $Sys32   = Join-Path $env:SystemRoot 'System32'
    $Drivers = Join-Path $env:SystemRoot 'System32\drivers'
}
$PnpUtil = Join-Path $Sys32 'pnputil.exe'
$RegExe  = Join-Path $Sys32 'reg.exe'

# ===================================================================
# Data declarations - single source of truth
# ===================================================================

# pvscsi is deliberately NOT in this list. On at least one guest OS
# (a Windows Server 2025 Insider Build image) it turned out to still be
# the live boot-critical storage controller driver post-conversion -
# deleting its service/file pulled the disk driver Windows was actually
# booting from out from under it, bricking the boot volume (Stop code
# INACCESSIBLE_BOOT_DEVICE). virt-v2v is supposed to have already
# switched the boot controller to VirtIO by the time this script runs,
# but that isn't guaranteed for every guest OS it doesn't fully
# recognize. Leaving pvscsi installed (inert, cosmetic leftover on
# guests where the VirtIO swap did happen) is simpler and safer than
# risking the boot device, so every phase below explicitly excludes it.
$VMwareServices = @(
    'VGAuthService', 'VM3DService', 'VMTools', 'vmvss', 'GISvc',
    'vmhgfs', 'vmmemctl', 'vmrawdsk', 'vnetWFP', 'vnetflt', 'vsepflt',
    'vmci', 'vmxnet3', 'vmxnet3ndis6',
    'vmusbmouse', 'vmmouse',
    'vm3dmp', 'vm3dmp-debug', 'vm3dmp-stats', 'vm3dmp_loader',
    # vgauth/cblauncher: legacy alternate names, no-op if absent.
    'vsock', 'efifw', 'svga_wddm', 'vmaudio', 'vgauth', 'cblauncher',
    'vmwtimeprovider', 'vmstatsprovider', 'vmupgradehelper',
    'vmwefifw',
    # vm3dservice backs vm3dservice.exe below; rest are newer/legacy names.
    'VMCISockets', 'vm3dservice', 'vmxnet', 'vmx_svga', 'vmkbd',
    'vmdesched', 'vmdebug', 'vmware', 'vmx86', 'VMwareCertService'
)

# Registry keys that are NOT auto-derived from service names.
$VMwareRegistryKeys = @(
    'HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE',
    'HKLM\SOFTWARE\Classes\Applications\VMwareHostOpen.exe',
    'HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile',
    'HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL',
    'HKLM\SOFTWARE\VMware, Inc.',
    'HKLM\SOFTWARE\Classes\CLSID\{C73DA087-EDDB-4a7c-B216-8EF8A3B92C7B}',
    'HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider',
    'HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools',
    'HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider',
    'HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper',
    'HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth',
    'HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools',
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP',
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetflt',
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt',
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vmci',
    # App Paths entry for vmtoolsd.exe - orphaned once the exe is gone.
    'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\vmtoolsd.exe'
)

$VMwareDirs = @(
    "$env:ProgramFiles\VMware",
    "$env:ProgramFiles\Common Files\VMware",
    "${env:ProgramFiles(x86)}\VMware",
    "${env:ProgramFiles(x86)}\Common Files\VMware",
    "$env:ProgramData\VMware"
)

# Handled separately from $VMwareDirs: Tools creates one
# "vmware-<account>" temp folder per account that's run it.
$VMwareTempDirPatterns = @(
    "$env:SystemRoot\Temp\vmware-*",
    "$env:TEMP\vmware-*"
)

# pvscsi.sys intentionally excluded - see the $VMwareServices comment above.
$VMwareDriverFiles = @(
    'vmci.sys', 'vmmouse.sys', 'vmrawdsk.sys', 'vmhgfs.sys',
    'vmusbmouse.sys', 'vmxnet3.sys',
    'vm3dmp.sys', 'vm3dmp-debug.sys', 'vm3dmp-stats.sys', 'vm3dmp_loader.sys',
    'vsock.sys', 'vmmemctl.sys', 'vsepflt.sys', 'vnetWFP.sys', 'vnetflt.sys',
    'svga_wddm.sys', 'efifw.sys', 'vmaudio.sys',
    # Legacy pre-WDDM driver files.
    'vmx_svga.sys', 'vmkbd.sys', 'vmdesched.sys'
)

$VMwareSystemFiles = @(
    'vmGuestLib.dll', 'vmhgfs.dll', 'vm3dgl64.dll', 'vm3dver.dll',
    'vmGuestLibJava.dll', 'VMWSU.DLL',
    'vm3dc003.dll',
    'vm3ddevapi64.dll', 'vm3ddevapi64-debug.dll',
    'vm3ddevapi64-release.dll', 'vm3ddevapi64-stats.dll',
    'vm3dglhelper64.dll',
    'vm3dum64.dll', 'vm3dum64-debug.dll', 'vm3dum64-stats.dll',
    'vm3dum64_10.dll', 'vm3dum64_10-debug.dll', 'vm3dum64_10-stats.dll',
    'vm3dum64_loader.dll',
    'vm3dservice.exe'
)

$VMwareSysWOW64Files = @(
    'vmGuestLib.dll', 'vmGuestLibJava.dll',
    'vm3ddevapi.dll', 'vm3ddevapi-debug.dll',
    'vm3ddevapi-release.dll', 'vm3ddevapi-stats.dll',
    'vm3dgl.dll', 'vm3dglhelper.dll',
    'vm3dservice.exe',
    'vm3dum.dll', 'vm3dum-debug.dll', 'vm3dum-stats.dll',
    'vm3dum_10.dll', 'vm3dum_10-debug.dll', 'vm3dum_10-stats.dll',
    'vm3dum_loader.dll'
)

# DriverStore dirs matching this pattern are VMware, Hyper-V dirs
# (vmbus, vmgid, vmgen*, vmstor*, vms3cap, vmbk*) are excluded.
$DriverStorePattern = '^vm(3d|ci|hgfs|mouse|rawdsk|memctl|xnet|ware|tools|vss|usb)'

# ===================================================================
# Helper functions
# ===================================================================

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$LogFile   = Join-Path $ScriptDir 'cleanup_vmware.log'

function Log {
    param([string]$msg)
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    $line = "[$ts] $msg"
    Write-Host $line
    Add-Content -Path $LogFile -Value $line
}

# Delete a registry key tree via reg.exe (avoids WOW64 redirection).
function Delete-RegKey {
    param([string]$KeyPath)
    & $RegExe query $KeyPath /ve 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { return }
    $result = & $RegExe delete $KeyPath /f 2>&1 | Out-String
    if ($LASTEXITCODE -eq 0) {
        Log "[DEL KEY] $KeyPath"
        $script:regDeleted++
    } else {
        Log "[WARNING] reg.exe delete failed: $KeyPath - $result"
        $script:regErrors++
    }
}

# Delete a single registry value via reg.exe.
function Delete-RegValue {
    param([string]$KeyPath, [string]$ValueName)
    & $RegExe query $KeyPath /v $ValueName 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { return }
    $result = & $RegExe delete $KeyPath /v $ValueName /f 2>&1 | Out-String
    if ($LASTEXITCODE -eq 0) {
        Log "[DEL VAL] $KeyPath\$ValueName"
        $script:regDeleted++
    } else {
        Log "[WARNING] reg.exe delete value failed: $KeyPath\$ValueName"
        $script:regErrors++
    }
}

# List immediate subkey paths of a key (WOW64-safe).
function Get-RegSubKeyPaths {
    param([string]$KeyPath)
    $out = & $RegExe query $KeyPath 2>&1
    if ($LASTEXITCODE -ne 0) { return @() }
    $out |
        Where-Object { $_ -match '^HKEY_LOCAL_MACHINE\\' } |
        ForEach-Object { ($_ -replace '^HKEY_LOCAL_MACHINE\\', 'HKLM\').Trim() } |
        Where-Object { $_ -ne $KeyPath }
}

# Read a single string value from a key via reg.exe.
function Get-RegValue {
    param([string]$KeyPath, [string]$ValueName)
    $out = & $RegExe query $KeyPath /v $ValueName 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { return $null }
    $m = [regex]::Match($out, [regex]::Escape($ValueName) + '\s+REG_\w+\s+(.+)')
    if ($m.Success) { return $m.Groups[1].Value.Trim() }
    return $null
}

# .NET registry reads used for in-process tree scans (DeviceClasses,
# CLSID/TypeLib). These throw a terminating SecurityException/
# UnauthorizedAccessException on a restricted-ACL key, which neither
# $ErrorActionPreference value suppresses - swallow it here so one
# unreadable key doesn't abort the whole scan.
function Get-SafeSubKeyNames {
    param([Microsoft.Win32.RegistryKey]$Key)
    try { return $Key.GetSubKeyNames() } catch { return @() }
}
function Open-SafeSubKey {
    param([Microsoft.Win32.RegistryKey]$Key, [string]$Name)
    try { return $Key.OpenSubKey($Name) } catch { return $null }
}
function Get-SafeRegValue {
    param([Microsoft.Win32.RegistryKey]$Key, [string]$Name)
    try { return $Key.GetValue($Name) } catch { return $null }
}

# Removes a file/dir, falling back to cmd /c rd/del, then scheduling
# locked files for deletion on reboot.
function Remove-PathItem {
    param([string]$Path, [switch]$Recurse)
    if (-not (Test-Path $Path)) { return }
    try {
        if ($Recurse) {
            Remove-Item -Path $Path -Recurse -Force -ErrorAction Stop
        } else {
            Remove-Item -Path $Path -Force -ErrorAction Stop
        }
    } catch {
        if ($Recurse) {
            & cmd.exe /c rd /s /q "$Path" 2>&1 | Out-Null
        } else {
            & cmd.exe /c del /f /q "$Path" 2>&1 | Out-Null
        }
    }
    if (-not (Test-Path $Path)) {
        Log "[DEL] $Path"
        $script:fileDeleted++
        return
    }
    # File is locked (e.g. loaded driver) - schedule for reboot deletion.
    if (-not $Recurse) {
        # Resolve back to System32 so the kernel can find the file.
        $kernelPath = $Path -replace '(?i)\\Sysnative\\', '\System32\'
        # [NullString]::Value, not $null - PowerShell marshals bare $null
        # as "", which MoveFileEx treats as a failing rename instead of delete.
        $ok = [LockedFileDeleter]::MoveFileEx($kernelPath, [NullString]::Value,
                [LockedFileDeleter]::MOVEFILE_DELAY_UNTIL_REBOOT)
        if ($ok) {
            Log "[DEL-REBOOT] $Path (scheduled for deletion on reboot)"
            $script:fileDeleted++
            return
        }
    }
    Log "[WARNING] Could not remove: $Path"
    $script:fileErrors++
}

# ===================================================================
# Main
# ===================================================================

Log '==============================================================='
Log '  VMware Full Cleanup Script'
Log '==============================================================='
Log "[INFO] Sys32=$Sys32  PnpUtil=$PnpUtil  RegExe=$RegExe"

# -------------------------------------------------------------------
# PHASE 0: Uninstall VMware Tools via Windows Installer (MSI)
# -------------------------------------------------------------------
# Uninstall via MSI first so it doesn't linger in Programs and Features,
# then force-delete any registry remnants msiexec leaves behind.
Log ''
Log '  PHASE 0: VMware Tools MSI Uninstall'
Log '---------------------------------------------------------------'

# Counters used by Delete-RegKey/Value, first called below.
$regDeleted = 0
$regErrors  = 0
$msiRemoved = 0
$UninstallRoots = @(
    'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
    'HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall'
)

$vmwareUninstallKeys = foreach ($root in $UninstallRoots) {
    foreach ($sub in (Get-RegSubKeyPaths $root)) {
        $displayName = Get-RegValue $sub 'DisplayName'
        if ($displayName -match 'VMware') {
            [PSCustomObject]@{
                KeyPath         = $sub
                DisplayName     = $displayName
                UninstallString = Get-RegValue $sub 'UninstallString'
            }
        }
    }
}

if (-not $vmwareUninstallKeys) {
    Log '[INFO] No VMware entries in Add/Remove Programs.'
} else {
    foreach ($entry in $vmwareUninstallKeys) {
        $productCode = $null
        if ($entry.KeyPath -match '(\{[0-9A-Fa-f\-]{36}\})$') { $productCode = $Matches[1] }
        elseif ($entry.UninstallString -match '(\{[0-9A-Fa-f\-]{36}\})') { $productCode = $Matches[1] }

        if ($productCode) {
            Log "[MSI] Uninstalling $($entry.DisplayName) $productCode ..."
            try {
                # Bounded wait (10 min): this runs unattended on firstboot, so
                # a stuck msiexec (e.g. blocked on the Installer mutex) must
                # not hang Phases 1-7 forever. 3010/1641 mean success with a
                # reboot pending, not failure.
                $p = Start-Process -FilePath 'msiexec.exe' `
                    -ArgumentList "/x $productCode /qn /norestart" `
                    -PassThru -ErrorAction Stop
                if (-not $p.WaitForExit(600000)) {
                    Log "[WARNING] msiexec timed out for $($entry.DisplayName) - killing and forcing registry cleanup"
                    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
                } elseif ($p.ExitCode -in 0, 1641, 3010) {
                    Log "[SUCCESS] msiexec removed $($entry.DisplayName)"
                    $msiRemoved++
                } else {
                    Log "[WARNING] msiexec exited $($p.ExitCode) for $($entry.DisplayName) - forcing registry cleanup"
                }
            } catch {
                Log "[WARNING] msiexec failed to launch for $($entry.DisplayName): $_"
            }
        } else {
            Log "[WARNING] No product code found for $($entry.DisplayName) - forcing registry cleanup"
        }

        # Ensure the Add/Remove Programs entry is gone even if msiexec left it.
        Delete-RegKey $entry.KeyPath
    }
}

# Clean orphaned Windows Installer product registrations (drives
# Get-Package/Win32_Product visibility) across every SID under UserData.
$UserDataRoot = 'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData'
$productsRoots = @('HKLM\SOFTWARE\Classes\Installer\Products')
foreach ($sidKey in (Get-RegSubKeyPaths $UserDataRoot)) {
    $productsRoots += "$sidKey\Products"
}

foreach ($productsRoot in $productsRoots) {
    foreach ($sub in (Get-RegSubKeyPaths $productsRoot)) {
        $name = Get-RegValue $sub 'ProductName'
        if (-not $name) { $name = Get-RegValue "$sub\InstallProperties" 'DisplayName' }
        if ($name -match 'VMware') {
            Delete-RegKey $sub
        }
    }
}

Log "[INFO] VMware MSI products uninstalled via msiexec: $msiRemoved"

# Clean orphaned Windows Installer "Components" ownership entries
# (UserData\<SID>\Components\<ComponentGuid>) - only the matching value
# is deleted (shared by products), then the key if empty.
function Get-RegDataMatches {
    param([string]$KeyPath, [string]$Pattern)
    $out = & $RegExe query $KeyPath /f $Pattern /d /s 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { return @() }
    $results = New-Object System.Collections.Generic.List[object]
    $currentKey = $null
    foreach ($line in ($out -split "`r?`n")) {
        if ($line -match '^HKEY_LOCAL_MACHINE\\(.+)$') {
            $currentKey = "HKLM\$($Matches[1])"
        } elseif ($currentKey -and $line -match '^\s+(\S+)\s+REG_\w+\s+.*') {
            $results.Add([PSCustomObject]@{ KeyPath = $currentKey; ValueName = $Matches[1] })
        }
    }
    return $results
}

$componentsRemoved = 0
foreach ($sidKey in (Get-RegSubKeyPaths $UserDataRoot)) {
    $componentsRoot = "$sidKey\Components"
    foreach ($hit in (Get-RegDataMatches $componentsRoot 'VMware')) {
        Delete-RegValue $hit.KeyPath $hit.ValueName
        $componentsRemoved++
        $remaining = & $RegExe query $hit.KeyPath 2>&1 | Out-String
        if ($LASTEXITCODE -eq 0 -and $remaining -notmatch 'REG_') {
            Delete-RegKey $hit.KeyPath
        }
    }
}
Log "[INFO] Orphaned Windows Installer Components entries removed: $componentsRemoved"

# -------------------------------------------------------------------
# PHASE 1: Stop and disable VMware services
# -------------------------------------------------------------------
# Order matters: quiesce services, then drivers, then devices; delete
# service definitions last (Phase 4) - otherwise vmci can re-assert its
# device and stale drivers can get re-selected on a PnP rescan.
Log ''
Log '  PHASE 1: Stop and Disable VMware Services'
Log '---------------------------------------------------------------'

$svcDeleted = 0
$servicesToRemove = @()

foreach ($name in $VMwareServices) {
    $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
    if ($svc) { $servicesToRemove += $svc }
}

# Catch any service with VMware in its display name or binary path
$allCimServices = Get-CimInstance Win32_Service -ErrorAction SilentlyContinue
$vmwareCim = $allCimServices | Where-Object {
    $_.DisplayName -match 'VMware' -or
    $_.Name -match 'vmware|vmtools|vgauth' -or
    $_.PathName -match 'VMware'
}
if ($vmwareCim) {
    foreach ($cim in $vmwareCim) {
        $svc = Get-Service -Name $cim.Name -ErrorAction SilentlyContinue
        if ($svc) { $servicesToRemove += $svc }
    }
}

# pvscsi would otherwise get swept in by the broad "VMware in
# DisplayName" match above - excluded here too so it's never touched
# regardless of which path found it (see $VMwareServices comment).
$servicesToRemove = $servicesToRemove | Where-Object { $_.Name -notin @('pvscsi') }

$servicesToRemove = $servicesToRemove | Sort-Object -Property Name -Unique

if ($servicesToRemove.Count -eq 0) {
    Log '[INFO] No VMware-related services found.'
} else {
    Log "[INFO] Found $($servicesToRemove.Count) VMware-related service(s)"
    foreach ($svc in $servicesToRemove) {
        Log "[SERVICE] Stopping/disabling $($svc.Name) ($($svc.DisplayName)) - Status: $($svc.Status)"

        & sc.exe stop "$($svc.Name)" 2>&1 | Out-Null
        Start-Sleep -Seconds 2

        $cimSvc = Get-CimInstance Win32_Service -Filter "Name='$($svc.Name)'" -ErrorAction SilentlyContinue
        if ($cimSvc -and $cimSvc.ProcessId -ne 0) {
            Log "  Force-killing PID $($cimSvc.ProcessId)"
            Stop-Process -Id $cimSvc.ProcessId -Force -ErrorAction SilentlyContinue
        }

        & sc.exe config "$($svc.Name)" start= disabled 2>&1 | Out-Null
    }
}

# -------------------------------------------------------------------
# PHASE 2: Remove VMware driver packages (pnputil)
# -------------------------------------------------------------------
Log ''
Log '  PHASE 2: VMware Driver Packages'
Log '---------------------------------------------------------------'

# Some packages are republished as "Broadcom Inc." post-acquisition, but
# keep the same .inf name - match known VMware .inf basenames instead of
# the ambiguous "Broadcom" text. vm3d.inf (binary vm3dmp.sys) added explicitly.
$VMwareExtraInfNames = @('vm3d.inf')
$VMwareInfNames = ((($VMwareDriverFiles | ForEach-Object { $_ -replace '\.sys$', '.inf' }) + $VMwareExtraInfNames) |
    ForEach-Object { [regex]::Escape($_) }) -join '|'

# Get-WindowsDriver returns structured fields, so it works regardless of
# display language. pnputil's text output is English-only, so it's only
# a fallback below.
# pvscsi excluded here too - a generic "ProviderName -match VMware"
# match would otherwise still catch its driver package even though the
# service/file are deliberately left alone (see $VMwareServices comment
# in the data declarations above).
$publishedNames = $null
try {
    $publishedNames = @(
        Get-WindowsDriver -Online -ErrorAction Stop |
            Where-Object {
                $_.OriginalFileName -notmatch '(?i)^pvscsi\.inf$' -and
                (
                    $_.ProviderName -match 'VMware' -or
                    $_.OriginalFileName -match "(?i)($VMwareInfNames)"
                )
            } |
            ForEach-Object { $_.Driver }
    )
    Log "[INFO] Get-WindowsDriver identified $($publishedNames.Count) VMware driver package(s)"
} catch {
    Log "[WARNING] Get-WindowsDriver unavailable ($_) - falling back to pnputil text parsing (English output only)"
}

$drvRemoved = 0
if ($null -ne $publishedNames) {
    foreach ($inf in $publishedNames) {
        Log "[DRIVER] Removing $inf ..."
        $result = & $PnpUtil /delete-driver "$inf" /uninstall /force 2>&1 | Out-String
        Log "  $result"
        $drvRemoved++
    }
} else {
    $pnpOutput = & $PnpUtil /enum-drivers 2>&1 | Out-String
    $driverBlocks = $pnpOutput -split '(?=Published Name)' |
        Where-Object {
            $_ -notmatch '(?i)Original Name:\s*pvscsi\.inf\b' -and
            (
                $_ -match 'VMware' -or
                $_ -match "(?i)Original Name:\s*($VMwareInfNames)\b"
            )
        }

    if ($driverBlocks.Count -eq 0) {
        Log '[INFO] No VMware driver packages found in driver store.'
    } else {
        Log "[INFO] Found $($driverBlocks.Count) VMware driver package(s)"
        foreach ($block in $driverBlocks) {
            $inf = if ($block -match 'Published Name\s*:\s*(\S+)') { $Matches[1] } else { '(unknown)' }
            Log "[DRIVER] Removing $inf ..."
            $result = & $PnpUtil /delete-driver "$inf" /uninstall /force 2>&1 | Out-String
            Log "  $result"
            $drvRemoved++
        }
    }
}

# -------------------------------------------------------------------
# PHASE 3: Disable and remove VMware PnP devices
# -------------------------------------------------------------------
Log ''
Log '  PHASE 3: VMware PnP Devices'
Log '---------------------------------------------------------------'

# vmci can re-assert its device if left running, so delete it outright
# now and drop it from the Phase 4 list.
& sc.exe stop vmci 2>&1 | Out-Null
& sc.exe delete vmci 2>&1 | Out-Null
$servicesToRemove = @($servicesToRemove | Where-Object { $_.Name -ne 'vmci' })

# vm3dc003.dll is a class coinstaller that can veto removal of any
# display device; strip only this entry from the shared REG_MULTI_SZ value.
$displayClassGuid = '{4d36e968-e325-11ce-bfc1-08002be10318}'
$coDevKey = 'HKLM:\SYSTEM\CurrentControlSet\Control\CoDeviceInstallers'
$coDevProp = Get-ItemProperty -Path $coDevKey -Name $displayClassGuid -ErrorAction SilentlyContinue
if ($coDevProp -and $coDevProp.$displayClassGuid) {
    $filtered = @($coDevProp.$displayClassGuid | Where-Object { $_ -notmatch 'vm3dc003' })
    if ($filtered.Count -ne @($coDevProp.$displayClassGuid).Count) {
        Set-ItemProperty -Path $coDevKey -Name $displayClassGuid -Value $filtered -Type MultiString
        Log '[INFO] Cleared VMware SVGA class coinstaller registration (vm3dc003)'
    }
}

$vmDevices = Get-PnpDevice -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Manufacturer -match 'VMware' -or
        $_.FriendlyName -match 'VMware' -or
        $_.InstanceId -match 'VEN_15AD' -or
        # VMware's USB vendor ID - catches the "USB Composite Device"
        # parent of the VMware USB mouse, which doesn't say VMware itself.
        $_.InstanceId -match 'VID_0E0F' -or
        # ROOT-enumerated software devices (e.g. ROOT\VMWVMCIHOSTDEV\0000)
        # that can lack a VMware Manufacturer/FriendlyName entirely.
        $_.InstanceId -match '^ROOT\\(VMWVMCI|VMware)'
    }

# Builds an InstanceId -> DeviceClasses key lookup in one .NET pass,
# avoiding a reg.exe scan per ghost device (would mean thousands of spawns).
# DeviceClasses entries often carry restrictive ACLs, so every read below
# goes through the Get-Safe*/Open-Safe* helpers above.
function Get-DeviceClassesInstanceMap {
    $map = @{}
    $hklm64 = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine', 'Registry64')
    $root = $hklm64.OpenSubKey('SYSTEM\CurrentControlSet\Control\DeviceClasses')
    if (-not $root) { return $map }
    foreach ($ifaceGuidName in (Get-SafeSubKeyNames $root)) {
        $ifaceGuidKey = Open-SafeSubKey $root $ifaceGuidName
        if (-not $ifaceGuidKey) { continue }
        foreach ($instName in (Get-SafeSubKeyNames $ifaceGuidKey)) {
            $instKey = Open-SafeSubKey $ifaceGuidKey $instName
            if ($instKey) {
                $devInst = Get-SafeRegValue $instKey 'DeviceInstance'
                if ($devInst) {
                    if (-not $map.ContainsKey($devInst)) {
                        $map[$devInst] = New-Object System.Collections.Generic.List[string]
                    }
                    $map[$devInst].Add("HKLM\SYSTEM\CurrentControlSet\Control\DeviceClasses\$ifaceGuidName\$instName")
                }
                $instKey.Close()
            }
        }
        $ifaceGuidKey.Close()
    }
    $root.Close()
    return $map
}

# Cleans a ghost device's class driver binding and DeviceClasses
# registration - leftovers here can make Windows resynthesize a fresh
# ghost Enum entry. DeviceContainers is only reported, not deleted
# (may be shared). Must run BEFORE the Enum key delete below.
function Clear-GhostDeviceReferences {
    param([string]$InstanceId)

    # The Enum key's "Driver" value ("{ClassGUID}\NNNN") points to its
    # class driver binding key - the only way to find it, since that key
    # has no value pointing back.
    $enumKey = "HKLM\SYSTEM\CurrentControlSet\Enum\$InstanceId"
    $driverBinding = Get-RegValue $enumKey 'Driver'
    if ($driverBinding) {
        $bindingKey = "HKLM\SYSTEM\CurrentControlSet\Control\Class\$driverBinding"
        Delete-RegKey $bindingKey
        Log "[INFO] Cleared class driver binding for $InstanceId ($bindingKey)"
    }

    # Built once, lazily, and reused across calls - see
    # Get-DeviceClassesInstanceMap above.
    if (-not $script:deviceClassesMap) {
        $script:deviceClassesMap = Get-DeviceClassesInstanceMap
    }
    if ($script:deviceClassesMap.ContainsKey($InstanceId)) {
        foreach ($ifaceInstanceKey in $script:deviceClassesMap[$InstanceId]) {
            Delete-RegKey $ifaceInstanceKey
            Log "[INFO] Cleared DeviceClasses interface registration for $InstanceId ($ifaceInstanceKey)"
        }
    }

    $containerHits = Get-RegDataMatches 'HKLM\SYSTEM\CurrentControlSet\Control\DeviceContainers' $InstanceId
    foreach ($hit in $containerHits) {
        Log "[WARNING] DeviceContainers still references $InstanceId (not removed - may be shared with another device): $($hit.KeyPath)\$($hit.ValueName)"
    }
}

if (-not $vmDevices -or $vmDevices.Count -eq 0) {
    Log '[INFO] No VMware PnP devices found.'
} else {
    Log "[INFO] Found $($vmDevices.Count) VMware PnP device(s)"

    $pnpHelp = & $PnpUtil /? 2>&1 | Out-String
    $hasRemoveDevice = $pnpHelp -match '/remove-device'

    foreach ($dev in $vmDevices) {
        Log "[DEVICE] $($dev.FriendlyName) [$($dev.InstanceId)] Status=$($dev.Status)"

        if ($dev.Status -ne 'Unknown') {
            try {
                Disable-PnpDevice -InstanceId $dev.InstanceId -Confirm:$false -ErrorAction Stop
                Log "[DISABLED] $($dev.FriendlyName)"
            } catch {
                Log "[WARNING] Could not disable: $($dev.FriendlyName) - $_"
            }
        }

        $removed = $false
        if ($hasRemoveDevice) {
            & $PnpUtil /remove-device "$($dev.InstanceId)" 2>&1 | Out-Null
            # 3010 = ERROR_SUCCESS_REBOOT_REQUIRED - removed, not a failure.
            if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 3010) {
                Log "[REMOVED] $($dev.FriendlyName)"
                $removed = $true
            }
        }

        # Ghost devices often survive the above; try CM_Uninstall_DevNode,
        # then raw registry delete as last resort.
        if (-not $removed -and $dev.Status -eq 'Unknown') {
            if (Remove-GhostDevNode -InstanceId $dev.InstanceId) {
                Log "[REMOVED] $($dev.FriendlyName) (CM_Uninstall_DevNode)"
                $removed = $true
            }
        }

        if (-not $removed -and $dev.Status -eq 'Unknown') {
            Clear-GhostDeviceReferences -InstanceId $dev.InstanceId
            $enumKey = "HKLM\SYSTEM\CurrentControlSet\Enum\$($dev.InstanceId)"
            & $RegExe delete $enumKey /f 2>&1 | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Log "[DEL ENUM] $($dev.FriendlyName) (registry fallback)"
            }
        }
    }
}

# -------------------------------------------------------------------
# PHASE 4: Delete VMware service definitions (devices are gone now)
# -------------------------------------------------------------------
Log ''
Log '  PHASE 4: Delete VMware Services'
Log '---------------------------------------------------------------'

if ($servicesToRemove.Count -eq 0) {
    Log '[INFO] No VMware-related services to delete.'
} else {
    foreach ($svc in $servicesToRemove) {
        $scResult = & sc.exe delete "$($svc.Name)" 2>&1 | Out-String
        if ($LASTEXITCODE -eq 0) {
            Log "[SUCCESS] Deleted service: $($svc.Name)"
            $svcDeleted++
        } else {
            Log "[WARNING] sc.exe delete failed for $($svc.Name): $scResult"
        }
    }
}

# -------------------------------------------------------------------
# PHASE 5: Remove VMware registry entries
# -------------------------------------------------------------------
Log ''
Log '  PHASE 5: VMware Registry Entries'
Log '---------------------------------------------------------------'

# Auto-generate service registry keys from $VMwareServices
$serviceRegKeys = $VMwareServices | ForEach-Object {
    "HKLM\SYSTEM\CurrentControlSet\Services\$_"
}
$allRegKeys = $VMwareRegistryKeys + $serviceRegKeys

foreach ($key in $allRegKeys) {
    Delete-RegKey $key
}

Delete-RegValue 'HKLM\SOFTWARE\RegisteredApplications' 'VMware Host Open'

# --- Stray COM registrations (CLSID/TypeLib) ---
# GUIDs vary per Tools build, so scan in-process via .NET rather than
# spawning reg.exe per GUID across tens of thousands of subkeys.
# Ownership is decided by the COM server module path/name (InprocServer32/
# LocalServer32/win32/win64's default value), not by "VMware" appearing
# anywhere in the subtree - a free-text match would also delete an
# unrelated component that merely references a VMware path/DLL in some
# other value, and this deletion isn't reversible on the migrated guest.
$VMwareComServerPattern = '(?i)(\\Program Files( \(x86\))?\\(Common Files\\)?VMware\\|\\vm3d|\\vmGuestLib|\\vmhgfs|VMWSU\.DLL)'

# CLSID's server path is one level down (InprocServer32/LocalServer32);
# TypeLib's is three levels down (version\lcid\win32|win64) - depth 3
# covers both while still only matching those specific named subkeys.
function Test-KeyTreeHasVMwareComServer {
    param([Microsoft.Win32.RegistryKey]$Key, [int]$Depth = 3)
    if (-not $Key -or $Depth -le 0) { return $false }
    foreach ($childName in (Get-SafeSubKeyNames $Key)) {
        $child = Open-SafeSubKey $Key $childName
        if (-not $child) { continue }
        if ($childName -in @('InprocServer32', 'LocalServer32', 'win32', 'win64')) {
            $modulePath = Get-SafeRegValue $child ''
            if ($modulePath -and "$modulePath" -match $VMwareComServerPattern) {
                $child.Close()
                return $true
            }
        }
        if (Test-KeyTreeHasVMwareComServer $child ($Depth - 1)) {
            $child.Close()
            return $true
        }
        $child.Close()
    }
    return $false
}

function Get-VMwareStrayComKey {
    param([string]$SubPath, [string]$RootPathForLog)
    $hklm64 = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine', 'Registry64')
    $root = $hklm64.OpenSubKey($SubPath)
    if (-not $root) { return @() }
    $stray = @()
    foreach ($guidName in (Get-SafeSubKeyNames $root)) {
        $guidKey = Open-SafeSubKey $root $guidName
        if (-not $guidKey) { continue }
        if (Test-KeyTreeHasVMwareComServer $guidKey) { $stray += "$RootPathForLog\$guidName" }
        $guidKey.Close()
    }
    $root.Close()
    return $stray
}

$comRoots = @(
    @{ SubPath = 'SOFTWARE\Classes\CLSID';               LogPath = 'HKLM\SOFTWARE\Classes\CLSID' },
    @{ SubPath = 'SOFTWARE\Classes\WOW6432Node\CLSID';   LogPath = 'HKLM\SOFTWARE\Classes\WOW6432Node\CLSID' },
    @{ SubPath = 'SOFTWARE\Classes\TypeLib';             LogPath = 'HKLM\SOFTWARE\Classes\TypeLib' },
    @{ SubPath = 'SOFTWARE\Classes\WOW6432Node\TypeLib'; LogPath = 'HKLM\SOFTWARE\Classes\WOW6432Node\TypeLib' }
)
$comRemoved = 0
foreach ($root in $comRoots) {
    foreach ($strayKey in (Get-VMwareStrayComKey $root.SubPath $root.LogPath)) {
        Delete-RegKey $strayKey
        $comRemoved++
    }
}
Log "[INFO] Stray VMware COM (CLSID/TypeLib) registrations removed: $comRemoved"

# --- Installer\Folders stale path references ---
# Value NAME is the folder path an MSI wrote to; not reliably pruned by
# msiexec /x. WOW64-shared, no separate WOW6432Node copy.
$foldersKeyPS = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\Folders'
$foldersProps = Get-ItemProperty -Path $foldersKeyPS -ErrorAction SilentlyContinue
if ($foldersProps) {
    $vmwareFolderNames = $foldersProps.PSObject.Properties |
        Where-Object { $_.Name -notmatch '^PS(Path|ParentPath|ChildName|Drive|Provider)$' -and $_.Name -match 'VMware' } |
        ForEach-Object { $_.Name }
    foreach ($n in $vmwareFolderNames) {
        Remove-ItemProperty -Path $foldersKeyPS -Name $n -Force -ErrorAction SilentlyContinue
        Log "[DEL VAL] Installer\Folders\$n"
        $regDeleted++
    }
}

# --- Run/RunOnce autostart entries ---
# e.g. "VMware VM3DService Process" - a harmless leftover once the
# target .exe is gone.
function Remove-VMwareRunEntries {
    param([string]$KeyPath)
    $psPath = $KeyPath -replace '^HKLM\\', 'HKLM:\'
    $props = Get-ItemProperty -Path $psPath -ErrorAction SilentlyContinue
    if (-not $props) { return }
    $names = $props.PSObject.Properties |
        Where-Object { $_.Name -notmatch '^PS(Path|ParentPath|ChildName|Drive|Provider)$' } |
        Where-Object { $_.Name -match 'VMware' -or "$($_.Value)" -match 'VMware' } |
        ForEach-Object { $_.Name }
    foreach ($n in $names) {
        Remove-ItemProperty -Path $psPath -Name $n -Force -ErrorAction SilentlyContinue
        Log "[DEL VAL] $KeyPath\$n"
        $script:regDeleted++
    }
}
foreach ($runKey in @(
    'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Run',
    'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce',
    'HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run',
    'HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce'
)) { Remove-VMwareRunEntries $runKey }

# --- SVGA 3D display adapter "software device" node ---
# Control\Video\{GUID}\Video holds a separate registration from the
# vm3dmp_loader service key already deleted above.
$videoRoot = 'HKLM\SYSTEM\CurrentControlSet\Control\Video'
foreach ($guidKey in (Get-RegSubKeyPaths $videoRoot)) {
    $desc = Get-RegValue "$guidKey\Video" 'DeviceDesc'
    $svc  = Get-RegValue "$guidKey\Video" 'Service'
    if ($desc -match 'VMware' -or $svc -match '^vm3dmp') {
        Delete-RegKey $guidKey
    }
}

# -------------------------------------------------------------------
# PHASE 6: Remove VMware files and folders
# -------------------------------------------------------------------
Log ''
Log '  PHASE 6: VMware Files and Folders'
Log '---------------------------------------------------------------'

$fileDeleted = 0
$fileErrors  = 0

foreach ($dir in $VMwareDirs) {
    Remove-PathItem $dir -Recurse
}

foreach ($pattern in $VMwareTempDirPatterns) {
    $parent = Split-Path $pattern -Parent
    $leaf   = Split-Path $pattern -Leaf
    if (Test-Path $parent) {
        Get-ChildItem -Path $parent -Directory -Filter $leaf -ErrorAction SilentlyContinue |
            ForEach-Object { Remove-PathItem $_.FullName -Recurse }
    }
}

# Per-user AppData folders - path depends on each profile under \Users.
$usersRoot = Join-Path $env:SystemDrive 'Users'
if (Test-Path $usersRoot) {
    # -Force: C:\Users\Default is hidden and would otherwise be skipped.
    Get-ChildItem -Path $usersRoot -Directory -Force -ErrorAction SilentlyContinue |
        ForEach-Object {
            foreach ($sub in @('AppData\Local\VMware', 'AppData\Roaming\VMware', 'AppData\LocalLow\VMware')) {
                Remove-PathItem (Join-Path $_.FullName $sub) -Recurse
            }
        }
}

$SysWOW64 = Join-Path $env:SystemRoot 'SysWOW64'
$allFiles = ($VMwareDriverFiles  | ForEach-Object { Join-Path $Drivers $_ }) +
            ($VMwareSystemFiles  | ForEach-Object { Join-Path $Sys32 $_ }) +
            ($VMwareSysWOW64Files | ForEach-Object { Join-Path $SysWOW64 $_ })

foreach ($f in $allFiles) {
    Remove-PathItem $f
}

# DriverStore leftovers - report only. Force-deleting risks CBS
# bookkeeping inconsistency (DISM /CheckHealth corruption); log for
# manual pnputil follow-up instead.
$driverStoreResidual = 0
$driverStore = Join-Path $Sys32 'DriverStore\FileRepository'
if (Test-Path $driverStore) {
    Get-ChildItem -Path $driverStore -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match $DriverStorePattern } |
        ForEach-Object {
            Log "[WARNING] Residual DriverStore folder (not removed - use pnputil /delete-driver or investigate manually): $($_.FullName)"
            $driverStoreResidual++
        }
}

# -------------------------------------------------------------------
# PHASE 7: Remove VMware scheduled tasks
# -------------------------------------------------------------------
Log ''
Log '  PHASE 7: VMware Scheduled Tasks'
Log '---------------------------------------------------------------'

$vmTasks = Get-ScheduledTask -ErrorAction SilentlyContinue |
    Where-Object { $_.TaskName -match 'VMware|vmtools' -or $_.TaskPath -match 'VMware' }

if (-not $vmTasks -or $vmTasks.Count -eq 0) {
    Log '[INFO] No VMware scheduled tasks found.'
} else {
    foreach ($task in $vmTasks) {
        $taskFull = "$($task.TaskPath)$($task.TaskName)"
        try {
            Unregister-ScheduledTask -TaskName $task.TaskName -Confirm:$false -ErrorAction Stop
            Log "[DEL TASK] $taskFull"
        } catch {
            Log "[WARNING] Failed to remove task: $taskFull - $_"
        }
    }
}

# -------------------------------------------------------------------
# SUMMARY
# -------------------------------------------------------------------
Log ''
Log '==============================================================='
Log '  VMware Cleanup Complete'
Log '==============================================================='
Log "  MSI products uninstalled: $msiRemoved"
Log "  MSI Components removed:  $componentsRemoved"
Log "  Driver packages removed: $drvRemoved"
Log "  Services deleted:        $svcDeleted"
Log "  Stray COM keys removed:  $comRemoved"
Log "  Registry keys deleted:   $regDeleted"
Log "  Registry errors:         $regErrors"
Log "  Files/dirs removed:      $fileDeleted"
Log "  File removal errors:     $fileErrors"
Log "  DriverStore residuals (not removed, need follow-up): $driverStoreResidual"
Log "  Log: $LogFile"
Log '==============================================================='
