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

# Step 1: Preserve ALL default gateways (0.0.0.0/0) with their interface indexes
$defaultGateways = $routes | Where-Object { $_.DestinationPrefix -eq "0.0.0.0/0" }
Write-Host "Found $($defaultGateways.Count) default gateway(s) to preserve:" -ForegroundColor Cyan
foreach ($gw in $defaultGateways) {
    Write-Host "  Gateway: $($gw.NextHop) on Interface $($gw.InterfaceIndex) with metric $($gw.RouteMetric)" -ForegroundColor Cyan
}

# Step 2: Clean up duplicate routes (including default gateways)
Write-Host "Analyzing routes for duplicates..." -ForegroundColor Yellow

# Group ALL routes for duplicate detection  
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
            try {
                Remove-NetRoute -DestinationPrefix $route.DestinationPrefix -NextHop $route.NextHop -InterfaceIndex $route.InterfaceIndex -PolicyStore PersistentStore -Confirm:$false -ErrorAction Stop
                Write-Host "    Deleted: $($route.DestinationPrefix) via $($route.NextHop) on IF $($route.InterfaceIndex)" -ForegroundColor Red
            } catch {
                Write-Host "      Failed to delete: $($_.Exception.Message)" -ForegroundColor Red
            }
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
                $metricStr = if ($metric -is [int]) { "METRIC $metric" } else { "" }
                $command = "route -p ADD $network MASK $netmask $gateway IF $($toKeep.InterfaceIndex) $metricStr"
                Write-Host "      Trying route.exe: $command" -ForegroundColor Gray
                cmd /c $command
                
                Write-Host "    Re-added with route.exe: $dest via $gateway on IF $($toKeep.InterfaceIndex)" -ForegroundColor Green
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

# Step 3: Configure default gateways at NIC/IP configuration level
Write-Host "Configuring default gateways at interface level..." -ForegroundColor Cyan

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
    Write-Host "  Processing Interface $interfaceIndex ($interfaceAlias) - Gateway $nextHop" -ForegroundColor Yellow

    $ipConfig = Get-NetIPConfiguration -InterfaceIndex $interfaceIndex -ErrorAction SilentlyContinue
    $hasGatewayInConfig = $false
    if ($ipConfig -and $ipConfig.IPv4DefaultGateway) {
        $hasGatewayInConfig = $ipConfig.IPv4DefaultGateway.NextHop -contains $nextHop
    }

    if (-not $hasGatewayInConfig) {
        Write-Host "    Interface IP config missing gateway, configuring..." -ForegroundColor Yellow

        $currentIP = Get-NetIPAddress -InterfaceIndex $interfaceIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -ne "127.0.0.1" -and -not $_.IPAddress.StartsWith("169.254") }

        if ($currentIP) {
            $ipAddress = $currentIP.IPAddress
            $prefixLength = $currentIP.PrefixLength

            Write-Host "    Current IP: $ipAddress/$prefixLength, setting gateway: $nextHop" -ForegroundColor Yellow

            try {
                $netshCmd = "netsh interface ipv4 set address name=`"$interfaceAlias`" static $ipAddress $(Convert-PrefixToMask $prefixLength) $nextHop $metric"
                Write-Host "    Executing: $netshCmd" -ForegroundColor Gray
                $result = cmd /c $netshCmd 2>&1
                Write-Host "    Configured NIC gateway with netsh: $nextHop on $interfaceAlias" -ForegroundColor Green
            } catch {
                Write-Host "    Method 1 failed, trying PowerShell method..." -ForegroundColor Yellow
                try {
                    Remove-NetIPAddress -InterfaceIndex $interfaceIndex -AddressFamily IPv4 -Confirm:$false -ErrorAction SilentlyContinue
                    New-NetIPAddress -InterfaceIndex $interfaceIndex -IPAddress $ipAddress -PrefixLength $prefixLength -DefaultGateway $nextHop -ErrorAction Stop
                    Write-Host "    Reconfigured IP with gateway: $ipAddress -> $nextHop" -ForegroundColor Green
                } catch {
                    Write-Host "    All methods failed for interface gateway configuration: $($_.Exception.Message)" -ForegroundColor Red
                    Write-Host "    Manual intervention may be required for interface $interfaceAlias" -ForegroundColor Red
                }
            }
        } else {
            Write-Host "    Could not determine current IP address for interface $interfaceIndex" -ForegroundColor Red
        }
    } else {
        Write-Host "    Interface IP config has gateway: $nextHop" -ForegroundColor Green
    }
}

Write-Host "Persistent route cleanup completed!" -ForegroundColor Green
