# Migration - Reconfigure disks
# Initialize the log file created by the generated script

# Re-enable all offline drives
Write-Host 'Re-enabling all offline drives'
Get-Disk | Where { $_.FriendlyName -like '*VirtIO*' } | % {
    Write-Host ('  - ' + $_.Number + ': ' + $_.FriendlyName + '(' + [math]::Round($_.Size/1GB,2) + 'GB)')
    $_ | Set-Disk -IsOffline $false
    $_ | Set-Disk -IsReadOnly $false
}
