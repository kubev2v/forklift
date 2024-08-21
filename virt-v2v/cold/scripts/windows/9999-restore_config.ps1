# Migration - Reconfigure disks
# Initialize the log file created by the generated script
$logFile = $env:SystemDrive + '\Program Files\Guestfs\Firstboot\scripts-done\9999-restore_config.txt'
Write-Output ('Starting 9999-restore_config.ps1 script') > $logFile
Write-Output ('') >> $logFile

# script section to re-enable all offline drives
# Re-enable all offline drives

# Get all disks that match the filter using Win32_DiskDrive
$disks = Get-WmiObject -Query "SELECT * FROM Win32_DiskDrive WHERE Model LIKE '%VirtIO%'"

# Check if any disks were found
if ($disks.Count -eq 0) {
    Write-Output ("No disks found matching the filter 'VirtIO'.") >> $logFile
} else {
    # Iterate through each matching disk
    foreach ($disk in $disks) {
        $diskNumber = $disk.Index  # Get the disk index which corresponds to the disk number

        # Create a diskpart script to set the disk online
        $diskpartScript = @"
select disk $diskNumber
online disk
attributes disk clear readonly
"@

        # Execute the diskpart script
        $diskpartScript | diskpart

        Write-Output ("Disk $($disk.Model) (Disk Number: $diskNumber) is now online.") >> $logFile
    }
}
