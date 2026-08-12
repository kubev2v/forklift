# 9100_cleanup_vmware.ps1
# Complete VMware cleanup: PnP devices, driver packages, services,
# registry entries, files/folders, and scheduled tasks.
# Run as Administrator.

$ErrorActionPreference = 'Continue'

# ===================================================================
# WOW64-safe path resolution
# ===================================================================
# The firstboot service may run as a 32-bit (WOW64) process where
# System32 is redirected to SysWOW64.  Sysnative escapes back to the
# real 64-bit System32.
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
    'vmhgfs', 'vmmemctl', 'vmrawdsk', 'vnetWFP', 'vsepflt',
    'vmci', 'vmxnet3', 'vmxnet3ndis6', 'pvscsi',
    'vmusbmouse', 'vmmouse',
    'vm3dmp', 'vm3dmp-debug', 'vm3dmp-stats', 'vm3dmp_loader'
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
    'vsock.sys', 'vmmemctl.sys', 'vsepflt.sys', 'vnetWFP.sys'
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

# Delete a registry key tree via reg.exe.
# Uses reg.exe query (not PowerShell Test-Path) to avoid WOW64 redirection.
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

# Remove a file or directory with logging and counter updates.
# For directories, falls back to cmd /c rd if Remove-Item leaves remnants.
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
    if (Test-Path $Path) {
        Log "[WARNING] Could not remove: $Path"
        $script:fileErrors++
    } else {
        Log "[DEL] $Path"
        $script:fileDeleted++
    }
}

# ===================================================================
# Main
# ===================================================================

Log '==============================================================='
Log '  VMware Full Cleanup Script'
Log '==============================================================='
Log "[INFO] Sys32=$Sys32  PnpUtil=$PnpUtil  RegExe=$RegExe"

# -------------------------------------------------------------------
# PHASE 1: Disable and remove VMware PnP devices
# -------------------------------------------------------------------
Log ''
Log '  PHASE 1: VMware PnP Devices'
Log '---------------------------------------------------------------'

$vmDevices = Get-PnpDevice -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Manufacturer -match 'VMware' -or
        $_.FriendlyName -match 'VMware' -or
        $_.InstanceId -match 'VEN_15AD'
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

        if ($hasRemoveDevice) {
            & $PnpUtil /remove-device "$($dev.InstanceId)" 2>&1 | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Log "[REMOVED] $($dev.FriendlyName)"
            }
        }
    }

    if (-not $hasRemoveDevice) {
        Log '[INFO] pnputil /remove-device not available - removing ghost devices via registry.'
        foreach ($dev in $vmDevices) {
            if ($dev.Status -eq 'Unknown') {
                $enumKey = "HKLM\SYSTEM\CurrentControlSet\Enum\$($dev.InstanceId)"
                & $RegExe delete $enumKey /f 2>&1 | Out-Null
                if ($LASTEXITCODE -eq 0) {
                    Log "[DEL ENUM] $($dev.FriendlyName)"
                }
            }
        }
    }
}

# -------------------------------------------------------------------
# PHASE 2: Remove VMware driver packages (pnputil)
# -------------------------------------------------------------------
Log ''
Log '  PHASE 2: VMware Driver Packages'
Log '---------------------------------------------------------------'

$pnpOutput = & $PnpUtil /enum-drivers 2>&1 | Out-String
$driverBlocks = $pnpOutput -split '(?=Published Name)' |
    Where-Object { $_ -match 'VMware|Broadcom.*vmx' }

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
# PHASE 3: Stop, disable, and delete VMware services
# -------------------------------------------------------------------
Log ''
Log '  PHASE 3: VMware Services'
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
        Log "[SERVICE] Processing $($svc.Name) ($($svc.DisplayName)) - Status: $($svc.Status)"

        & sc.exe stop "$($svc.Name)" 2>&1 | Out-Null
        Start-Sleep -Seconds 2

        $cimSvc = Get-CimInstance Win32_Service -Filter "Name='$($svc.Name)'" -ErrorAction SilentlyContinue
        if ($cimSvc -and $cimSvc.ProcessId -ne 0) {
            Log "  Force-killing PID $($cimSvc.ProcessId)"
            Stop-Process -Id $cimSvc.ProcessId -Force -ErrorAction SilentlyContinue
        }

        & sc.exe config "$($svc.Name)" start= disabled 2>&1 | Out-Null
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
# PHASE 4: Remove VMware registry entries
# -------------------------------------------------------------------
Log ''
Log '  PHASE 4: VMware Registry Entries'
Log '---------------------------------------------------------------'

$regDeleted = 0
$regErrors  = 0

# Auto-generate service registry keys from $VMwareServices
$serviceRegKeys = $VMwareServices | ForEach-Object {
    "HKLM\SYSTEM\CurrentControlSet\Services\$_"
}
$allRegKeys = $VMwareRegistryKeys + $serviceRegKeys

foreach ($key in $allRegKeys) {
    Delete-RegKey $key
}

Delete-RegValue 'HKLM\SOFTWARE\RegisteredApplications' 'VMware Host Open'

# -------------------------------------------------------------------
# PHASE 5: Remove VMware files and folders
# -------------------------------------------------------------------
Log ''
Log '  PHASE 5: VMware Files and Folders'
Log '---------------------------------------------------------------'

$fileDeleted = 0
$fileErrors  = 0

foreach ($dir in $VMwareDirs) {
    Remove-PathItem $dir -Recurse
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
# PHASE 6: Remove VMware scheduled tasks
# -------------------------------------------------------------------
Log ''
Log '  PHASE 6: VMware Scheduled Tasks'
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
Log "  Driver packages removed: $drvRemoved"
Log "  Services deleted:        $svcDeleted"
Log "  Registry keys deleted:   $regDeleted"
Log "  Registry errors:         $regErrors"
Log "  Files/dirs removed:      $fileDeleted"
Log "  File removal errors:     $fileErrors"
Log "  Log: $LogFile"
Log '==============================================================='
