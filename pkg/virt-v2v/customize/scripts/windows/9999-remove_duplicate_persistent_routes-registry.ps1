# Requires Administrator privileges to run

Write-Host "Starting persistent route cleanup script..." -ForegroundColor Green

# Lookup table avoids [Convert]::ToInt32() which is blocked in Constrained Language Mode
$PrefixToMaskTable = @(
    "0.0.0.0",         "128.0.0.0",       "192.0.0.0",       "224.0.0.0",
    "240.0.0.0",       "248.0.0.0",       "252.0.0.0",       "254.0.0.0",
    "255.0.0.0",       "255.128.0.0",     "255.192.0.0",     "255.224.0.0",
    "255.240.0.0",     "255.248.0.0",     "255.252.0.0",     "255.254.0.0",
    "255.255.0.0",     "255.255.128.0",   "255.255.192.0",   "255.255.224.0",
    "255.255.240.0",   "255.255.248.0",   "255.255.252.0",   "255.255.254.0",
    "255.255.255.0",   "255.255.255.128", "255.255.255.192", "255.255.255.224",
    "255.255.255.240", "255.255.255.248", "255.255.255.252", "255.255.255.254",
    "255.255.255.255"
)

function Convert-PrefixToMask($prefix) {
    return $PrefixToMaskTable[[int]$prefix]
}

