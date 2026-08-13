# 9100_cleanup_vmware.ps1
# Complete VMware cleanup: PnP devices, driver packages, services,
# registry entries, files/folders, and scheduled tasks.
# Run as Administrator.

$ErrorActionPreference = 'Continue'

# Schedule-for-reboot helper via MoveFileEx(MOVEFILE_DELAY_UNTIL_REBOOT).
# Used as last resort when a file is locked (e.g. loaded driver).
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class LockedFileDeleter {
    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    public static extern bool MoveFileEx(
        string lpExistingFileName, string lpNewFileName, int dwFlags);
    public const int MOVEFILE_DELAY_UNTIL_REBOOT = 0x4;
}

// CM_Uninstall_DevNode: same API Device Manager's "Uninstall device"
// uses to remove a ghost/non-present device. Safer than deleting the
// CurrentControlSet\Enum key by hand, which can corrupt the PnP database.
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
# Use Sysnative (not System32) in case this runs as a 32-bit process,
# where System32 would otherwise be redirected to SysWOW64.
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
# Data declarations — single source of truth
# ===================================================================

$VMwareServices = @(
    'VGAuthService', 'VM3DService', 'VMTools', 'vmvss', 'GISvc',
    'vmhgfs', 'vmmemctl', 'vmrawdsk', 'vnetWFP', 'vnetflt', 'vsepflt',
    'vmci', 'vmxnet3', 'vmxnet3ndis6', 'pvscsi',
    'vmusbmouse', 'vmmouse',
    'vm3dmp', 'vm3dmp-debug', 'vm3dmp-stats', 'vm3dmp_loader',
    # vgauth/cblauncher are alternate/legacy names alongside
    # VGAuthService/CbLauncher - a no-op if not present.
    'vsock', 'efifw', 'svga_wddm', 'vmaudio', 'vgauth', 'cblauncher',
    'vmwtimeprovider', 'vmstatsprovider', 'vmupgradehelper',
    'vmwefifw'
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
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vmci'
)

$VMwareDirs = @(
    "$env:ProgramFiles\VMware",
    "$env:ProgramFiles\Common Files\VMware",
    "${env:ProgramFiles(x86)}\VMware",
    "${env:ProgramFiles(x86)}\Common Files\VMware",
    "$env:ProgramData\VMware",
    "$env:SystemRoot\Temp\vmware-SYSTEM"
)

