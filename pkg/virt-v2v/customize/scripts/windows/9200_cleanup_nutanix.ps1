# 9200_cleanup_nutanix.ps1
# Remove Nutanix Guest Tools and Nutanix VirtIO driver packages so that
# Windows rebinds to the Red Hat VirtIO drivers installed by virt-v2v.
# Run as Administrator on first boot after migration.

$ErrorActionPreference = 'Continue'

$Sysnative = Join-Path $env:SystemRoot 'Sysnative'
if (Test-Path $Sysnative) {
    $Sys32   = $Sysnative
} else {
    $Sys32   = Join-Path $env:SystemRoot 'System32'
}
$PnpUtil = Join-Path $Sys32 'pnputil.exe'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$LogFile   = Join-Path $ScriptDir 'cleanup_nutanix.log'

function Log {
    param([string]$msg)
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    $line = "[$ts] $msg"
    Write-Host $line
    Add-Content -Path $LogFile -Value $line
}

# .NET registry reads throw a terminating SecurityException/
# UnauthorizedAccessException on a restricted-ACL key, which neither
# $ErrorActionPreference value suppresses - swallow it here so one unreadable
# key doesn't abort the whole scan (same pattern as 9100_cleanup_vmware.ps1).
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

Log '==============================================================='
Log '  Nutanix Driver Cleanup Script'
Log '==============================================================='

# -------------------------------------------------------------------
# PHASE 0: Uninstall Nutanix Guest Tools and Nutanix VirtIO via MSI
# -------------------------------------------------------------------
Log ''
Log '  PHASE 0: Nutanix MSI Uninstall'
Log '---------------------------------------------------------------'

# Enumerate through the explicit 64-bit registry view. The firstboot runner
# launches a 32-bit PowerShell (see the Sysnative handling above), and
# ...\CurrentVersion\Uninstall is WOW64-redirected: reading it through the
# HKLM: drive from a 32-bit process yields only the 32-bit hive, so 64-bit
# installs are invisible. NGT registers its sub-MSIs (Infrastructure
# Components Package 1/2, VSS Modules, Guest Agent, Self Service Restore) as
# 64-bit products, which is why the redirected scan found none of them.
$UninstallRoots = @(
    'SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
    'SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall'
)

# Pass 1: collect matching products. Uninstalling mutates these same keys, so
# the scan is completed before any msiexec call rather than interleaved.
$targets = @()
$hklm = $null
try {
    $hklm = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine', 'Registry64')
} catch {
    Log "[WARNING] Could not open the 64-bit registry view ($_)"
}
if (-not $hklm) {
    # 32-bit Windows has no alternate view (the WOW64 flags are meaningless
    # there), so fall back to the default view rather than skipping Phase 0.
    try {
        $hklm = [Microsoft.Win32.RegistryKey]::OpenBaseKey('LocalMachine', 'Default')
        Log '[INFO] Using the default registry view (32-bit Windows)'
    } catch {
        Log "[WARNING] Could not open HKLM ($_) - skipping MSI uninstall"
    }
}

if ($hklm) {
    foreach ($subPath in $UninstallRoots) {
        $rootKey = Open-SafeSubKey $hklm $subPath
        if (-not $rootKey) { continue }
        foreach ($subName in (Get-SafeSubKeyNames $rootKey)) {
            $subKey = Open-SafeSubKey $rootKey $subName
            if (-not $subKey) { continue }
            $displayName  = Get-SafeRegValue $subKey 'DisplayName'
            $uninstallStr = Get-SafeRegValue $subKey 'UninstallString'
            $installDate  = Get-SafeRegValue $subKey 'InstallDate'
            $subKey.Close()

            if ($displayName -notmatch 'Nutanix') { continue }

            $productCode = $null
            if ($subName -match '(\{[0-9A-Fa-f\-]{36}\})') {
                $productCode = $Matches[1]
            } elseif ($uninstallStr -match '(\{[0-9A-Fa-f\-]{36}\})') {
                $productCode = $Matches[1]
            }

            $targets += [PSCustomObject]@{
                DisplayName = $displayName
                ProductCode = $productCode
                InstallDate = [string]$installDate
            }
        }
        $rootKey.Close()
    }
    $hklm.Close()
}