# Remove a persistent route using both Remove-NetRoute and route.exe as fallback.
# Remove-NetRoute -PolicyStore PersistentStore can silently fail on some Windows versions,
# so we always also invoke route.exe to guarantee the persistent store entry is gone.
function Remove-PersistentRoute($route) {
    try {
        Remove-NetRoute -DestinationPrefix $route.DestinationPrefix -NextHop $route.NextHop `
            -InterfaceIndex $route.InterfaceIndex -PolicyStore PersistentStore -Confirm:$false -ErrorAction Stop
    } catch {
        Write-Host "      Remove-NetRoute failed: $($_.Exception.Message)" -ForegroundColor Red
    }
    $parts = $route.DestinationPrefix.Split("/")
    $prefixLen = [int]$parts[1]
    if ($prefixLen -le 32) {
        $network = $parts[0]
        $netmask = Convert-PrefixToMask $prefixLen
        $result = cmd /c "route delete $network mask $netmask $($route.NextHop) IF $($route.InterfaceIndex)" 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Host "      route.exe delete failed (exit $LASTEXITCODE): $result" -ForegroundColor Yellow
        }
    }
}

# Get persistent routes
try {
    $routes = Get-NetRoute -PolicyStore PersistentStore -ErrorAction Stop
} catch {
    Write-Host "Error retrieving persistent routes: $_" -ForegroundColor Red
    Exit 1
}

$routes = $routes | Where-Object { $_.AddressFamily -eq "IPv4" }

# Step 1: Preserve ALL default gateways (0.0.0.0/0) with their interface indexes
$defaultGateways = $routes | Where-Object { $_.DestinationPrefix -eq "0.0.0.0/0" }
Write-Host "Found $($defaultGateways.Count) default gateway(s) to preserve:" -ForegroundColor Cyan
foreach ($gw in $defaultGateways) {
    Write-Host "  Gateway: $($gw.NextHop) on Interface $($gw.InterfaceIndex) with metric $($gw.RouteMetric)" -ForegroundColor Cyan
}

# Step 2: Clean up duplicate routes (including default gateways)
Write-Host "Analyzing routes for duplicates..." -ForegroundColor Yellow

# Group routes by destination/gateway/metric, intentionally excluding InterfaceIndex.
# After migration, stale persistent routes from the source VM's old adapters (e.g. VMware)
# remain alongside new routes on the target adapter (e.g. VirtIO). These must be grouped
# together to detect and remove the stale entries.
$groupedRoutes = $routes | Group-Object {
    "$($_.DestinationPrefix)-$($_.NextHop)-$($_.RouteMetric)"
} | Where-Object { $_.Count -gt 1 }

# Separate default gateway duplicates from other duplicates
$gatewayDuplicates = $groupedRoutes | Where-Object { $_.Name.Trim().StartsWith("0.0.0.0/0-") }
$nonGatewayDuplicates = $groupedRoutes | Where-Object { -not $_.Name.Trim().StartsWith("0.0.0.0/0-") }

Write-Host "Found $($gatewayDuplicates.Count) duplicate default gateway groups" -ForegroundColor Cyan
Write-Host "Found $($nonGatewayDuplicates.Count) duplicate non-gateway route groups" -ForegroundColor Cyan

if (-not $groupedRoutes) {
    Write-Host "No duplicate persistent routes found." -ForegroundColor Green
} else {
    Write-Host "Cleaning up duplicate persistent routes..." -ForegroundColor Yellow
    
    # First, handle duplicate default gateways
    foreach ($group in $gatewayDuplicates) {
        $routes = $group.Group
        
        # If all interfaces in this group are active, this is a legitimate multi-homed
        # configuration (same gateway reachable via multiple NICs) — leave it alone.
        $activeCount = 0
        foreach ($r in $routes) {
            if (Get-NetAdapter | Where-Object { $_.InterfaceIndex -eq $r.InterfaceIndex } -ErrorAction SilentlyContinue) {
                $activeCount++
            }
        }
        if ($activeCount -eq $routes.Count) {
            Write-Host "  Skipping group '$($group.Name)': all $activeCount interfaces are active (multi-homed)" -ForegroundColor Gray
            continue
        }

        # Choose the route with an interface that actually exists
        $toKeep = $null
        foreach ($route in $routes) {
            $interface = Get-NetAdapter | Where-Object { $_.InterfaceIndex -eq $route.InterfaceIndex } -ErrorAction SilentlyContinue
            if ($interface) {
                $toKeep = $route
                break
            }
        }
        
        # If no existing interface found, just use the first one
        if (-not $toKeep) {
            $toKeep = $routes[0]
        }
        
        $dest = $toKeep.DestinationPrefix
        $gateway = $toKeep.NextHop
        $metric = $toKeep.RouteMetric
        
        Write-Host "  Cleaning duplicate default gateway: $gateway (metric $metric) - keeping IF $($toKeep.InterfaceIndex)" -ForegroundColor Yellow
        
        # Remove ALL instances
        foreach ($route in $routes) {
            Remove-PersistentRoute $route
            Write-Host "    Deleted: $($route.DestinationPrefix) via $($route.NextHop) on IF $($route.InterfaceIndex)" -ForegroundColor Red
        }
        
        # Re-add only ONE instance (preserve the first interface)
        $reAddSucceeded = $false
        try {
            New-NetRoute -DestinationPrefix $dest -InterfaceIndex $toKeep.InterfaceIndex -NextHop $gateway -RouteMetric $metric -PolicyStore PersistentStore -ErrorAction Stop
            Write-Host "    Re-added: $dest via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
            $reAddSucceeded = $true
        } catch {
            Write-Host "      PolicyStore method failed: $($_.Exception.Message)" -ForegroundColor Yellow
        }
        
        # Fallback to route.exe if PolicyStore failed
        if (-not $reAddSucceeded) {
            try {
                $parts = $dest.Split("/")
                $network = $parts[0]
                $prefix = [int]$parts[1]
                $netmask = Convert-PrefixToMask $prefix
                $metricStr = if ($null -ne $metric) { "METRIC $([int]$metric)" } else { "" }
                $command = "route -p ADD $network MASK $netmask $gateway IF $($toKeep.InterfaceIndex) $metricStr"
                Write-Host "      Trying route.exe: $command" -ForegroundColor Gray
                cmd /c $command
                if ($LASTEXITCODE -ne 0) {
                    Write-Error "route.exe failed with exit code $LASTEXITCODE, command: $command"
                } else {
                    Write-Host "    Re-added with route.exe: $dest via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
                }
            } catch {
                Write-Host "      route.exe also failed: $($_.Exception.Message)" -ForegroundColor Red
            }
        }
    }
    
    # Then handle non-gateway duplicates
    foreach ($group in $nonGatewayDuplicates) {
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
        $metricStr = if ($null -ne $metric) { "METRIC $([int]$metric)" } else { "" }

        # Delete all matching routes
        foreach ($route in $group.Group) {
            Remove-PersistentRoute $route
            Write-Host "  Deleted: $($route.DestinationPrefix) via $($route.NextHop)" -ForegroundColor Red
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
            $command = "route -p ADD $network MASK $netmask $gateway IF $interfaceIndex"
            if ($metricStr -ne "") {
                $command += " $metricStr"
            }

            try {
                cmd /c $command
                if ($LASTEXITCODE -ne 0) {
                    Write-Error "route.exe failed with exit code $LASTEXITCODE, command: $command"
                } else {
                    Write-Host "  Re-added with route.exe: $dest via $gateway metric $metric on IF $interfaceIndex" -ForegroundColor Green
                }
            } catch {
                Write-Host "    Failed to re-add route: $($_.Exception.Message)" -ForegroundColor Red
            }
        }
    }
}

# Step 3: Remove persistent routes bound to interfaces that no longer exist (stale after migration)
Write-Host "Removing stale persistent routes on dead interfaces..." -ForegroundColor Yellow
# Re-read persistent routes after Step 2 cleanup
$allPersistent = Get-NetRoute -PolicyStore PersistentStore -AddressFamily IPv4 -ErrorAction SilentlyContinue
# Collect interface indexes that are actually present on this machine (including hidden adapters like loopback)
$activeIndexes = @(Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | ForEach-Object { $_.InterfaceIndex })
if ($activeIndexes.Count -eq 0) {
    Write-Host "  WARNING: Could not enumerate active adapters; skipping stale route sweep." -ForegroundColor Yellow
} else {
    $staleCount = 0
    foreach ($route in $allPersistent) {
        # If the route's interface doesn't exist, it's leftover from the source VM's old adapters
        if ($activeIndexes -notcontains $route.InterfaceIndex) {
            Remove-PersistentRoute $route
            Write-Host "  Removed stale: $($route.DestinationPrefix) via $($route.NextHop) on dead IF $($route.InterfaceIndex)" -ForegroundColor Red
            $staleCount++
        }
    }
    if ($staleCount -eq 0) {
        Write-Host "  No stale routes found." -ForegroundColor Green
    } else {
        Write-Host "  Removed $staleCount stale route(s)." -ForegroundColor Green
    }
}

# Step 4: Configure default gateways at NIC/IP configuration level via registry
Write-Host "Configuring default gateways at interface level (Registry)..." -ForegroundColor Cyan

# Get current default gateways after cleanup
$currentDefaultGateways = Get-NetRoute -PolicyStore PersistentStore -AddressFamily IPv4 | Where-Object { $_.DestinationPrefix -eq "0.0.0.0/0" }
Write-Host "Configuring $($currentDefaultGateways.Count) remaining default gateway(s)..." -ForegroundColor Cyan

foreach ($gateway in $currentDefaultGateways) {
    $nextHop = $gateway.NextHop
    $interfaceIndex = $gateway.InterfaceIndex
    $metric = $gateway.RouteMetric

    # Check if interface still exists
    $interface = Get-NetAdapter | Where-Object { $_.InterfaceIndex -eq $interfaceIndex } -ErrorAction SilentlyContinue
    if (-not $interface) {
        Write-Host "  Skipping Interface $interfaceIndex - Interface no longer exists" -ForegroundColor Gray
        continue
    }

    $interfaceAlias = $interface.Name
    $guid = $interface.InterfaceGuid
    Write-Host "  Processing Interface $interfaceIndex ($interfaceAlias) - Gateway $nextHop" -ForegroundColor Yellow

    $regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\$guid"
    if (-not (Test-Path $regPath)) {
        Write-Host "    Registry path not found: $regPath" -ForegroundColor Red
        continue
    }

    $regGateways = (Get-ItemProperty -Path $regPath -Name "DefaultGateway" -ErrorAction SilentlyContinue).DefaultGateway
    $gwList = @()
    if ($null -ne $regGateways) {
        $gwList = @($regGateways)
    }
    if ($gwList -contains $nextHop) {
        Write-Host "    Interface already has gateway $nextHop in registry DefaultGateway, removing from PersistentRoutes only" -ForegroundColor Green
        $persistKey = "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\PersistentRoutes"
        if (Test-Path $persistKey) {
            $routeKeyPrefix = "0.0.0.0,0.0.0.0,$nextHop,"
            $props = Get-Item -Path $persistKey | Select-Object -ExpandProperty Property
            foreach ($p in $props) {
                if ($p.StartsWith($routeKeyPrefix)) {
                    try {
                        Remove-ItemProperty -Path $persistKey -Name $p -ErrorAction Stop
                        Write-Host "    Removed PersistentRoutes entry: $p" -ForegroundColor Green
                    } catch {
                        Write-Host "    Failed to remove PersistentRoutes entry '$p': $($_.Exception.Message)" -ForegroundColor Red
                    }
                }
            }
        }
        continue
    }

    $merged = ($gwList + @($nextHop)) | Select-Object -Unique
    Write-Host "    Setting DefaultGateway=$($merged -join ',') in registry" -ForegroundColor Yellow
    Set-ItemProperty -Path $regPath -Name "DefaultGateway" -Value $merged -Type MultiString
    Write-Host "    [OK] Gateway written to registry for $interfaceAlias" -ForegroundColor Green

}

Write-Host "Persistent route cleanup completed!" -ForegroundColor Green
