# verify_vmware_cleanup.ps1
# Run on a Windows guest AFTER migration to verify all VMware artifacts are gone.
# Usage: powershell -ExecutionPolicy Bypass -File verify_vmware_cleanup.ps1

$ErrorActionPreference = 'SilentlyContinue'

# ===================================================================
# WOW64-safe path resolution (mirrors cleanup script)
# ===================================================================
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
# Data declarations — kept in sync with 9100_cleanup_vmware.ps1
# ===================================================================

$VMwareServices = @(
    'VGAuthService', 'VM3DService', 'VMTools', 'vmvss', 'GISvc',
    'vmhgfs', 'vmmemctl', 'vmrawdsk', 'vnetWFP', 'vsepflt',
    'vmci', 'vmxnet3', 'vmxnet3ndis6', 'pvscsi',
    'vmusbmouse', 'vmmouse',
    'vm3dmp', 'vm3dmp-debug', 'vm3dmp-stats', 'vm3dmp_loader'
)

# Registry keys checked via reg.exe (avoids WOW64 redirection on SOFTWARE hive).
$RegKeysToCheck = @(
    'HKLM\SOFTWARE\VMware, Inc.',
    'HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE',
    'HKLM\SOFTWARE\Classes\Applications\VMwareHostOpen.exe',
    'HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile',
    'HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL',
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
    "$env:ProgramData\VMware"
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

# Excludes Hyper-V services (vmbus, vmgid, vmgen*, etc.)
$SweepPattern = '^(vm(3d|ci|hgfs|mouse|rawdsk|memctl|xnet|ware|tools|vss|usb)|VGAuth|pvscsi|vsepflt|vnetWFP|GISvc)'

# ===================================================================
# Helpers
# ===================================================================

$fail = 0; $pass = 0; $warn = 0

function Check-Pass { param($msg) $script:pass++; Write-Host "  [PASS] $msg" -ForegroundColor Green }
function Check-Fail { param($msg) $script:fail++; Write-Host "  [FAIL] $msg" -ForegroundColor Red }
function Check-Warn { param($msg) $script:warn++; Write-Host "  [WARN] $msg" -ForegroundColor Yellow }

function Test-RegKey {
    param([string]$KeyPath)
    & $RegExe query $KeyPath /ve 2>&1 | Out-Null
    return ($LASTEXITCODE -eq 0)
}

# ===================================================================
# Checks
# ===================================================================

Write-Host ""
Write-Host "==============================================================="
Write-Host "  VMware Cleanup Verification"
Write-Host "  $(Get-Date)"
Write-Host "==============================================================="

# -------------------------------------------------------------------
# 1. Services
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- Services ---"

$foundServices = @()
foreach ($name in $VMwareServices) {
    $found = Get-Service -Name $name -ErrorAction SilentlyContinue
    if ($found) { $foundServices += $found }
}
$found = Get-Service | Where-Object {
    $_.DisplayName -match 'VMware' -or $_.Name -match 'vmware|vmtools|vgauth'
}
if ($found) { $foundServices += $found }
$foundServices = $foundServices | Sort-Object -Property Name -Unique

if ($foundServices.Count -eq 0) {
    Check-Pass "No VMware services found"
} else {
    foreach ($s in $foundServices) {
        Check-Fail "Service still present: $($s.Name) ($($s.DisplayName)) - State: $($s.Status)"
    }
}

# -------------------------------------------------------------------
# 2. Driver packages (pnputil)
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- Driver Packages ---"

$pnpOutput = & $PnpUtil /enum-drivers 2>&1 | Out-String
$driverBlocks = $pnpOutput -split '(?=Published Name)' |
    Where-Object { $_ -match 'VMware|Broadcom.*vmx' }

if ($driverBlocks.Count -eq 0) {
    Check-Pass "No VMware driver packages in driver store"
} else {
    foreach ($block in $driverBlocks) {
        $inf = if ($block -match 'Published Name\s*:\s*(\S+)') { $Matches[1] } else { '(unknown)' }
        Check-Fail "VMware driver package still in store: $inf"
    }
}

# -------------------------------------------------------------------
# 3. PnP devices
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- PnP Devices ---"

$vmDevices = Get-PnpDevice -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Manufacturer -match 'VMware' -or
        $_.FriendlyName -match 'VMware' -or
        $_.InstanceId -match 'VEN_15AD'
    }

