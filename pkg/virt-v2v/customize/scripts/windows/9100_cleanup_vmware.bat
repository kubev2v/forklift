@echo off
:: ===============================================================
:: 9100_cleanup_vmware.bat
:: Complete VMware cleanup: drivers, driver packages, services,
:: and registry entries.
:: Run as Administrator.
:: ===============================================================

setlocal enabledelayedexpansion

:: --- Script directory and log ---
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"
set "LOG=%SCRIPT_DIR%\cleanup_vmware.log"
echo =============================================================== > "%LOG%"
echo   VMware Cleanup Log - %DATE% %TIME% >> "%LOG%"
echo =============================================================== >> "%LOG%"

:: --- Locate devcon.exe (try fixed path, local arch copy, then PATH) ---
set "DEVCON="
set "HAS_DEVCON=0"

if exist "C:\Windows\Build\Tools\devcon.exe" (
    set "DEVCON=C:\Windows\Build\Tools\devcon.exe"
    set "HAS_DEVCON=1"
    goto :DevconFound
)

set "ARCH="
if defined PROCESSOR_ARCHITEW6432 (
    set "ARCH=64"
) else (
    if "%PROCESSOR_ARCHITECTURE%"=="AMD64" (
        set "ARCH=64"
    ) else (
        set "ARCH=32"
    )
)
if "%ARCH%"=="64" (
    if exist "%SCRIPT_DIR%\x64\devcon.exe" (
        set "DEVCON=%SCRIPT_DIR%\x64\devcon.exe"
        set "HAS_DEVCON=1"
        goto :DevconFound
    )
) else (
    if exist "%SCRIPT_DIR%\x86\devcon.exe" (
        set "DEVCON=%SCRIPT_DIR%\x86\devcon.exe"
        set "HAS_DEVCON=1"
        goto :DevconFound
    )
)

where devcon.exe >nul 2>&1
if %errorlevel% equ 0 (
    for /f "delims=" %%P in ('where devcon.exe') do (
        set "DEVCON=%%P"
        set "HAS_DEVCON=1"
        goto :DevconFound
    )
)

:DevconFound

echo.
echo ===============================================================
echo   VMware Full Cleanup Script
echo ===============================================================
echo.
echo [INFO] Log file: %LOG%
if "%HAS_DEVCON%"=="1" (
    echo [INFO] devcon:   %DEVCON%
) else (
    echo [WARNING] devcon.exe not found - driver device removal will be skipped
    echo [WARNING] devcon.exe not found >> "%LOG%"
)
echo.

:: ===============================================================
:: PHASE 1: Disable and remove VMware PnP devices (devcon)
:: ===============================================================
echo.
echo ===============================================================
echo  PHASE 1: VMware PnP Devices
echo ===============================================================
echo.

if "%HAS_DEVCON%"=="0" (
    echo [SKIP] devcon.exe not available, skipping PnP device removal
    echo [SKIP] devcon.exe not available >> "%LOG%"
    goto :Phase2
)

echo Searching for VMware devices...
"%DEVCON%" findall * | findstr /I "VMware" > "%temp%\vmware_devices.txt"

if %errorlevel% neq 0 (
    echo No VMware devices found.
    echo [INFO] No VMware devices found. >> "%LOG%"
    del "%temp%\vmware_devices.txt" >nul 2>&1
    goto :Phase2
)

echo VMware devices found:
type "%temp%\vmware_devices.txt"
echo.

for /f "usebackq delims=" %%L in ("%temp%\vmware_devices.txt") do (
    set "LINE=%%L"
    for /f "tokens=1,* delims=:" %%A in ("!LINE!") do (
        set "DEVID=%%A"
        set "DEVDESC=%%B"
        call :DisableAndRemoveDevice
    )
)

del "%temp%\vmware_devices.txt" >nul 2>&1
echo [INFO] PnP device phase complete. >> "%LOG%"

:: ===============================================================
:: PHASE 2: Remove VMware driver packages (pnputil)
:: ===============================================================
:Phase2
echo.
echo ===============================================================
echo  PHASE 2: VMware Driver Packages
echo ===============================================================
echo.

pnputil /enum-drivers > "%temp%\all_drivers.txt"
findstr /i /c:"Published Name" /c:"Provider Name" "%temp%\all_drivers.txt" > "%temp%\vmware_drivers.txt"

set DRV_COUNT=0
set LAST_PUBLISHED=

for /f "tokens=1,* delims=:" %%A in (%temp%\vmware_drivers.txt) do (
    set LINE=%%A
    set VALUE=%%B
    set VALUE=!VALUE: =!

    if /i "!LINE!"=="Published Name" (
        set LAST_PUBLISHED=!VALUE!
    )

    if /i "!LINE!"=="Provider Name" (
        echo !VALUE! | findstr /i "VMware" >nul
        if !errorlevel! == 0 (
            if not "!LAST_PUBLISHED!"=="" (
                echo Found VMware INF: !LAST_PUBLISHED!
                set INF_LIST[!DRV_COUNT!]=!LAST_PUBLISHED!
                set /a DRV_COUNT+=1
            )
        )
        set LAST_PUBLISHED=
    )
)

