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

Log '==============================================================='
Log '  Nutanix Driver Cleanup Script'
Log '==============================================================='

# -------------------------------------------------------------------
# PHASE 0: Uninstall Nutanix Guest Tools and Nutanix VirtIO via MSI
# -------------------------------------------------------------------
Log ''
Log '  PHASE 0: Nutanix MSI Uninstall'
Log '---------------------------------------------------------------'

$UninstallRoots = @(
    'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall',
    'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall'
)

$msiRemoved = 0
foreach ($root in $UninstallRoots) {
    if (-not (Test-Path $root)) { continue }
    Get-ChildItem $root -ErrorAction SilentlyContinue | ForEach-Object {
        $displayName = $_.GetValue('DisplayName')
        if ($displayName -match 'Nutanix') {
            $productCode = $null
            if ($_.PSChildName -match '(\{[0-9A-Fa-f\-]{36}\})') {
                $productCode = $Matches[1]
            } else {
                $uninstallStr = $_.GetValue('UninstallString')
                if ($uninstallStr -match '(\{[0-9A-Fa-f\-]{36}\})') {
                    $productCode = $Matches[1]
                }
            }
            if ($productCode) {
                Log "[MSI] Uninstalling '$displayName' ($productCode) ..."
                $p = Start-Process -FilePath 'msiexec.exe' `
                    -ArgumentList "/x $productCode /qn /norestart" `
                    -PassThru -ErrorAction SilentlyContinue
                if ($p -and $p.WaitForExit(300000)) {
                    if ($p.ExitCode -in 0, 1641, 3010) {
                        Log "[SUCCESS] Uninstalled '$displayName'"
                        $msiRemoved++
                    } else {
                        Log "[WARNING] msiexec exited $($p.ExitCode) for '$displayName'"
                    }
                } else {
                    Log "[WARNING] msiexec timed out or failed to start for '$displayName'"
                    if ($p) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
                }
            } else {
                Log "[INFO] No product code found for '$displayName' - skipping MSI uninstall"
            }
        }
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
        Log "  $($result.Trim())"
        $drvRemoved++
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
        Log "  $($result.Trim())"
        $drvRemoved++
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
