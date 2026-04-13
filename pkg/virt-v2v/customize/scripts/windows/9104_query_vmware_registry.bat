@echo off
:: ===============================================================
:: query_vmware_registry.cmd
:: Query all VMware-related registry entries
:: Displays current registry keys and values
:: Read-only operation - does not modify registry
:: ===============================================================

setlocal enabledelayedexpansion

REM =============================
REM Check for admin privileges
REM =============================
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: This script must be run as Administrator!
    echo Please right-click and select "Run as administrator"
    pause
    exit /b 1
)

echo.
echo ===============================================================
echo   Query VMware Registry Entries Script
echo   Run as Administrator
echo ===============================================================
echo.

:: --- Initialize counters ---
set /a FOUND=0
set /a NOT_FOUND=0

echo ==========================================
echo  Querying registry entries
echo ==========================================
echo.

:: ===============================================================
:: Embedded Registry Entries
:: Format: call :QueryRegistryEntry "HKLM\Path" "ValueName"
:: Empty ValueName means query entire key
:: ===============================================================

call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\shell\open\command" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "IconsVisible"
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE" "LocalizedString"
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "ReinstallCommand"
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "HideIconsCommand"
call :QueryRegistryEntry "HKLM\SOFTWARE\Classes\Applications\VMwareHostOpen.exe\shell\open\command" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "ShowIconsCommand"
call :QueryRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\DefaultIcon" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile\shell\open\command" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL\shell\open\command" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "sftp"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "ftp"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".htm"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "news"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "http"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities" "ApplicationDescription"
call :QueryRegistryEntry "HKLM\SOFTWARE\RegisteredApplications" "VMware Host Open"
call :QueryRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".shtml"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "mailto"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".xhtml"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "feed"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "https"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "telnet"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".xht"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".html"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "ssh"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\Startmenu" "StartmenuInternet"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "*"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "TypesSupported"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "Name"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "ProviderPath"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "DeviceName"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\parameters" "ServerName"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\parameters" "ShareName"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware VGAuth" "PreferencesFile"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Tools" "InstallPath"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "TypesSupported"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "*"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "Enabled"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "*"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "TypesSupported"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "*"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "TypesSupported"
call :QueryRegistryEntry "HKLM\SOFTWARE\Classes\CLSID\{C73DA087-EDDB-4a7c-B216-8EF8A3B92C7B}\InprocServer32" ""
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "InputProvider"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "DllName"
call :QueryRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\VMUpgradeHelper" "-"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "CategoryMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "CategoryCount"
call :QueryRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\GuestIntrospection" "-"
call :QueryRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\GuestStoreUpgrade" "-"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper" "TypesSupported"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth" "TypesSupported"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools" "EventMessageFile"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools" "TypesSupported"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "CommonAppData"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "PrevBootMode"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "CacheDir"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "Windir"

:: --- VMCI Registry Entries (from export files) ---
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" ""
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Type"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Start"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "ErrorControl"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Tag"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "ImagePath"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "DisplayName"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Group"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "Owners"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci" "vwdk.installers"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" ""
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" "0"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" "Count"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmci\Enum" "NextInstance"

:: --- VMMouse Registry Entries (from export files) ---
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" ""
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Type"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Start"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "ErrorControl"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Tag"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "ImagePath"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "DisplayName"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Group"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "Owners"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse" "vwdk.installers"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" ""
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" "0"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" "Count"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmmouse\Enum" "NextInstance"

:: --- VMUsbMouse Registry Entries (from export files) ---
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" ""
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Type"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Start"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "ErrorControl"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Tag"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "ImagePath"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "DisplayName"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Group"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "Owners"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse" "vwdk.installers"
call :QueryRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmusbmouse\Parameters" ""

:: --- VMware Software Registry Entries (from vmwreg.txt) ---
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc." ""
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\CbLauncher" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\CbLauncher" "Cb.LauncherVersion"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\CbLauncher" "Cb.InstallStatus"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "VmciHostDevInst"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmci.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmci.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsock.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsock.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsockSys.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vsockDll.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "efifw.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "efifw.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmxnet3.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmxnet3.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "pvscsi.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "pvscsi.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmusbmouse.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmusbmouse.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmouse.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmouse.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmemctl.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "vmmemctl.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "svga_wddm.status"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Drivers" "svga_wddm.installPath"
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Tools" ""
call :QueryRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware VGAuth" ""

echo.
echo ===============================================================
echo  Registry query process finished
echo ===============================================================
echo.
echo Summary:
echo   Found: %FOUND% entries
echo   Not Found: %NOT_FOUND% entries
echo ===============================================================
goto :EOF

:: --- Subroutine: Query registry entry ---
:QueryRegistryEntry
setlocal enabledelayedexpansion
set "REG_PATH=%~1"
set "VAL_NAME=%~2"

:: Handle registry query based on whether Name field is empty
if "!VAL_NAME!"=="" (
    :: Empty name means query entire key
    echo ---------------------------------------------------------------
    echo [KEY] !REG_PATH!
    echo ---------------------------------------------------------------
    reg query "!REG_PATH!" >nul 2>&1
    if !errorlevel! equ 0 (
        reg query "!REG_PATH!" 2>nul
        if !errorlevel! equ 0 (
            echo [FOUND] Key exists
            set "RESULT=FOUND"
        ) else (
            echo [NOT FOUND] Key does not exist
            set "RESULT=NOT_FOUND"
        )
    ) else (
        echo [NOT FOUND] Key does not exist
        set "RESULT=NOT_FOUND"
    )
) else (
    :: Specific value name - query just that value
    echo ---------------------------------------------------------------
    echo [VALUE] !REG_PATH!\!VAL_NAME!
    echo ---------------------------------------------------------------
    reg query "!REG_PATH!" /v "!VAL_NAME!" >nul 2>&1
    if !errorlevel! equ 0 (
        reg query "!REG_PATH!" /v "!VAL_NAME!" 2>nul
        if !errorlevel! equ 0 (
            echo [FOUND] Value exists
            set "RESULT=FOUND"
        ) else (
            echo [NOT FOUND] Value does not exist
            set "RESULT=NOT_FOUND"
        )
    ) else (
        echo [NOT FOUND] Value or key does not exist
        set "RESULT=NOT_FOUND"
    )
)
echo.

:: Update counters in parent scope
for %%R in ("!RESULT!") do (
    endlocal
    if "%%~R"=="FOUND" (
        set /a FOUND+=1
    ) else if "%%~R"=="NOT_FOUND" (
        set /a NOT_FOUND+=1
    )
)
goto :eof