if %DRV_COUNT% EQU 0 (
    echo No VMware driver packages found.
    echo [INFO] No VMware driver packages found. >> "%LOG%"
    goto :Phase3
)

echo.
echo Removing %DRV_COUNT% VMware driver packages...
set "REBOOT_REQUIRED=0"
for /l %%I in (0,1,%DRV_COUNT%-1) do (
    set INF=!INF_LIST[%%I]!
    if not "!INF!"=="" (
        echo Removing !INF! ...
        pnputil /delete-driver "!INF!" /uninstall /force > "%temp%\pnputil_output.txt" 2>&1
        type "%temp%\pnputil_output.txt" >> "%LOG%"
        type "%temp%\pnputil_output.txt" | findstr /i "reboot restart" >nul
        if !errorlevel! == 0 (
            set REBOOT_REQUIRED=1
        )
        echo Done.
    )
)
del "%temp%\pnputil_output.txt" >nul 2>&1

if %REBOOT_REQUIRED% EQU 1 (
    echo [WARNING] A system reboot may be required to complete driver removal.
    echo [WARNING] Reboot required for driver removal. >> "%LOG%"
)

del "%temp%\all_drivers.txt" >nul 2>&1
del "%temp%\vmware_drivers.txt" >nul 2>&1
echo [INFO] Driver package phase complete. >> "%LOG%"

:: ===============================================================
:: PHASE 3: Stop, disable, and delete VMware services
:: ===============================================================
:Phase3
echo.
echo ===============================================================
echo  PHASE 3: VMware Services
echo ===============================================================
echo.

echo [INFO] Enumerating services...
set "TEMPFILE=%temp%\all_services.txt"
sc query type= service state= all > "%TEMPFILE%"

set "VMWARE_LIST="
set /a SVC_MATCH_COUNT=0

for /f "tokens=2 delims=:" %%A in ('findstr /R "^SERVICE_NAME" "%TEMPFILE%"') do (
    set "SVC=%%A"
    set "SVC=!SVC: =!"
    call :CheckVMwareService !SVC!
)

del "%TEMPFILE%" >nul 2>&1

if %SVC_MATCH_COUNT%==0 (
    echo No VMware-related services found.
    echo [INFO] No VMware-related services found. >> "%LOG%"
    goto :Phase4
)

echo [INFO] Found %SVC_MATCH_COUNT% VMware-related services:
for %%S in (!VMWARE_LIST!) do echo   %%S
echo.

for %%S in (!VMWARE_LIST!) do (
    call :StopAndDeleteService "%%S"
)

echo [INFO] Service phase complete. >> "%LOG%"

:: ===============================================================
:: PHASE 4: Remove VMware registry entries
:: ===============================================================
:Phase4
echo.
echo ===============================================================
echo  PHASE 4: VMware Registry Entries
echo ===============================================================
echo.

set /a REG_DELETED=0
set /a REG_ERRORS=0

:: --- StartmenuInternet / Host Open ---
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "IconsVisible"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE" "LocalizedString"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "ReinstallCommand"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "HideIconsCommand"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "ShowIconsCommand"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\DefaultIcon" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\Applications\VMwareHostOpen.exe\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile" ""

:: --- VMwareHostOpen Capabilities ---
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "sftp"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "ftp"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "news"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "http"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "mailto"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "feed"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "https"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "telnet"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "ssh"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".htm"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".shtml"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".xhtml"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".xht"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".html"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities" "ApplicationDescription"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\Startmenu" "StartmenuInternet"
call :DeleteRegistryEntry "HKLM\SOFTWARE\RegisteredApplications" "VMware Host Open"

:: --- Event Log entries ---
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "CategoryMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "CategoryCount"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools" "TypesSupported"

:: --- vmhgfs network provider ---
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "Name"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "ProviderPath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "DeviceName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\parameters" "ServerName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\parameters" "ShareName"

:: --- VMware Tools / VGAuth / Time Provider ---
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware VGAuth" "PreferencesFile"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Tools" "InstallPath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "Enabled"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "InputProvider"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "DllName"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\CLSID\{C73DA087-EDDB-4a7c-B216-8EF8A3B92C7B}\InprocServer32" ""
call :DeleteRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\VMUpgradeHelper" "-"
call :DeleteRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\GuestIntrospection" "-"
call :DeleteRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\GuestStoreUpgrade" "-"

:: --- vmrawdsk parameters ---
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "CommonAppData"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "PrevBootMode"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "CacheDir"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "Windir"

:: --- VMCI ---
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Type"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Start"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "ErrorControl"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Tag"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "ImagePath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "DisplayName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Group"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Owners"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "vwdk.installers"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" "0"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" "Count"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" "NextInstance"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" ""
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" ""

:: --- VMMouse ---
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Type"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Start"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "ErrorControl"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Tag"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "ImagePath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "DisplayName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Group"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Owners"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "vwdk.installers"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" "0"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" "Count"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" "NextInstance"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" ""
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" ""

:: --- VMUsbMouse ---
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Type"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Start"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "ErrorControl"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Tag"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "ImagePath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "DisplayName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Group"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Owners"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "vwdk.installers"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse\Parameters" ""
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" ""

