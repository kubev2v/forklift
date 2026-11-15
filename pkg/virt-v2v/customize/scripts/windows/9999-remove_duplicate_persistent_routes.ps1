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

# CLM-safe check: is an IPv4 next-hop within a given subnet?
function Test-IPv4OnLink($nextHop, $ifAddress, $prefixLength) {
    $hopParts = $nextHop.Split('.')
    $addrParts = $ifAddress.Split('.')
    if ($hopParts.Count -ne 4 -or $addrParts.Count -ne 4) { return $false }
    $maskParts = (Convert-PrefixToMask $prefixLength).Split('.')
    for ($i = 0; $i -lt 4; $i++) {
        if (([int]$hopParts[$i] -band [int]$maskParts[$i]) -ne `
            ([int]$addrParts[$i] -band [int]$maskParts[$i])) {
            return $false
        }
    }
    return $true
}

# IPv4 only. Match NextHop, or destination when NextHop is 0.0.0.0 (on-link).
# Returns @{ Index: LookupFailed } — any failed lookup with no match => LookupFailed.
# Only Status=Up adapters (disconnected source NICs often still have stale IPs).
function Find-LiveInterfaceForNextHop($route, $liveAdapters) {
    if ($route.AddressFamily -ne "IPv4") {
        return @{ Index = $null; LookupFailed = $false }
    }
    $hop = $route.NextHop
    if ($hop -eq "0.0.0.0") {
        $hop = $route.DestinationPrefix.Split("/")[0]
    }
    $lookupFailed = $false
    foreach ($adapter in $liveAdapters) {
        if ($adapter.Status -ne "Up") { continue }
        try {
            $addresses = Get-NetIPAddress -InterfaceIndex $adapter.InterfaceIndex `
                -AddressFamily IPv4 -ErrorAction Stop
        } catch {
            $lookupFailed = $true
            continue
        }
        foreach ($addr in $addresses) {
            if ($addr.IPAddress -eq "127.0.0.1" -or $addr.IPAddress.StartsWith("169.254")) { continue }
            if (Test-IPv4OnLink $hop $addr.IPAddress $addr.PrefixLength) {
                return @{ Index = $adapter.InterfaceIndex; LookupFailed = $false }
            }
        }
    }
    return @{ Index = $null; LookupFailed = $lookupFailed }
}

