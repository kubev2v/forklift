# Migration - Reconfigure disks
# Initialize the log file created by the generated script
# Bring all offline disks online (OS disk is usually already online; non-OS disks may be offline after migration)

Write-Host 'Re-enabling all offline drives'
$offlineDisks = Get-Disk | Where { $_.IsOffline -eq $true }
if ($offlineDisks) {
    $offlineDisks | ForEach-Object {
        Write-Host ('  - ' + $_.Number + ': ' + $_.FriendlyName + ' (' + [math]::Round($_.Size/1GB,2) + 'GB)')
        $_ | Set-Disk -IsOffline $false
        $_ | Set-Disk -IsReadOnly $false
    }
} else {
    Write-Host '  No offline disks found.'
}
