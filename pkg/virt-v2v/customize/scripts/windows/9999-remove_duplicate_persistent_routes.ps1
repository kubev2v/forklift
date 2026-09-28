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
# route.exe only supports IPv4, IPv6 routes rely on Remove-NetRoute alone.
function Remove-PersistentRoute($route) {
    try {
        Remove-NetRoute -DestinationPrefix $route.DestinationPrefix -NextHop $route.NextHop `
            -InterfaceIndex $route.InterfaceIndex -PolicyStore PersistentStore -Confirm:$false -ErrorAction Stop
    } catch {
        Write-Host "      Remove-NetRoute failed: $($_.Exception.Message)" -ForegroundColor Red
    }
    # route.exe fallback is IPv4 only
    if ($route.AddressFamily -eq "IPv4") {
        $parts = $route.DestinationPrefix.Split("/")
        $prefixLen = [int]$parts[1]
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

# Step 1: Preserve ALL default gateways (0.0.0.0/0 and ::/0) with their interface indexes
$defaultGateways = $routes | Where-Object { $_.DestinationPrefix -eq "0.0.0.0/0" -or $_.DestinationPrefix -eq "::/0" }
Write-Host "Found $($defaultGateways.Count) default gateway(s) to preserve:" -ForegroundColor Cyan
foreach ($gw in $defaultGateways) {
    Write-Host "  Gateway: $($gw.NextHop) on Interface $($gw.InterfaceIndex) with metric $($gw.RouteMetric)" -ForegroundColor Cyan
}

# Step 2: Clean up duplicate routes (including default gateways)
Write-Host "Analyzing routes for duplicates..." -ForegroundColor Yellow

# Group ALL routes for duplicate detection
$groupedRoutes = $routes | Group-Object {
    "$($_.DestinationPrefix)-$($_.NextHop)"
} | Where-Object { $_.Count -gt 1 }

# Separate default gateway duplicates from other duplicates
$gatewayDuplicates = $groupedRoutes | Where-Object {
    $n = $_.Name.Trim()
    $n.StartsWith("0.0.0.0/0-") -or $n.StartsWith("::/0-")
}
$nonGatewayDuplicates = $groupedRoutes | Where-Object {
    $n = $_.Name.Trim()
    -not ($n.StartsWith("0.0.0.0/0-") -or $n.StartsWith("::/0-"))
}

Write-Host "Found $($gatewayDuplicates.Count) duplicate default gateway groups" -ForegroundColor Cyan
Write-Host "Found $($nonGatewayDuplicates.Count) duplicate non-gateway route groups" -ForegroundColor Cyan

if (-not $groupedRoutes) {
    Write-Host "No duplicate persistent routes found." -ForegroundColor Green
} else {
    Write-Host "Cleaning up duplicate persistent routes..." -ForegroundColor Yellow
    $liveAdapters = Get-NetAdapter
    
    # First, handle duplicate default gateways
    foreach ($group in $gatewayDuplicates) {
        $dupRoutes = $group.Group
        
        # Choose the route on a live interface with the lowest metric
        $sortedRoutes = $dupRoutes | Sort-Object RouteMetric
        $toKeep = $null
        foreach ($route in $sortedRoutes) {
            $interface = $liveAdapters | Where-Object { $_.InterfaceIndex -eq $route.InterfaceIndex }
            if ($interface) {
                $toKeep = $route
                break
            }
        }
        
        # If no existing interface found, just use the lowest-metric one
        if (-not $toKeep) {
            $toKeep = $sortedRoutes[0]
        }
        
        $dest = $toKeep.DestinationPrefix
        $gateway = $toKeep.NextHop
        $metric = $toKeep.RouteMetric
        
        Write-Host "  Cleaning duplicate default gateway: $gateway (metric $metric) - keeping IF $($toKeep.InterfaceIndex)" -ForegroundColor Yellow
        
        # Remove ALL instances
        foreach ($route in $dupRoutes) {
            Remove-PersistentRoute $route
            Write-Host "    Deleted: $($route.DestinationPrefix) via $($route.NextHop) on IF $($route.InterfaceIndex)" -ForegroundColor Red
        }
        
        # After deleting all duplicates, verify if a persistent route still
        # exists for this destination. We must check the PersistentStore directly
        # rather than Get-NetIPConfiguration, which reflects active (non-persistent)
        # routes and would falsely indicate the gateway exists.
        $existingPersistent = Get-NetRoute -DestinationPrefix $dest -InterfaceIndex $toKeep.InterfaceIndex `
            -PolicyStore PersistentStore -ErrorAction SilentlyContinue | Where-Object { $_.NextHop -eq $gateway }

        if ($existingPersistent) {
            Write-Host "    Skipping re-add: persistent route already exists for $dest via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
        } else {
            # Re-add only ONE instance (preserve the first interface)
            $reAddSucceeded = $false
            try {
                New-NetRoute -DestinationPrefix $dest -InterfaceIndex $toKeep.InterfaceIndex -NextHop $gateway -RouteMetric $metric -PolicyStore PersistentStore -ErrorAction Stop
                Write-Host "    Re-added: $dest via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
                $reAddSucceeded = $true
            } catch {
                Write-Host "      PolicyStore method failed: $($_.Exception.Message)" -ForegroundColor Yellow
            }
            
            # Fallback to route.exe if PolicyStore failed (IPv4 only)
            if (-not $reAddSucceeded -and $dest -eq "0.0.0.0/0") {
                try {
                    $metricStr = if ($null -ne $metric) { "METRIC $([int]$metric)" } else { "" }
                    $command = "route -p ADD 0.0.0.0 MASK 0.0.0.0 $gateway IF $($toKeep.InterfaceIndex) $metricStr"
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

            # Fallback for IPv6: use netsh or New-NetRoute without PolicyStore
            if (-not $reAddSucceeded -and $dest -eq "::/0") {
                try {
                    $metricVal = if ($null -ne $metric) { [int]$metric } else { 256 }
                    # First try "set route" (modifies existing active route to be persistent)
                    $netshCmd = "netsh interface ipv6 set route ::/0 interface=$($toKeep.InterfaceIndex) nexthop=$gateway metric=$metricVal store=persistent publish=Yes"
                    Write-Host "      Trying netsh set: $netshCmd" -ForegroundColor Gray
                    cmd /c $netshCmd 2>&1
                    if ($LASTEXITCODE -ne 0) {
                        # "set route" failed, try removing active then re-adding fresh
                        Write-Host "      netsh set failed, removing active route and re-adding..." -ForegroundColor Yellow
                        cmd /c "netsh interface ipv6 delete route ::/0 interface=$($toKeep.InterfaceIndex) nexthop=$gateway" 2>&1
                        $addCmd = "netsh interface ipv6 add route ::/0 interface=$($toKeep.InterfaceIndex) nexthop=$gateway metric=$metricVal store=persistent publish=Yes"
                        Write-Host "      Trying netsh add: $addCmd" -ForegroundColor Gray
                        cmd /c $addCmd 2>&1
                        if ($LASTEXITCODE -ne 0) {
                            # Last resort: New-NetRoute without PolicyStore
                            New-NetRoute -DestinationPrefix "::/0" -InterfaceIndex $toKeep.InterfaceIndex -NextHop $gateway -RouteMetric $metricVal -ErrorAction Stop
                            Write-Host "    Re-added with New-NetRoute (active): ::/0 via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
                        } else {
                            Write-Host "    Re-added with netsh add: ::/0 via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
                        }
                    } else {
                        Write-Host "    Made persistent with netsh set: ::/0 via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
                    }
                } catch {
                    Write-Host "      IPv6 route re-add failed: $($_.Exception.Message)" -ForegroundColor Red
                }
            }
        }
    }
    
    # Then handle non-gateway duplicates
    foreach ($group in $nonGatewayDuplicates) {
        # Prefer route on a live interface with the lowest metric
        $sorted = $group.Group | Sort-Object RouteMetric
        $toKeep = $null
        foreach ($candidate in $sorted) {
            $iface = $liveAdapters | Where-Object { $_.InterfaceIndex -eq $candidate.InterfaceIndex }
            if ($iface) {
                $toKeep = $candidate
                break
            }
        }
        if (-not $toKeep) {
            $toKeep = $sorted[0]
        }
        $dest = $toKeep.DestinationPrefix
        $gateway = $toKeep.NextHop
        $metric = $toKeep.RouteMetric
        $interfaceIndex = $toKeep.InterfaceIndex
        $isIPv4 = $toKeep.AddressFamily -eq "IPv4"

        $parts = $dest.Split("/")
        if ($parts.Count -ne 2) {
            Write-Host "  Invalid destination prefix format: $dest" -ForegroundColor Red
            continue
        }

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

        # Fallback to route.exe if New-NetRoute failed (IPv4 only)
        if (-not $reAddSucceeded -and $isIPv4) {
            $network = $parts[0]
            $prefix = [int]$parts[1]
            $netmask = Convert-PrefixToMask $prefix
            $metricStr = if ($null -ne $metric) { "METRIC $([int]$metric)" } else { "" }
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
# Re-read persistent routes after Step 2 cleanup (both IPv4 and IPv6)
$allPersistent = Get-NetRoute -PolicyStore PersistentStore -ErrorAction SilentlyContinue
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

# Step 4: Configure default gateways at NIC/IP configuration level
Write-Host "Configuring default gateways at interface level..." -ForegroundColor Cyan

# Get current default gateways after cleanup (both IPv4 and IPv6)
$currentDefaultGateways = Get-NetRoute -PolicyStore PersistentStore | Where-Object {
    $_.DestinationPrefix -eq "0.0.0.0/0" -or $_.DestinationPrefix -eq "::/0"
}
Write-Host "Configuring $($currentDefaultGateways.Count) remaining default gateway(s)..." -ForegroundColor Cyan

foreach ($gateway in $currentDefaultGateways) {
    $nextHop = $gateway.NextHop
    $interfaceIndex = $gateway.InterfaceIndex
    $metric = $gateway.RouteMetric
    $isIPv4 = $gateway.AddressFamily -eq "IPv4"
    $family = if ($isIPv4) { "IPv4" } else { "IPv6" }

    # Check if interface still exists
    $interface = Get-NetAdapter | Where-Object { $_.InterfaceIndex -eq $interfaceIndex } -ErrorAction SilentlyContinue
    if (-not $interface) {
        Write-Host "  Skipping Interface $interfaceIndex - Interface no longer exists" -ForegroundColor Gray
        continue
    }
    
    $interfaceAlias = $interface.Name
    Write-Host "  Processing Interface $interfaceIndex ($interfaceAlias) - $family Gateway $nextHop" -ForegroundColor Yellow

    $ipConfig = Get-NetIPConfiguration -InterfaceIndex $interfaceIndex -ErrorAction SilentlyContinue
    $hasGatewayInConfig = $false
    if ($ipConfig) {
        if ($isIPv4 -and $ipConfig.IPv4DefaultGateway) {
            $hasGatewayInConfig = $ipConfig.IPv4DefaultGateway.NextHop -contains $nextHop
        } elseif (-not $isIPv4 -and $ipConfig.IPv6DefaultGateway) {
            $hasGatewayInConfig = $ipConfig.IPv6DefaultGateway.NextHop -contains $nextHop
        }
    }

    if (-not $hasGatewayInConfig) {
        Write-Host "    Interface IP config missing $family gateway, configuring..." -ForegroundColor Yellow

        if ($isIPv4) {
            $currentIP = Get-NetIPAddress -InterfaceIndex $interfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -ne "127.0.0.1" -and -not $_.IPAddress.StartsWith("169.254") }

            if ($currentIP) {
                $ipAddress = $currentIP.IPAddress
                $prefixLength = $currentIP.PrefixLength

                Write-Host "    Current IP: $ipAddress/$prefixLength, setting gateway: $nextHop" -ForegroundColor Yellow

                $netshCmd = "netsh interface ipv4 set address name=`"$interfaceAlias`" static $ipAddress $(Convert-PrefixToMask $prefixLength) $nextHop $metric"
                Write-Host "    Executing: $netshCmd" -ForegroundColor Gray
                $result = cmd /c $netshCmd 2>&1
                if ($LASTEXITCODE -eq 0) {
                    Write-Host "    Configured NIC gateway with netsh: $nextHop on $interfaceAlias" -ForegroundColor Green
                } else {
                    Write-Host "    netsh failed (exit $LASTEXITCODE): $result, trying PowerShell method..." -ForegroundColor Yellow
                    try {
                        Remove-NetIPAddress -InterfaceIndex $interfaceIndex -AddressFamily IPv4 -Confirm:$false -ErrorAction SilentlyContinue
                        New-NetIPAddress -InterfaceIndex $interfaceIndex -IPAddress $ipAddress -PrefixLength $prefixLength -DefaultGateway $nextHop -ErrorAction Stop
                        Write-Host "    Reconfigured IP with gateway: $ipAddress -> $nextHop" -ForegroundColor Green
                    } catch {
                        Write-Host "    All methods failed for IPv4 gateway configuration: $($_.Exception.Message)" -ForegroundColor Red
                    }
                }
            } else {
                Write-Host "    Could not determine current IPv4 address for interface $interfaceIndex" -ForegroundColor Red
            }
        } else {
            # IPv6: use New-NetRoute directly (no netsh/route.exe for IPv6 gateways)
            try {
                New-NetRoute -DestinationPrefix "::/0" -InterfaceIndex $interfaceIndex -NextHop $nextHop -RouteMetric $metric -ErrorAction Stop
                Write-Host "    Configured IPv6 gateway: $nextHop on $interfaceAlias" -ForegroundColor Green
            } catch {
                Write-Host "    Failed to configure IPv6 gateway: $($_.Exception.Message)" -ForegroundColor Red
            }
        }
    } else {
        Write-Host "    Interface IP config has $family gateway: $nextHop" -ForegroundColor Green
    }
}

Write-Host "Persistent route cleanup completed!" -ForegroundColor Green
