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
# Data declarations - kept in sync with 9100_cleanup_vmware.ps1
# ===================================================================

# pvscsi intentionally excluded here too - 9100_cleanup_vmware.ps1
# deliberately leaves it installed (see its $VMwareServices comment),
# so checking for its absence would be a false failure.
$VMwareServices = @(
    'VGAuthService', 'VM3DService', 'VMTools', 'vmvss', 'GISvc',
    'vmhgfs', 'vmmemctl', 'vmrawdsk', 'vnetWFP', 'vnetflt', 'vsepflt',
    'vmci', 'vmxnet3', 'vmxnet3ndis6',
    'vmusbmouse', 'vmmouse',
    'vm3dmp', 'vm3dmp-debug', 'vm3dmp-stats', 'vm3dmp_loader',
    # Additional short names - mirrors 9100_cleanup_vmware.ps1.
    'vsock', 'efifw', 'svga_wddm', 'vmaudio', 'vgauth', 'cblauncher',
    'vmwtimeprovider', 'vmstatsprovider', 'vmupgradehelper',
    # vmwefifw: on-disk service name for the EFI firmware component.
    'vmwefifw',
    # Legacy/newer names - mirrors 9100_cleanup_vmware.ps1.
    'VMCISockets', 'vm3dservice', 'vmxnet', 'vmx_svga', 'vmkbd',
    'vmdesched', 'vmdebug', 'vmware', 'vmx86', 'VMwareCertService'
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
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetflt',
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt',
    'HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vmci',
    # App Paths entry for vmtoolsd.exe - mirrors 9100_cleanup_vmware.ps1.
    'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\vmtoolsd.exe'
)

$VMwareDirs = @(
    "$env:ProgramFiles\VMware",
    "$env:ProgramFiles\Common Files\VMware",
    "${env:ProgramFiles(x86)}\VMware",
    "${env:ProgramFiles(x86)}\Common Files\VMware",
    "$env:ProgramData\VMware"
)

# Wildcarded temp folders - one "vmware-<account>" folder per account.
$VMwareTempDirPatterns = @(
    "$env:SystemRoot\Temp\vmware-*",
    "$env:TEMP\vmware-*"
)

$VMwareDriverFiles = @(
    'vmci.sys', 'vmmouse.sys', 'vmrawdsk.sys', 'vmhgfs.sys',
    'vmusbmouse.sys', 'vmxnet3.sys',
    'vm3dmp.sys', 'vm3dmp-debug.sys', 'vm3dmp-stats.sys', 'vm3dmp_loader.sys',
    'vsock.sys', 'vmmemctl.sys', 'vsepflt.sys', 'vnetWFP.sys', 'vnetflt.sys',
    # svga_wddm/efifw/vmaudio - mirrors 9100_cleanup_vmware.ps1.
    'svga_wddm.sys', 'efifw.sys', 'vmaudio.sys',
    # Legacy pre-WDDM driver files.
    'vmx_svga.sys', 'vmkbd.sys', 'vmdesched.sys'
)

# DriverStore dirs matching this pattern are VMware (Hyper-V's own dirs excluded).
$DriverStorePattern = '^vm(3d|ci|hgfs|mouse|rawdsk|memctl|xnet|ware|tools|vss|usb)'

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

# Excludes Hyper-V services (vmbus, vmgid, vmgen*, etc.). pvscsi excluded
# too - it's deliberately left installed by 9100_cleanup_vmware.ps1.
$SweepPattern = '^(vm(3d|ci|hgfs|mouse|rawdsk|memctl|xnet|ware|tools|vss|usb)|VGAuth|vsepflt|vnetWFP|vnetflt|GISvc)'

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

# List immediate subkey paths via reg.exe (WOW64-safe).
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
# unreadable key doesn't abort the rest of the checks (and the final
# RESULTS/exit code below).
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
# 2. VMware Tools MSI / Add-Remove Programs
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- MSI / Add-Remove Programs ---"

$UninstallRoots = @(
    'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
    'HKLM\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall'
)
$vmwareUninstallKeys = foreach ($root in $UninstallRoots) {
    foreach ($sub in (Get-RegSubKeyPaths $root)) {
        $displayName = Get-RegValue $sub 'DisplayName'
        if ($displayName -match 'VMware') { "$sub ($displayName)" }
    }
}
if (-not $vmwareUninstallKeys) {
    Check-Pass "No VMware entries in Add/Remove Programs"
} else {
    foreach ($k in $vmwareUninstallKeys) {
        Check-Fail "Add/Remove Programs entry still present: $k"
    }
}