:: --- VMware Software keys ---
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\CbLauncher" "Cb.LauncherVersion"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\CbLauncher" "Cb.InstallStatus"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\CbLauncher" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "VmciHostDevInst"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmci.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmci.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsock.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsock.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsockSys.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsockDll.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "efifw.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "efifw.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmxnet3.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmxnet3.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "pvscsi.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "pvscsi.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmusbmouse.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmusbmouse.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmouse.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmouse.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmemctl.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmemctl.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "svga_wddm.status"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "svga_wddm.installPath"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Tools" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware VGAuth" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc." ""

echo [INFO] Registry phase complete. >> "%LOG%"

:: ===============================================================
:: SUMMARY
:: ===============================================================
echo.
echo ===============================================================
echo  VMware Cleanup Complete
echo ===============================================================
echo.
echo  Registry entries deleted: %REG_DELETED%
echo  Registry errors:         %REG_ERRORS%
echo  Log: %LOG%
if "%REBOOT_REQUIRED%"=="1" (
    echo.
    echo  WARNING: A system reboot is required to complete removal.
)
echo ===============================================================
goto :EOF

:: ===============================================================
:: SUBROUTINES
:: ===============================================================

:: --- Disable then remove a single PnP device ---
:DisableAndRemoveDevice
setlocal enabledelayedexpansion
set "DEVID=!DEVID:~0!"
set "DEVDESC=!DEVDESC:~1!"
set "ATDEVID=@!DEVID!"
echo ---------------------------------------------------------------
echo Device: !DEVDESC!
echo ID: !ATDEVID!
echo ---------------------------------------------------------------
echo Disabling device: !DEVDESC!
"%DEVCON%" disable !ATDEVID!
echo Removing device: !DEVDESC!
"%DEVCON%" remove !ATDEVID!
echo [DRIVER] Disabled and removed !DEVDESC! >> "%LOG%"
endlocal
goto :eof

:: --- Check if a service is VMware-related ---
:CheckVMwareService
setlocal
set "SVC=%~1"
set "IS_VMWARE="
for /f "tokens=*" %%L in ('sc qc "%SVC%" 2^>nul') do (
    echo %%L | findstr /I "VMware" >nul && set "IS_VMWARE=1"
)
if defined IS_VMWARE (
    echo [MATCH] %SVC%
    echo [MATCH] %SVC% >> "%LOG%"
    endlocal & set "VMWARE_LIST=%VMWARE_LIST% %SVC%" & set /a SVC_MATCH_COUNT+=1
) else (
    endlocal
)
goto :eof

:: --- Stop, disable, and delete a service ---
:StopAndDeleteService
setlocal enabledelayedexpansion
set "SVC=%~1"
echo ---------------------------------------------------------------
echo [ACTION] Stopping, disabling, and deleting %SVC%...
echo [ACTION] Processing %SVC% >> "%LOG%"

sc stop "%SVC%" >nul 2>&1
ping 127.0.0.1 -n 3 >nul

for /f "tokens=2 delims=:" %%P in ('sc queryex "%SVC%" ^| find "PID"') do (
    set "PID=%%P"
    set "PID=!PID: =!"
    if not "!PID!"=="0" (
        echo [ACTION] Forcibly killing PID !PID!
        echo [ACTION] Killing PID !PID! >> "%LOG%"
        taskkill /PID !PID! /F >nul 2>&1
    )
)

sc config "%SVC%" start= disabled >nul 2>&1
sc delete "%SVC%" >nul 2>&1
if errorlevel 1 (
    echo [WARNING] Failed to delete %SVC%
    echo [WARNING] Failed to delete %SVC% >> "%LOG%"
) else (
    echo [SUCCESS] %SVC% deleted
    echo [SUCCESS] %SVC% deleted >> "%LOG%"
)
endlocal
goto :eof

:: --- Delete a registry entry ---
:DeleteRegistryEntry
setlocal enabledelayedexpansion
set "REG_PATH=%~1"
set "VAL_NAME=%~2"

if "!VAL_NAME!"=="" (
    reg delete "!REG_PATH!" /f >nul 2>&1
    if !errorlevel! equ 0 (
        echo [DEL KEY] !REG_PATH!
        set "RESULT=DELETED"
    ) else (
        set "RESULT=ERROR"
    )
) else (
    reg delete "!REG_PATH!" /v "!VAL_NAME!" /f >nul 2>&1
    if !errorlevel! equ 0 (
        echo [DEL VAL] !REG_PATH!\!VAL_NAME!
        set "RESULT=DELETED"
    ) else (
        reg delete "!REG_PATH!" /f >nul 2>&1
        if !errorlevel! equ 0 (
            echo [DEL KEY] !REG_PATH! ^(fallback^)
            set "RESULT=DELETED"
        ) else (
            set "RESULT=ERROR"
        )
    )
)

for %%R in ("!RESULT!") do (
    endlocal
    if "%%~R"=="DELETED" (
        set /a REG_DELETED+=1
    )
    if "%%~R"=="ERROR" (
        set /a REG_ERRORS+=1
    )
)
goto :eof
