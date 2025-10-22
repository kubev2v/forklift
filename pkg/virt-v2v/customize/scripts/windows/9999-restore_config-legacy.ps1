# Migration - Reconfigure disks
# A legacy version of the script that does not use Get-Disk

Write-Host "`nRe-enabling all offline drives`n"

# Find VirtIO disks and their disk numbers
$virtioDisks = Get-WmiObject -Class Win32_DiskDrive | Where-Object {
    $_.Model -like '*VirtIO*' -or $_.Caption -like '*VirtIO*'
}

if ($virtioDisks) {
    foreach ($disk in $virtioDisks) {
        Write-Host "Processing disk $($disk.Index): $($disk.Model)`n"

        # Prepare diskpart script content
        $diskpartScriptContent = @"
select disk $($disk.Index)
online disk
attributes disk clear readonly
exit
"@

        # Write to temporary file
        $tempScriptPath = Join-Path $env:TEMP "diskpart_script_$($disk.Index).txt"
        $diskpartScriptContent | Out-File $tempScriptPath -Encoding ASCII

        try {
            $diskpartOutput = & diskpart /s $tempScriptPath 2>&1
            $outputString = $diskpartOutput -join "`n"

            if ($outputString -match "error" -and $outputString -notmatch "already online") {
                Write-Warning "  - DiskPart reported error. Output:`n$outputString`n"
            } else {
                Write-Host "  - Successfully processed disk $($disk.Index).`n"
            }
        } catch {
            Write-Error "  - Error running diskpart: $($_.Exception.Message)`n"
        } finally {
            # Clean up temporary file
            Remove-Item $tempScriptPath -Force -ErrorAction SilentlyContinue
        }
    }
} else {
    Write-Warning "No VirtIO disks found.`n"
}
