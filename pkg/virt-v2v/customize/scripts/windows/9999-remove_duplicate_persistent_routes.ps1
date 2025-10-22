# Requires Administrator privileges to run

Write-Host "Starting persistent route cleanup script..." -ForegroundColor Green

# Function to convert CIDR prefix length to subnet mask
function Convert-PrefixToMask($prefix) {
    $bin = ("1" * $prefix).PadRight(32, "0")
    $octets = @(
        $bin.Substring(0, 8)
        $bin.Substring(8, 8)
        $bin.Substring(16, 8)
        $bin.Substring(24, 8)
    ) | ForEach-Object { [Convert]::ToInt32($_, 2) }

    return ($octets -join ".")
}

# Get persistent routes
try {
    $routes = Get-NetRoute -PolicyStore PersistentStore -ErrorAction Stop
} catch {
    Write-Host "Error retrieving persistent routes: $_" -ForegroundColor Red
    Exit 1
}

$routes = $routes | Where-Object { $_.AddressFamily -eq "IPv4" }

$groupedRoutes = $routes | Group-Object {
    "$($_.DestinationPrefix)-$($_.NextHop)-$($_.RouteMetric)"
} | Where-Object { $_.Count -gt 1 }

if (-not $groupedRoutes) {
    Write-Host "No duplicate persistent routes found." -ForegroundColor Green
} else {
    Write-Host "Cleaning up duplicate persistent routes..." -ForegroundColor Yellow

    foreach ($group in $groupedRoutes) {
        $toKeep = $group.Group[0]
        $dest = $toKeep.DestinationPrefix
        $gateway = $toKeep.NextHop
        $metric = $toKeep.RouteMetric
        $interfaceIndex = $toKeep.InterfaceIndex

        $parts = $dest.Split("/")
        if ($parts.Count -ne 2) {
            Write-Host "  Invalid destination prefix format: $dest" -ForegroundColor Red
            continue
        }

        $network = $parts[0]
        $prefix = [int]$parts[1]
        $netmask = Convert-PrefixToMask $prefix
        $metricStr = if ($metric -is [int]) { "METRIC $metric" } else { "" }

        # Delete all matching routes
        foreach ($route in $group.Group) {
            try {
                Remove-NetRoute -DestinationPrefix $route.DestinationPrefix -NextHop $route.NextHop `
                    -InterfaceIndex $route.InterfaceIndex -PolicyStore PersistentStore -Confirm:$false -ErrorAction Stop
                Write-Host "  Deleted: $($route.DestinationPrefix) via $($route.NextHop)" -ForegroundColor Red
            } catch {
                Write-Host "    Failed to delete route: $($_.Exception.Message)" -ForegroundColor Red
            }
        }

        # Try re-adding route using New-NetRoute with PolicyStore
        $reAddSucceeded = $false
        try {
            New-NetRoute -DestinationPrefix $dest -InterfaceIndex $interfaceIndex `
                -NextHop $gateway -RouteMetric $metric -PolicyStore PersistentStore -ErrorAction Stop
            Write-Host "  Re-added with New-NetRoute: $dest via $gateway metric $metric" -ForegroundColor Green
            $reAddSucceeded = $true
        } catch {
            Write-Host "  New-NetRoute failed: $($_.Exception.Message), falling back to route.exe " -ForegroundColor Yellow
        }

        # Fallback to route.exe if New-NetRoute failed
        if (-not $reAddSucceeded) {
            # Re-add one preserved route using legacy route.exe
            $command = "route -p ADD $network MASK $netmask $gateway IF $interfaceIndex"
            if ($metricStr -ne "") {
                $command += " $metricStr"
            }

            try {
                cmd /c $command
                Write-Host "  Re-added with route.exe: $dest via $gateway metric $metric on IF $interfaceIndex" -ForegroundColor Green
            } catch {
                Write-Host "    Failed to re-add route: $($_.Exception.Message)" -ForegroundColor Red
            }
        }
    }
}