# Enumerate every SID under UserData rather than hardcoding SYSTEM.
$UserDataRoot = 'HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\UserData'
$productsRoots = @('HKLM\SOFTWARE\Classes\Installer\Products')
foreach ($sidKey in (Get-RegSubKeyPaths $UserDataRoot)) {
    $productsRoots += "$sidKey\Products"
}

$vmwareInstallerProducts = foreach ($productsRoot in $productsRoots) {
    foreach ($sub in (Get-RegSubKeyPaths $productsRoot)) {
        $name = Get-RegValue $sub 'ProductName'
        if (-not $name) { $name = Get-RegValue "$sub\InstallProperties" 'DisplayName' }
        if ($name -match 'VMware') { "$sub ($name)" }
    }
}
if (-not $vmwareInstallerProducts) {
    Check-Pass "No orphaned VMware Windows Installer product registrations"
} else {
    foreach ($p in $vmwareInstallerProducts) {
        Check-Fail "Windows Installer product registration still present: $p"
    }
}

# Windows Installer "Components" ownership map - only cleaned
# automatically by a successful msiexec /x. Mirrors 9100_cleanup_vmware.ps1.
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

$vmwareComponentHits = foreach ($sidKey in (Get-RegSubKeyPaths $UserDataRoot)) {
    Get-RegDataMatches "$sidKey\Components" 'VMware'
}
if (-not $vmwareComponentHits) {
    Check-Pass "No orphaned VMware Windows Installer Components entries"
} else {
    Check-Fail "$(@($vmwareComponentHits).Count) orphaned Windows Installer Components entries still reference VMware (e.g. $($vmwareComponentHits[0].KeyPath))"
}

# -------------------------------------------------------------------
# 3. Driver packages (pnputil)
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- Driver Packages ---"

# Post-Broadcom-acquisition, packages can be republished as "Broadcom
# Inc." but keep the same .inf filename, so match known VMware .inf
# basenames rather than the ambiguous "Broadcom" text. vm3d.inf has no
# matching .sys (its binary is vm3dmp.sys), so it's listed explicitly.
$VMwareExtraInfNames = @('vm3d.inf')
$VMwareInfNames = ((($VMwareDriverFiles | ForEach-Object { $_ -replace '\.sys$', '.inf' }) + $VMwareExtraInfNames) |
    ForEach-Object { [regex]::Escape($_) }) -join '|'

# Get-WindowsDriver is locale-independent; pnputil's text fields are English-only.
# pvscsi excluded - 9100_cleanup_vmware.ps1 deliberately leaves its
# driver package alone, so it staying in the store isn't a failure.
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
} catch {
    Write-Host "  (Get-WindowsDriver unavailable ($_) - falling back to pnputil text parsing)"
}

if ($null -ne $publishedNames) {
    if ($publishedNames.Count -eq 0) {
        Check-Pass "No VMware driver packages in driver store"
    } else {
        foreach ($inf in $publishedNames) {
            Check-Fail "VMware driver package still in store: $inf"
        }
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
        Check-Pass "No VMware driver packages in driver store"
    } else {
        foreach ($block in $driverBlocks) {
            $inf = if ($block -match 'Published Name\s*:\s*(\S+)') { $Matches[1] } else { '(unknown)' }
            Check-Fail "VMware driver package still in store: $inf"
        }
    }
}

# DriverStore\FileRepository residuals. 9100_cleanup_vmware.ps1's Phase 6
# doesn't force-delete these (unsupported, risks CBS inconsistency) - it
# only logs them. Surfaced here too for the reviewer.
$Sys32ForDriverStore = if (Test-Path (Join-Path $env:SystemRoot 'Sysnative')) { Join-Path $env:SystemRoot 'Sysnative' } else { Join-Path $env:SystemRoot 'System32' }
$driverStorePath = Join-Path $Sys32ForDriverStore 'DriverStore\FileRepository'
if (Test-Path $driverStorePath) {
    $driverStoreResiduals = Get-ChildItem -Path $driverStorePath -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match $DriverStorePattern }
    if (-not $driverStoreResiduals) {
        Check-Pass "No residual VMware folders in DriverStore\FileRepository"
    } else {
        foreach ($residual in $driverStoreResiduals) {
            Check-Warn "Residual DriverStore folder (needs manual pnputil cleanup): $($residual.FullName)"
        }
    }
}

