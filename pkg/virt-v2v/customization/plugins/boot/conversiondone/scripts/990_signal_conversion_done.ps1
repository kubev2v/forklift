$port = New-Object System.IO.Ports.SerialPort "COM1", 9600, "None", 8, "One"
try {
    $port.Open()
    $port.WriteLine("CONVERSION_DONE")
} catch {
    if ($port.IsOpen) { try { $port.Close() } catch {} }
    [System.IO.File]::WriteAllText("\\.\COM1", "CONVERSION_DONE`r`n")
} finally {
    if ($port.IsOpen) { try { $port.Close() } catch {} }
}