# Add a persistent route on the given interface, with route.exe fallback for IPv4.
# Returns $true on success.
function Add-PersistentRouteToInterface($dest, $nextHop, $metric, $interfaceIndex, $isIPv4) {
    $ok = $false
    try {
        $null = New-NetRoute -DestinationPrefix $dest -InterfaceIndex $interfaceIndex `
            -NextHop $nextHop -RouteMetric $metric -PolicyStore PersistentStore -ErrorAction Stop
        $ok = $true
    } catch {
        Write-Host "      New-NetRoute failed: $($_.Exception.Message)" -ForegroundColor Yellow
    }
    if (-not $ok -and $isIPv4) {
        $parts = $dest.Split("/")
        $network = $parts[0]
        $netmask = Convert-PrefixToMask ([int]$parts[1])
        # Windows requires METRIC before IF
        $metricStr = if ($null -ne $metric) { " METRIC $([int]$metric)" } else { "" }
        $command = "route -p ADD $network MASK $netmask $nextHop$metricStr IF $interfaceIndex"
        $result = cmd /c $command 2>&1
        if ($LASTEXITCODE -eq 0) {
            $ok = $true
        } else {
            Write-Host "      route.exe add failed (exit $LASTEXITCODE): $result" -ForegroundColor Yellow
        }
    }
    return $ok
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
                    # Windows requires METRIC before IF
                    $metricStr = if ($null -ne $metric) { " METRIC $([int]$metric)" } else { "" }
                    $command = "route -p ADD 0.0.0.0 MASK 0.0.0.0 $gateway$metricStr IF $($toKeep.InterfaceIndex)"
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
            # Windows requires METRIC before IF
            $metricStr = if ($null -ne $metric) { " METRIC $([int]$metric)" } else { "" }
            $command = "route -p ADD $network MASK $netmask $gateway$metricStr IF $interfaceIndex"

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

# Step 3: rebind routes on dead IF indexes to a live NIC if the
# next-hop is on-link, remove orphans. Skip unbound (IF <= 0).
Write-Host "Reconciling persistent routes on obsolete interface indexes..." -ForegroundColor Yellow
# Re-read persistent routes after Step 2 cleanup (both IPv4 and IPv6)
$allPersistent = Get-NetRoute -PolicyStore PersistentStore -ErrorAction SilentlyContinue
# Collect interface indexes that are actually present on this machine (including hidden adapters like loopback)
$activeIndexes = @(Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | ForEach-Object { $_.InterfaceIndex })
if ($activeIndexes.Count -eq 0) {
    Write-Host "  WARNING: Could not enumerate active adapters; skipping stale route reconciliation." -ForegroundColor Yellow
} else {
    $liveAdaptersStep3 = Get-NetAdapter
    $reboundCount = 0
    $removedCount = 0
    $failedCount = 0

    foreach ($route in $allPersistent) {
        # Unbound routes (IF 0 or negative) are not tied to any adapter — leave them alone
        if ($route.InterfaceIndex -le 0) { continue }

        # Route is on a live adapter — nothing to do
        if ($activeIndexes -contains $route.InterfaceIndex) { continue }

        # --- dead interface index ---
        $dest     = $route.DestinationPrefix
        $nextHop  = $route.NextHop
        $metric   = $route.RouteMetric
        $deadIF   = $route.InterfaceIndex
        $isIPv4   = $route.AddressFamily -eq "IPv4"

        # Check whether an identical dest+nextHop already exists on a live interface
        $liveCopy = Get-NetRoute -DestinationPrefix $dest -PolicyStore PersistentStore `
            -ErrorAction SilentlyContinue |
            Where-Object { $_.NextHop -eq $nextHop -and ($activeIndexes -contains $_.InterfaceIndex) }

        if ($liveCopy) {
            # A live copy exists — this dead-IF entry is a leftover duplicate; just delete it
            Remove-PersistentRoute $route
            $still = Get-NetRoute -DestinationPrefix $dest -InterfaceIndex $deadIF `
                -PolicyStore PersistentStore -ErrorAction SilentlyContinue |
                Where-Object { $_.NextHop -eq $nextHop }
            if (-not $still) {
                Write-Host "  Removed duplicate: $dest via $nextHop on dead IF $deadIF (live copy on IF $($liveCopy[0].InterfaceIndex))" -ForegroundColor Yellow
                $removedCount++
            } else {
                Write-Host "  FAILED to remove duplicate: $dest via $nextHop on dead IF $deadIF" -ForegroundColor Red
                $failedCount++
            }
            continue
        }

        # No live copy — try to rebind to a live adapter whose subnet covers the route
        $find = Find-LiveInterfaceForNextHop $route $liveAdaptersStep3
        if ($find.LookupFailed) {
            Write-Host "  FAILED address lookup (left on dead IF $deadIF): $dest via $nextHop" -ForegroundColor Red
            $failedCount++
            continue
        }
        $targetIF = $find.Index

        if ($null -ne $targetIF) {
            # Rebind: add on live IF first so a failed re-add cannot drop the only copy
            $added = Add-PersistentRouteToInterface $dest $nextHop $metric $targetIF $isIPv4
            if (-not $added) {
                Write-Host "  FAILED to rebind (left on dead IF $deadIF): $dest via $nextHop -> IF $targetIF" -ForegroundColor Red
                $failedCount++
                continue
            }
            Remove-PersistentRoute $route
            $still = Get-NetRoute -DestinationPrefix $dest -InterfaceIndex $deadIF `
                -PolicyStore PersistentStore -ErrorAction SilentlyContinue |
                Where-Object { $_.NextHop -eq $nextHop }
            if (-not $still) {
                Write-Host "  Rebound: $dest via $nextHop from dead IF $deadIF -> live IF $targetIF" -ForegroundColor Green
                $reboundCount++
            } else {
                Write-Host "  Added on IF $targetIF but FAILED to remove dead IF ${deadIF}: $dest via $nextHop" -ForegroundColor Yellow
                $reboundCount++
            }
        } else {
            # True orphan: not on-link on any live adapter — remove
            Remove-PersistentRoute $route
            $still = Get-NetRoute -DestinationPrefix $dest -InterfaceIndex $deadIF `
                -PolicyStore PersistentStore -ErrorAction SilentlyContinue |
                Where-Object { $_.NextHop -eq $nextHop }
            if (-not $still) {
                Write-Host "  Removed orphan: $dest via $nextHop on dead IF $deadIF (next-hop unreachable)" -ForegroundColor Red
                $removedCount++
            } else {
                Write-Host "  FAILED to remove orphan: $dest via $nextHop on dead IF $deadIF" -ForegroundColor Red
                $failedCount++
            }
        }
    }
    Write-Host "  Reconciled: $reboundCount rebound, $removedCount removed, $failedCount failed." -ForegroundColor Cyan
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