# -------------------------------------------------------------------
# 4. PnP devices
# -------------------------------------------------------------------
Write-Host ""
Write-Host "--- PnP Devices ---"

$vmDevices = Get-PnpDevice -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Manufacturer -match 'VMware' -or
        $_.FriendlyName -match 'VMware' -or
        $_.InstanceId -match 'VEN_15AD' -or
        # VMware's USB vendor ID - catches the USB composite parent of
        # "VMware USB Pointing Device", which doesn't say VMware itself.
        $_.InstanceId -match 'VID_0E0F' -or
        # ROOT-enumerated software devices (e.g. ROOT\VMWVMCIHOSTDEV\0000).
        $_.InstanceId -match '^ROOT\\(VMWVMCI|VMware)'
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

# Builds an InstanceId -> [DeviceClasses key path] lookup in one pass via
# .NET instead of reg.exe (mirrors 9100_cleanup_vmware.ps1). DeviceClasses
# can hold thousands of entries - walking it per device via reg.exe would
# mean tens of thousands of process spawns. DeviceClasses entries often
# carry restrictive ACLs, so every read below goes through the
# Get-Safe*/Open-Safe* helpers above.
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

# For any device still present, also check the secondary registry
# locations Clear-GhostDeviceReferences targets - a stale entry there is
# a suspected cause of Windows resynthesizing a ghost Enum entry.
if ($vmDevices -and $vmDevices.Count -gt 0) {
    $deviceClassesMap = Get-DeviceClassesInstanceMap
    foreach ($d in $vmDevices) {
        # Enum key's "Driver" value points at its class driver binding key.
        $driverBinding = Get-RegValue "HKLM\SYSTEM\CurrentControlSet\Enum\$($d.InstanceId)" 'Driver'
        if ($driverBinding) {
            $bindingKey = "HKLM\SYSTEM\CurrentControlSet\Control\Class\$driverBinding"
            if (Test-RegKey $bindingKey) {
                Check-Fail "Class driver binding still references $($d.InstanceId): $bindingKey"
            }
        }

        if ($deviceClassesMap.ContainsKey($d.InstanceId)) {
            foreach ($ifaceInstanceKey in $deviceClassesMap[$d.InstanceId]) {
                Check-Fail "DeviceClasses interface registration still references $($d.InstanceId): $ifaceInstanceKey"
            }
        }

        $containerHits = Get-RegDataMatches 'HKLM\SYSTEM\CurrentControlSet\Control\DeviceContainers' $d.InstanceId
        foreach ($hit in $containerHits) {
            Check-Warn "DeviceContainers still references $($d.InstanceId) (may be shared with another device): $($hit.KeyPath)\$($hit.ValueName)"
        }
    }
}

# -------------------------------------------------------------------
# 5. Registry
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

# Stray COM registrations (CLSID/TypeLib) - mirrors 9100_cleanup_vmware.ps1.
# Uses .NET directly rather than "reg query /s" - CLSID/TypeLib can each
# have tens of thousands of subkeys, and text-parsing took minutes.
# Ownership is decided by the COM server module path/name, not by "VMware"
# appearing anywhere in the subtree - see 9100_cleanup_vmware.ps1 for why.
Write-Host ""
Write-Host "--- COM registrations (CLSID/TypeLib) ---"
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
$strayComTotal = 0
foreach ($root in $comRoots) {
    foreach ($strayKey in (Get-VMwareStrayComKey $root.SubPath $root.LogPath)) {
        Check-Fail "Stray VMware COM registration: $strayKey"
        $strayComTotal++
    }
}
if ($strayComTotal -eq 0) {
    Check-Pass "No stray VMware COM (CLSID/TypeLib) registrations"
}

