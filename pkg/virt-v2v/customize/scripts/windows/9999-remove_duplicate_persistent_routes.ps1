# Requires Administrator privileges to run

Write-Host "Starting persistent route cleanup script..." -ForegroundColor Green

# Optional Debug: Backup 'route print' to file
#$timestamp = Get-Date -Format 'yyyyMMdd_HHmmss'
#$backupPath = "C:\persistent_routes_backup_$timestamp.txt"
# Write-Host "Backing up 'route print' output to $backupPath..." -ForegroundColor Yellow
# route print > $backupPath 2>&1

$routePrintOutput = route print | Out-String

# 2. Parse Persistent Routes
$headerPattern = 'Persistent Routes:'
$startIndex = $routePrintOutput.IndexOf($headerPattern)

if ($startIndex -eq -1) {
    Write-Host "Could not find 'Persistent Routes:' in route print output." -ForegroundColor Red
    Exit 1
}

$sectionAfterHeader = $routePrintOutput.Substring($startIndex + $headerPattern.Length)
$activeRoutesStartIndex = $sectionAfterHeader.IndexOf('Active Routes:')
$persistentRoutesBlock = if ($activeRoutesStartIndex -ne -1) {
    $sectionAfterHeader.Substring(0, $activeRoutesStartIndex).Trim()
} else {
    $sectionAfterHeader.Trim()
}

$persistentRouteLines = $persistentRoutesBlock.Split([System.Environment]::NewLine, [System.StringSplitOptions]::RemoveEmptyEntries) | Where-Object {
    $_ -notmatch 'Network Address' -and $_ -notmatch '^-+$' -and $_ -notmatch '^\s*$' -and $_ -notmatch 'IPv6 Route Table'
}

$routeLineRegex = '^\s*(\d{1,3}(?:\.\d{1,3}){3})\s+(\d{1,3}(?:\.\d{1,3}){3})\s+(\d{1,3}(?:\.\d{1,3}){3})\s+(Default|\d+)\s*$'

$routes = @()
foreach ($line in $persistentRouteLines) {
    $match = $line | Select-String -Pattern $routeLineRegex -AllMatches | Select-Object -ExpandProperty Matches
    if ($match) {
        $routes += [PSCustomObject]@{
            Network = $match[0].Groups[1].Value
            Netmask = $match[0].Groups[2].Value
            Gateway = $match[0].Groups[3].Value
            Metric = $match[0].Groups[4].Value
            OriginalLine = $line.Trim()
            Key = "$($match[0].Groups[1].Value)-$($match[0].Groups[2].Value)-$($match[0].Groups[3].Value)-$($match[0].Groups[4].Value)"
        }
    }
}

# 3. Identify duplicates and clean up
$duplicateGroups = $routes | Group-Object -Property Key | Where-Object { $_.Count -gt 1 }

if (-not $duplicateGroups) {
    Write-Host "No duplicate persistent routes found." -ForegroundColor Green
} else {
    Write-Host "Cleaning up duplicate persistent routes..." -ForegroundColor Yellow
    foreach ($group in $duplicateGroups) {
        $routeToPreserve = $group.Group[0]
        $network = $routeToPreserve.Network
        $netmask = $routeToPreserve.Netmask
        $gateway = $routeToPreserve.Gateway
        $metric = $routeToPreserve.Metric

        $deleteCmd = "route DELETE $network MASK $netmask $gateway"
        Write-Host "  Deleting ALL duplicates: $deleteCmd" -ForegroundColor Red
        try {
            Invoke-Expression $deleteCmd | Out-Null
            Write-Host "    Deleted successfully." -ForegroundColor Green
        } catch {
            Write-Host "    Failed to delete route: $_" -ForegroundColor Red
            continue
        }

        $metricStr = if ($metric -eq "Default") { "METRIC 10" } elseif ($metric -match '^\d+$') { "METRIC $metric" } else { "" }
        $addCmd = "route -p ADD $network MASK $netmask $gateway $metricStr"

        Write-Host "  Re-adding one copy: $addCmd" -ForegroundColor Cyan
        try {
            Invoke-Expression $addCmd | Out-Null
            Write-Host "    Re-added successfully." -ForegroundColor Green
        } catch {
            Write-Host "    Failed to re-add route: $_" -ForegroundColor Red
        }
    }
}

Write-Host "`nFinal Persistent Routes after cleanup:" -ForegroundColor Green
