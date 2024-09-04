@echo off

set firstboot=C:\Program Files\Guestfs\Firstboot

set scripts=%firstboot%\scripts
set scripts_done=%firstboot%\scripts-done
echo Running MTV first boot scripts
for %%f in ("%scripts%"\*.ps1) do (
  echo running "%%f"
  PowerShell -NoProfile -ExecutionPolicy Bypass -File "%%~f"
  set elvl=!errorlevel!
  echo .... exit code !elvl!
  move "%%f" "%scripts_done%"
)