# Installer\Folders stale path references (value NAME is the folder
# path). WOW64-shared, so no separate WOW6432Node copy exists.
Write-Host ""
Write-Host "--- Installer Folders ---"
$foldersProps = Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Installer\Folders' -ErrorAction SilentlyContinue
$vmwareFolderNames = @()
if ($foldersProps) {
    $vmwareFolderNames = $foldersProps.PSObject.Properties |
        Where-Object { $_.Name -notmatch '^PS(Path|ParentPath|ChildName|Drive|Provider)$' -and $_.Name -match 'VMware' } |
        ForEach-Object { $_.Name }
}
if ($vmwareFolderNames.Count -eq 0) {
    Check-Pass "No VMware entries in Installer\Folders"
} else {
    foreach ($n in $vmwareFolderNames) {
        Check-Fail "Installer\Folders entry still present: $n"
    }
}

# Run/RunOnce autostart entries (e.g. "VMware VM3DService Process").
Write-Host ""
Write-Host "--- Run / RunOnce autostart entries ---"
function Get-VMwareRunEntryNames {
    param([string]$PsKeyPath)
    $props = Get-ItemProperty -Path $PsKeyPath -ErrorAction SilentlyContinue
    if (-not $props) { return @() }
    $props.PSObject.Properties |
        Where-Object { $_.Name -notmatch '^PS(Path|ParentPath|ChildName|Drive|Provider)$' } |
        Where-Object { $_.Name -match 'VMware' -or "$($_.Value)" -match 'VMware' } |
        ForEach-Object { "$PsKeyPath\$($_.Name)" }
}
$runKeys = @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run',
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\RunOnce'
)
$staleRunEntries = foreach ($rk in $runKeys) { Get-VMwareRunEntryNames $rk }
if (-not $staleRunEntries) {
    Check-Pass "No VMware Run/RunOnce autostart entries"
} else {
    foreach ($e in $staleRunEntries) {
        Check-Fail "Run/RunOnce entry still present: $e"
    }
}

# SVGA 3D display adapter "software device" node - separate from the
# vm3dmp_loader service key already checked above.
Write-Host ""
Write-Host "--- Display adapter (Control\Video) ---"
$videoRoot = 'HKLM\SYSTEM\CurrentControlSet\Control\Video'
$strayVideoKeys = foreach ($guidKey in (Get-RegSubKeyPaths $videoRoot)) {
    $desc = Get-RegValue "$guidKey\Video" 'DeviceDesc'
    $svc  = Get-RegValue "$guidKey\Video" 'Service'
    if ($desc -match 'VMware' -or $svc -match '^vm3dmp') { $guidKey }
}
if (-not $strayVideoKeys) {
    Check-Pass "No VMware display adapter registration under Control\Video"
} else {
    foreach ($k in $strayVideoKeys) {
        Check-Fail "VMware display adapter registration still present: $k"
    }
}

# SVGA 3D class coinstaller (vm3dc003) can silently veto removal of any
# display device if left behind. Mirrors 9100_cleanup_vmware.ps1 Phase 3.
$displayClassGuid = '{4d36e968-e325-11ce-bfc1-08002be10318}'
$coDevProp = Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\CoDeviceInstallers' -Name $displayClassGuid -ErrorAction SilentlyContinue
if ($coDevProp -and (@($coDevProp.$displayClassGuid) -match 'vm3dc003')) {
    Check-Fail "VMware SVGA class coinstaller (vm3dc003) still registered under CoDeviceInstallers"
} else {
    Check-Pass "No VMware SVGA class coinstaller registration"
}

# -------------------------------------------------------------------
# 6. Files on disk
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

# Per-user AppData folders - path depends on each profile under \Users.
$usersRoot = Join-Path $env:SystemDrive 'Users'
$staleUserDirs = @()
if (Test-Path $usersRoot) {
    # -Force: C:\Users\Default is hidden and would otherwise be skipped.
    Get-ChildItem -Path $usersRoot -Directory -Force -ErrorAction SilentlyContinue |
        ForEach-Object {
            foreach ($sub in @('AppData\Local\VMware', 'AppData\Roaming\VMware', 'AppData\LocalLow\VMware')) {
                $p = Join-Path $_.FullName $sub
                if (Test-Path $p) { $staleUserDirs += $p }
            }
        }
}
if ($staleUserDirs.Count -eq 0) {
    Check-Pass "No per-user VMware AppData folders"
} else {
    foreach ($d in $staleUserDirs) {
        Check-Fail "VMware AppData folder still exists: $d"
    }
}