Log "[INFO] Found $($targets.Count) Nutanix product(s) in the uninstall registry"

# Pass 2: uninstall newest first. The NGT sub-packages depend on the
# Infrastructure Components packages, so removing in reverse install order
# avoids msiexec refusing to remove a still-referenced dependency.
$msiRemoved = 0
foreach ($target in ($targets | Sort-Object InstallDate -Descending)) {
    $displayName = $target.DisplayName
    if (-not $target.ProductCode) {
        Log "[INFO] No product code found for '$displayName' - skipping MSI uninstall"
        continue
    }
    Log "[MSI] Uninstalling '$displayName' ($($target.ProductCode)) ..."
    $p = Start-Process -FilePath 'msiexec.exe' `
        -ArgumentList "/x $($target.ProductCode) /qn /norestart" `
        -PassThru -ErrorAction SilentlyContinue
    if ($p -and $p.WaitForExit(300000)) {
        if ($p.ExitCode -in 0, 1641, 3010) {
            Log "[SUCCESS] Uninstalled '$displayName'"
            $msiRemoved++
        } elseif ($p.ExitCode -eq 1605) {
            # Already gone - typically removed as part of a parent bundle.
            Log "[INFO] '$displayName' was already removed (msiexec 1605)"
        } else {
            Log "[WARNING] msiexec exited $($p.ExitCode) for '$displayName'"
        }
    } else {
        Log "[WARNING] msiexec timed out or failed to start for '$displayName'"
        if ($p) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
    }
}
Log "[INFO] Nutanix MSI products uninstalled: $msiRemoved"

# -------------------------------------------------------------------
# PHASE 1: Remove Nutanix driver packages from the driver store
# -------------------------------------------------------------------
Log ''
Log '  PHASE 1: Nutanix VirtIO Driver Packages'
Log '---------------------------------------------------------------'

$drvRemoved = 0
try {
    $nutanixDrivers = @(
        Get-WindowsDriver -Online -ErrorAction Stop |
            Where-Object { $_.ProviderName -match 'Nutanix' } |
            ForEach-Object { $_.Driver }
    )
    Log "[INFO] Found $($nutanixDrivers.Count) Nutanix driver package(s) via Get-WindowsDriver"
    foreach ($inf in $nutanixDrivers) {
        Log "[DRIVER] Removing $inf ..."
        $result = & $PnpUtil /delete-driver "$inf" /uninstall /force 2>&1 | Out-String
        $rc = $LASTEXITCODE
        Log "  $($result.Trim())"
        if ($rc -eq 0 -or $rc -eq 3010) { $drvRemoved++ } else { Log "[WARNING] pnputil exited $rc for $inf" }
    }
} catch {
    Log "[WARNING] Get-WindowsDriver unavailable ($_) - falling back to pnputil text parsing"
    $pnpOutput = & $PnpUtil /enum-drivers 2>&1 | Out-String
    $driverBlocks = $pnpOutput -split '(?=Published Name)' |
        Where-Object { $_ -match 'Nutanix' }
    Log "[INFO] Found $($driverBlocks.Count) Nutanix driver package(s) via pnputil"
    foreach ($block in $driverBlocks) {
        $inf = if ($block -match 'Published Name\s*:\s*(\S+)') { $Matches[1] } else { '(unknown)' }
        Log "[DRIVER] Removing $inf ..."
        $result = & $PnpUtil /delete-driver "$inf" /uninstall /force 2>&1 | Out-String
        $rc = $LASTEXITCODE
        Log "  $($result.Trim())"
        if ($rc -eq 0 -or $rc -eq 3010) { $drvRemoved++ } else { Log "[WARNING] pnputil exited $rc for $inf" }
    }
}
Log "[INFO] Nutanix driver packages removed: $drvRemoved"

# -------------------------------------------------------------------
# SUMMARY
# -------------------------------------------------------------------
Log ''
Log '==============================================================='
Log '  Nutanix Cleanup Complete'
Log '==============================================================='
Log "  MSI products uninstalled: $msiRemoved"
Log "  Driver packages removed:  $drvRemoved"
Log "  Log: $LogFile"
Log '==============================================================='