$VMwareDriverFiles = @(
    'vmci.sys', 'vmmouse.sys', 'vmrawdsk.sys', 'vmhgfs.sys',
    'vmusbmouse.sys', 'pvscsi.sys', 'vmxnet3.sys',
    'vm3dmp.sys', 'vm3dmp-debug.sys', 'vm3dmp-stats.sys', 'vm3dmp_loader.sys',
    'vsock.sys', 'vmmemctl.sys', 'vsepflt.sys', 'vnetWFP.sys',
    # vnetflt: pre-10.2.5 predecessor of vnetWFP for the Tools
    # "NetworkIntrospection" feature. A Tools installer bug can leave
    # it behind and BSOD alongside the newer vsepflt driver.
    'vnetflt.sys',
    'svga_wddm.sys', 'efifw.sys', 'vmaudio.sys'
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

# DriverStore dirs matching this pattern are VMware; Hyper-V dirs
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

# Delete a registry key tree. Uses reg.exe (not PowerShell) to avoid
# WOW64 redirection.
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

# Remove a file or directory with logging and counter updates.
# Falls back to cmd /c rd/del, then schedules locked files for
# deletion on reboot via MoveFileEx(MOVEFILE_DELAY_UNTIL_REBOOT).
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
    # Resolve Sysnative back to System32 so the kernel can find the file.
    if (-not $Recurse) {
        $kernelPath = $Path -replace '(?i)\\Sysnative\\', '\System32\'
        # Must pass [NullString]::Value, not $null - PowerShell marshals a
        # bare $null as "", which MoveFileEx treats as "rename to empty
        # path" (fails) instead of "delete on reboot".
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
# VMware Tools is an MSI product. Manually deleting its files/services/
# drivers (Phases 1-6) never touches Windows Installer's own bookkeeping,
# so it would keep showing up in Programs and Features forever. Uninstall
# it properly first, then force-delete the registry remnants if msiexec
# can't fully clean up (e.g. the payload was already removed by a
# previous partial run).
Log ''
Log '  PHASE 0: VMware Tools MSI Uninstall'
Log '---------------------------------------------------------------'

# Declared here since Phase 0 is the first phase to call Delete-RegKey/Value.
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
                $p = Start-Process -FilePath 'msiexec.exe' `
                    -ArgumentList "/x $productCode /qn /norestart" `
                    -Wait -PassThru -ErrorAction Stop
                if ($p.ExitCode -eq 0) {
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

        # Guarantee the Add/Remove Programs entry is gone even if msiexec
        # left it behind (e.g. payload already missing).
        Delete-RegKey $entry.KeyPath
    }
}

# Clean orphaned Windows Installer product registrations - these drive
# Get-Package/Win32_Product visibility independently of the Uninstall key
# above and can survive even a clean msiexec /x. Enumerate every SID
# under UserData rather than hardcoding S-1-5-18 (SYSTEM).
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

# Clean orphaned Windows Installer "Components" ownership entries. Every
# file/registry-key a product installs is tracked under
# UserData\<SID>\Components\<ComponentGuid>, keyed by the owning
# product's packed ProductCode (value data = install path). A clean
# msiexec /x removes these automatically; the registry fallback above
# doesn't. A component can be shared by multiple products, so delete
# only the matching value, then remove the key once it has no owners left.
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
# PHASE 1: Stop and disable VMware services (quiesce before touching
# drivers/devices below)
# -------------------------------------------------------------------
# Phase order matters: quiesce services first, remove driver packages
# before PnP device instances, and only delete service definitions last
# (Phase 4, after devices are gone). Otherwise: vmci (a PnP bus
# enumerator) can keep re-asserting its bus device during removal, and
# drivers still in the driver store can get re-selected on a PnP rescan.
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
# PHASE 2: Remove VMware driver packages (pnputil) - before PnP
# device instances, see Phase 1 rationale above.
# -------------------------------------------------------------------
Log ''
Log '  PHASE 2: VMware Driver Packages'
Log '---------------------------------------------------------------'

# Post-Broadcom-acquisition, VMware driver packages are republished with
# "Provider Name: Broadcom Inc." instead of "VMware, Inc.", though the
# .inf filename is unchanged. Matching "Broadcom" alone would be unsafe
# (unrelated Broadcom NICs/RAID controllers exist), so anchor on known
# VMware .inf basenames derived from $VMwareDriverFiles instead.
# vm3d.inf (SVGA 3D setup package) has no matching .sys of its own
# (the binary is vm3dmp.sys, already in that list), so add it explicitly.
$VMwareExtraInfNames = @('vm3d.inf')
$VMwareInfNames = ((($VMwareDriverFiles | ForEach-Object { $_ -replace '\.sys$', '.inf' }) + $VMwareExtraInfNames) |
    ForEach-Object { [regex]::Escape($_) }) -join '|'

$pnpOutput = & $PnpUtil /enum-drivers 2>&1 | Out-String
$driverBlocks = $pnpOutput -split '(?=Published Name)' |
    Where-Object {
        $_ -match 'VMware' -or
        $_ -match "(?i)Original Name:\s*($VMwareInfNames)\b"
    }

$drvRemoved = 0
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

# -------------------------------------------------------------------
# PHASE 3: Disable and remove VMware PnP devices
# -------------------------------------------------------------------
Log ''
Log '  PHASE 3: VMware PnP Devices'
Log '---------------------------------------------------------------'

# vmci is a PnP bus enumerator and can keep re-asserting its bus device
# during removal below - Phase 1 only stopped/disabled it, so delete it
# outright here, immediately before device removal. Remove it from the
# pending Phase 4 list so that phase doesn't warn it's already gone.
& sc.exe stop vmci 2>&1 | Out-Null
& sc.exe delete vmci 2>&1 | Out-Null
$servicesToRemove = @($servicesToRemove | Where-Object { $_.Name -ne 'vmci' })

# vm3d.inf registers a class coinstaller (vm3dc003.dll) under the
# Display-adapters class GUID. A class coinstaller runs for every
# device in that class and can veto removal for any of them, so strip
# only the matching entry - never the whole REG_MULTI_SZ value, since
# other legitimate coinstallers may share it.
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
        # VMware's USB vendor ID - catches the generic "USB Composite
        # Device"/"USB Input Device" parent of the VMware USB mouse,
        # which never says VMware in its own description.
        $_.InstanceId -match 'VID_0E0F'
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
            if ($LASTEXITCODE -eq 0) {
                Log "[REMOVED] $($dev.FriendlyName)"
                $removed = $true
            }
        }

        # Ghost devices (Status=Unknown) commonly survive both
        # Disable-PnpDevice and pnputil /remove-device (which also
        # doesn't exist pre-Windows 10 2004/Server 2022). Try
        # CM_Uninstall_DevNode next; fall back to a raw registry
        # delete only as a last resort.
        if (-not $removed -and $dev.Status -eq 'Unknown') {
            if (Remove-GhostDevNode -InstanceId $dev.InstanceId) {
                Log "[REMOVED] $($dev.FriendlyName) (CM_Uninstall_DevNode)"
                $removed = $true
            }
        }

        if (-not $removed -and $dev.Status -eq 'Unknown') {
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
# VMware Tools registers several COM components under CLSID/TypeLib
# GUIDs that vary across Tools builds, so hardcoding them doesn't scale.
# CLSID/TypeLib can each have tens of thousands of subkeys, so this uses
# the .NET registry API directly (64-bit view) instead of spawning
# reg.exe per GUID or recursively text-dumping the whole subtree
# (both far too slow - minutes vs. seconds).
function Test-RegValueContainsVMware {
    param([Microsoft.Win32.RegistryKey]$Key)
    if (-not $Key) { return $false }
    foreach ($valName in $Key.GetValueNames()) {
        $v = $Key.GetValue($valName)
        if ($v -and "$v" -match 'VMware') { return $true }
    }
    return $false
}

# Bounded-depth recursive scan. CLSID's path sits one level down
# (InprocServer32/LocalServer32), TypeLib's two levels down
# (e.g. {GUID}\1.0\0\win64) - depth 3 covers both cheaply.
function Test-KeyTreeContainsVMware {
    param([Microsoft.Win32.RegistryKey]$Key, [int]$Depth = 3)
    if (-not $Key) { return $false }
    if (Test-RegValueContainsVMware $Key) { return $true }
    if ($Depth -le 0) { return $false }
    foreach ($childName in $Key.GetSubKeyNames()) {
        $child = $Key.OpenSubKey($childName)
        if ($child) {
            $found = Test-KeyTreeContainsVMware $child ($Depth - 1)
            $child.Close()
            if ($found) { return $true }
        }
    }
    return $false
}

function Get-VMwareStrayComKeys {
    param([string]$SubPath, [string]$RootPathForLog)
    $hklm64 = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine', 'Registry64')
    $root = $hklm64.OpenSubKey($SubPath)
    if (-not $root) { return @() }
    $stray = @()
    foreach ($guidName in $root.GetSubKeyNames()) {
        $guidKey = $root.OpenSubKey($guidName)
        if (-not $guidKey) { continue }
        if (Test-KeyTreeContainsVMware $guidKey) { $stray += "$RootPathForLog\$guidName" }
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
    foreach ($strayKey in (Get-VMwareStrayComKeys $root.SubPath $root.LogPath)) {
        Delete-RegKey $strayKey
        $comRemoved++
    }
}
Log "[INFO] Stray VMware COM (CLSID/TypeLib) registrations removed: $comRemoved"

# --- Installer\Folders stale path references ---
# Windows Installer records every folder a per-machine MSI product wrote
# to under Installer\Folders (the folder path is the value NAME, e.g.
# "C:\ProgramData\VMware\VMware VGAuth\msgCatalog\"). Not reliably
# pruned by msiexec /x, so sweep it directly. This key is WOW64-shared,
# so there's no separate WOW6432Node copy to check.
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
# e.g. "VMware VM3DService Process" under Run - a harmless but visible
# leftover once the target .exe is gone.
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
# HKLM\SYSTEM\CurrentControlSet\Control\Video\{GUID}\Video holds a
# separate DeviceDesc/Service registration from the vm3dmp_loader
# service key already deleted above.
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

# Per-user VMware AppData folders - not covered by $VMwareDirs since
# the path depends on each profile under \Users.
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

# DriverStore leftovers
$driverStore = Join-Path $Sys32 'DriverStore\FileRepository'
if (Test-Path $driverStore) {
    Get-ChildItem -Path $driverStore -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match $DriverStorePattern } |
        ForEach-Object { Remove-PathItem $_.FullName -Recurse }
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
Log "  Log: $LogFile"
Log '==============================================================='
