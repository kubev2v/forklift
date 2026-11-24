# 9200_cleanup_nutanix.ps1
# Remove Nutanix Guest Tools and Nutanix VirtIO driver packages so that
# Windows rebinds to the Red Hat VirtIO drivers installed by virt-v2v.
# Run as Administrator on first boot after migration.
#
# Some guests enforce PowerShell Constrained Language Mode (AppLocker/WDAC
# policy), which blocks .NET method calls on most types (e.g.
# [Microsoft.Win32.RegistryKey]::OpenBaseKey(), $process.WaitForExit()) with
# "Cannot invoke method. Method invocation is supported only on core types in
# this language mode". Keep this script to cmdlets, native commands and
# property access so it works there too.

param([switch]$Relaunched)

$ErrorActionPreference = 'Continue'

$ScriptPath = $MyInvocation.MyCommand.Definition
$ScriptDir  = Split-Path -Parent $ScriptPath
$LogFile    = Join-Path $ScriptDir 'cleanup_nutanix.log'

function Log {
    param([string]$msg)
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    $line = "[$ts] $msg"
    Write-Host $line
    Add-Content -Path $LogFile -Value $line
}

# The firstboot runner launches a 32-bit PowerShell. On 64-bit Windows a
# 32-bit process gets a WOW64-redirected view of the registry, so
# ...\CurrentVersion\Uninstall shows only the 32-bit hive and the 64-bit
# products NGT registers (Infrastructure Components Package 1/2, VSS Modules,
# Guest Agent, Self Service Restore) are invisible. Rather than requesting the
# 64-bit view explicitly - which needs .NET registry calls that Constrained
# Language Mode blocks - re-run this script in the native 64-bit PowerShell,
# where the plain registry cmdlets see both hives. PROCESSOR_ARCHITEW6432 is
# only set in a WOW64 process; -Relaunched guards against a relaunch loop.
if ($env:PROCESSOR_ARCHITEW6432 -and -not $Relaunched) {
    $NativePowerShell = Join-Path $env:SystemRoot 'Sysnative\WindowsPowerShell\v1.0\powershell.exe'
    if (Test-Path $NativePowerShell) {
        Log '[INFO] Running in 32-bit PowerShell - relaunching in 64-bit PowerShell'
        & $NativePowerShell -NoProfile -ExecutionPolicy Bypass -File $ScriptPath -Relaunched
        exit $LASTEXITCODE
    }
    Log '[WARNING] 64-bit PowerShell not found - continuing in 32-bit PowerShell; 64-bit products may be missed'
}

# Sysnative only exists for a 32-bit process on 64-bit Windows, i.e. when the
# relaunch above was not possible.
$Sysnative = Join-Path $env:SystemRoot 'Sysnative'
if (Test-Path $Sysnative) {
    $Sys32   = $Sysnative
} else {
    $Sys32   = Join-Path $env:SystemRoot 'System32'
}
$PnpUtil = Join-Path $Sys32 'pnputil.exe'

Log '==============================================================='
Log '  Nutanix Driver Cleanup Script'
Log '==============================================================='
Log "[INFO] Process architecture: $env:PROCESSOR_ARCHITECTURE, language mode: $($ExecutionContext.SessionState.LanguageMode)"

# -------------------------------------------------------------------
# PHASE 0: Uninstall Nutanix Guest Tools and Nutanix VirtIO via MSI
# -------------------------------------------------------------------
Log ''
Log '  PHASE 0: Nutanix MSI Uninstall'
Log '---------------------------------------------------------------'

# In a 64-bit process the first root is the 64-bit hive and WOW6432Node is
# the 32-bit hive. Get-ItemProperty is used instead of RegistryKey methods
# (OpenSubKey/GetValue), which Constrained Language Mode blocks; an
# unreadable key just produces a non-terminating error that is skipped
# rather than aborting the scan.
$UninstallRoots = @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)

# Pass 1: collect matching products. Uninstalling mutates these same keys, so
# the scan is completed before any msiexec call rather than interleaved.
$targets = @()
$seenCodes = @{}
foreach ($root in $UninstallRoots) {
    foreach ($entry in @(Get-ItemProperty -Path $root -ErrorAction SilentlyContinue)) {
        $displayName = [string]$entry.DisplayName
        if ($displayName -notmatch 'Nutanix') { continue }

        $productCode = $null
        if ($entry.PSChildName -match '(\{[0-9A-Fa-f\-]{36}\})') {
            $productCode = $Matches[1]
        } elseif ([string]$entry.UninstallString -match '(\{[0-9A-Fa-f\-]{36}\})') {
            $productCode = $Matches[1]
        }

        # Without the relaunch both roots resolve to the 32-bit hive, so the
        # same product would be listed twice.
        if ($productCode) {
            if ($seenCodes.ContainsKey($productCode)) { continue }
            $seenCodes[$productCode] = $true
        }

        # A plain hashtable, not [PSCustomObject]@{...}: on Windows PowerShell
        # 5.1 that cast goes through an internal type that Constrained
        # Language Mode rejects (ConversionSupportedOnlyToCoreTypes).
        $targets += @{
            DisplayName = $displayName
            ProductCode = $productCode
            InstallDate = [string]$entry.InstallDate
        }
    }
}

Log "[INFO] Found $($targets.Count) Nutanix product(s) in the uninstall registry"

# Pass 2: uninstall newest first. The NGT sub-packages depend on the
# Infrastructure Components packages, so removing in reverse install order
# avoids msiexec refusing to remove a still-referenced dependency.
$msiRemoved = 0
# Sort on a script block: hashtable keys aren't properties, so
# Sort-Object InstallDate would see $null for every entry.
foreach ($target in ($targets | Sort-Object { $_.InstallDate } -Descending)) {
    $displayName = $target.DisplayName
    if (-not $target.ProductCode) {
        Log "[INFO] No product code found for '$displayName' - skipping MSI uninstall"
        continue
    }
    Log "[MSI] Uninstalling '$displayName' ($($target.ProductCode)) ..."
    $p = Start-Process -FilePath 'msiexec.exe' `
        -ArgumentList "/x $($target.ProductCode) /qn /norestart" `
        -PassThru -ErrorAction SilentlyContinue
    if (-not $p) {
        Log "[WARNING] msiexec failed to start for '$displayName'"
        continue
    }
    # $p.WaitForExit() is blocked in Constrained Language Mode, so wait with
    # Wait-Process. Reading Handle first keeps the process handle open, without
    # which ExitCode can come back empty after the process exits.
    $null = $p.Handle
    Wait-Process -InputObject $p -Timeout 300 -ErrorAction SilentlyContinue
    if (-not $p.HasExited) {
        Log "[WARNING] msiexec timed out for '$displayName'"
        Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
        continue
    }
    $exitCode = $p.ExitCode
    if ($exitCode -in 0, 1641, 3010) {
        Log "[SUCCESS] Uninstalled '$displayName'"
        $msiRemoved++
    } elseif ($exitCode -eq 1605) {
        # Already gone - typically removed as part of a parent bundle.
        Log "[INFO] '$displayName' was already removed (msiexec 1605)"
    } else {
        Log "[WARNING] msiexec exited $exitCode for '$displayName'"
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