# Wildcarded temp folders - mirrors 9100_cleanup_vmware.ps1's Phase 6
# sweep of vmware-<account> folders (not just vmware-SYSTEM).
$staleTempDirs = @()
foreach ($pattern in $VMwareTempDirPatterns) {
    $parent = Split-Path $pattern -Parent
    $leaf   = Split-Path $pattern -Leaf
    if (Test-Path $parent) {
        Get-ChildItem -Path $parent -Directory -Filter $leaf -ErrorAction SilentlyContinue |
            ForEach-Object { $staleTempDirs += $_.FullName }
    }
}
if ($staleTempDirs.Count -eq 0) {
    Check-Pass "No VMware temp folders (vmware-*)"
} else {
    foreach ($d in $staleTempDirs) {
        Check-Fail "VMware temp folder still exists: $d"
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
# 7. Scheduled tasks
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
# 8. Component store integrity (DISM CheckHealth)
# -------------------------------------------------------------------
# Catches component-store corruption, e.g. from DriverStore residuals
# that 9100_cleanup_vmware.ps1's Phase 6 deliberately doesn't force-delete.
# `sfc /scannow` is intentionally skipped here - it can take 10-20+
# minutes, disproportionate to this check; run it manually if DISM flags
# corruption. DISM's output is English-only; non-English falls through
# to a WARN with the raw text.
Write-Host ""
Write-Host "--- Component Store Integrity ---"

Write-Host "  Running DISM /Online /Cleanup-Image /CheckHealth..."
# Bare "DISM.exe" resolves via WOW64 to the 32-bit SysWOW64 copy on a
# 32-bit PowerShell host, which can't service a running 64-bit image -
# $Sys32 (Sysnative on WOW64) forces the real 64-bit DISM.
$DismExe = Join-Path $Sys32 'Dism.exe'
$dismOutput = & $DismExe /Online /Cleanup-Image /CheckHealth 2>&1 | Out-String
$dismNormalized = ($dismOutput.ToLowerInvariant() -replace '[^a-z]', '')
if ($dismNormalized -match 'nocomponentstorecorruptiondetected') {
    Check-Pass "DISM /CheckHealth: no component store corruption detected"
} elseif ($dismNormalized -match 'isrepairable') {
    Check-Warn "DISM /CheckHealth: component store corruption detected (repairable) - run DISM /Online /Cleanup-Image /RestoreHealth"
} elseif ($dismNormalized -match 'isnotrepairable') {
    Check-Fail "DISM /CheckHealth: component store corruption detected (NOT repairable) - review $env:windir\Logs\DISM\dism.log"
} else {
    Check-Warn "DISM /CheckHealth: could not parse result (non-English output?) - raw: $($dismOutput.Trim())"
}

# -------------------------------------------------------------------
# 9. Windows Update Agent driver-install history (msu provider)
# -------------------------------------------------------------------
# Informational only (WARN, never FAIL). Since the Broadcom/VMware
# acquisition, driver packages can show up under
# Get-Package -ProviderName msu. This is just WU's own install-history
# bookkeeping with no supported removal path, and doesn't mean the
# driver/service is still active - surfaced so it isn't a surprise later.
Write-Host ""
Write-Host "--- Windows Update Driver History (informational) ---"

# Only match "VMware" by name - the generic "Broadcom" vendor name would
# also flag unrelated Broadcom NIC/storage drivers that have nothing to do
# with VMware Tools, and Get-Package's Name field has no VMware-specific
# text to disambiguate a republished VMware driver from those.
$msuEntries = $null
$msuAvailable = $false
try {
    $msuEntries = Get-Package -ProviderName msu -ErrorAction Stop |
        Where-Object { $_.Name -match 'VMware' }
    $msuAvailable = $true
} catch {
    Write-Host "  (Get-Package msu provider unavailable on this system - skipped)"
}

if ($msuEntries) {
    foreach ($e in $msuEntries) {
        Check-Warn "WU package history still references: $($e.Name) (Status=$($e.Status)) - expected residue, no supported removal path"
    }
} elseif ($msuAvailable) {
    Check-Pass "No VMware entries in Windows Update package history"
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
