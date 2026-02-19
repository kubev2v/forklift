# Migration - Reconfigure disks
# Legacy version: use diskpart to bring all offline disks online (no Get-Disk dependency)

Write-Host "`nRe-enabling all offline drives`n"

# Get list of disks and their status via diskpart
$listScript = @"
list disk
exit
"@
$tempListPath = Join-Path $env:TEMP "diskpart_list_disks.txt"
$listScript | Out-File $tempListPath -Encoding ASCII
$listResult = & diskpart /s $tempListPath 2>&1 | Out-String
Remove-Item $tempListPath -Force -ErrorAction SilentlyContinue

# Parse "list disk" output: lines like "  Disk 0    Online    ..." or "  Disk 1    Offline   ..."
$diskLines = $listResult -split "`n" | Where-Object { $_ -match '^\s+Disk\s+(\d+)\s+(\w+)' }
$offlineDiskNumbers = @()
foreach ($line in $diskLines) {
    if ($line -match '^\s+Disk\s+(\d+)\s+Offline\s') {
        $offlineDiskNumbers += [int]$Matches[1]
    }
}

if ($offlineDiskNumbers.Count -gt 0) {
    foreach ($diskNum in $offlineDiskNumbers) {
        Write-Host "Processing disk $diskNum (offline)...`n"
        $diskpartScriptContent = @"
select disk $diskNum
online disk
attributes disk clear readonly
exit
"@
        $tempScriptPath = Join-Path $env:TEMP "diskpart_script_$diskNum.txt"
        $diskpartScriptContent | Out-File $tempScriptPath -Encoding ASCII
        try {
            $diskpartOutput = & diskpart /s $tempScriptPath 2>&1
            $outputString = $diskpartOutput -join "`n"
            if ($outputString -match "error" -and $outputString -notmatch "already online") {
                Write-Warning "  - DiskPart reported error. Output:`n$outputString`n"
            } else {
                Write-Host "  - Successfully processed disk $diskNum.`n"
            }
        } catch {
            Write-Error "  - Error running diskpart: $($_.Exception.Message)`n"
        } finally {
            Remove-Item $tempScriptPath -Force -ErrorAction SilentlyContinue
        }
    }
} else {
    Write-Host "No offline disks found.`n"
}
