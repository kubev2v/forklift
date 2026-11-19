@echo off
setlocal enabledelayedexpansion

set firstboot=C:\Program Files\Guestfs\Firstboot

set scripts=%firstboot%\scripts
set scripts_done=%firstboot%\scripts-done

rem The firstboot chain is 32-bit, so a bare PowerShell runs 32-bit with
rem WOW64-redirected registry views. Invoke the native 64-bit one instead.
set ps=%SystemRoot%\Sysnative\WindowsPowerShell\v1.0\powershell.exe
if not exist "%ps%" set ps=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe

echo Running MTV first boot scripts
for %%f in ("%scripts%"\*.ps1) do (
  echo running "%%f"
  "%ps%" -NoProfile -ExecutionPolicy Bypass -File "%%~f"
  set elvl=!errorlevel!
  echo .... exit code !elvl!
  move "%%f" "%scripts_done%"
)