if (-not $vmDevices -or $vmDevices.Count -eq 0) {
    Check-Pass "No VMware PnP devices found"
} else {
    foreach ($d in $vmDevices) {
        if ($d.Status -eq 'Unknown') {
            Check-Warn "Ghost VMware device: $($d.FriendlyName) [$($d.InstanceId)]"
        } else {
            Check-Fail "Active VMware device: $($d.FriendlyName) [$($d.InstanceId)] Status=$($d.Status)"
        }
    }
}

# -------------------------------------------------------------------
# 4. Registry
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- Registry ---"

# Non-service keys + auto-generated service keys
$serviceRegKeys = $VMwareServices | ForEach-Object {
    "HKLM\SYSTEM\CurrentControlSet\Services\$_"
}
$allRegKeys = $RegKeysToCheck + $serviceRegKeys

foreach ($key in $allRegKeys) {
    if (Test-RegKey $key) {
        Check-Fail "Registry key still exists: $key"
    } else {
        Check-Pass "Removed: $key"
    }
}

# RegisteredApplications value
$regApps = Get-ItemProperty 'HKLM:\SOFTWARE\RegisteredApplications' -ErrorAction SilentlyContinue
if ($regApps.'VMware Host Open') {
    Check-Fail "RegisteredApplications still has 'VMware Host Open'"
} else {
    Check-Pass "RegisteredApplications clean"
}

# Targeted sweep (excludes Hyper-V names)
Write-Host ""
Write-Host "--- Registry targeted sweep ---"
$strayKeys = Get-ChildItem -Path 'HKLM:\SYSTEM\CurrentControlSet\Services' -Depth 0 -ErrorAction SilentlyContinue |
    Where-Object { $_.PSChildName -match $SweepPattern }
if ($strayKeys.Count -eq 0) {
    Check-Pass "No stray VMware service registry keys"
} else {
    foreach ($k in $strayKeys) {
        Check-Warn "Stray VMware registry key: $($k.PSChildName)"
    }
}

# -------------------------------------------------------------------
# 5. Files on disk
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- Files ---"

foreach ($dir in $VMwareDirs) {
    if (Test-Path $dir) {
        Check-Fail "VMware directory still exists: $dir"
    } else {
        Check-Pass "Gone: $dir"
    }
}

$SysWOW64 = Join-Path $env:SystemRoot 'SysWOW64'
$allFiles = ($VMwareDriverFiles   | ForEach-Object { Join-Path $Drivers $_ }) +
            ($VMwareSystemFiles   | ForEach-Object { Join-Path $Sys32 $_ }) +
            ($VMwareSysWOW64Files | ForEach-Object { Join-Path $SysWOW64 $_ })

foreach ($f in $allFiles) {
    if (Test-Path $f) {
        Check-Fail "VMware file still exists: $f"
    } else {
        Check-Pass "Gone: $f"
    }
}

# -------------------------------------------------------------------
# 6. Scheduled tasks
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- Scheduled Tasks ---"

$vmTasks = Get-ScheduledTask -ErrorAction SilentlyContinue |
    Where-Object { $_.TaskName -match 'VMware|vmtools' -or $_.TaskPath -match 'VMware' }

if (-not $vmTasks -or $vmTasks.Count -eq 0) {
    Check-Pass "No VMware scheduled tasks"
} else {
    foreach ($t in $vmTasks) {
        Check-Fail "VMware scheduled task: $($t.TaskPath)$($t.TaskName)"
    }
}

# -------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------
Write-Host ""
Write-Host "==============================================================="
Write-Host "  RESULTS:  PASS=$pass  FAIL=$fail  WARN=$warn"
Write-Host "==============================================================="
if ($fail -eq 0 -and $warn -eq 0) {
    Write-Host "  All VMware artifacts removed successfully." -ForegroundColor Green
} elseif ($fail -eq 0) {
    Write-Host "  No hard failures, but $warn warnings to review." -ForegroundColor Yellow
} else {
    Write-Host "  $fail items still need cleanup." -ForegroundColor Red
}
Write-Host ""

exit $fail
