# Use cmd /c to avoid PowerShell Constrained Language Mode restrictions
# that block New-Object for non-core types and static method invocation.
cmd /c "echo CONVERSION_DONE>\\.\COM1" 2>&1 | Out-Null
