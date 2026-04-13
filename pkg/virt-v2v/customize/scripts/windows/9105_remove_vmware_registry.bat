@echo off
:: ===============================================================
:: remove_vmware_registry.cmd
:: Remove all VMware-related registry entries
:: Processes embedded registry keys and values
:: Maps all entries to HKEY_LOCAL_MACHINE (HKLM)
:: Handles both key deletions and individual value deletions
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
echo   Remove VMware Registry Entries Script
echo   Run as Administrator
echo ===============================================================
echo.

:: --- Initialize counters ---
set /a DELETED=0
set /a ERRORS=0

echo ==========================================
echo  Processing registry entries
echo ==========================================
echo.

:: ===============================================================
:: Embedded Registry Entries
:: Format: call :DeleteRegistryEntry "HKLM\Path" "ValueName"
:: Empty ValueName means delete entire key
:: ===============================================================

call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "IconsVisible"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE" "LocalizedString"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "ReinstallCommand"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "HideIconsCommand"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\Applications\VMwareHostOpen.exe\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\InstallInfo" "ShowIconsCommand"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Clients\StartmenuInternet\VMWAREHOSTOPEN.EXE\DefaultIcon" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL\shell\open\command" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "sftp"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "ftp"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".htm"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "news"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "http"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities" "ApplicationDescription"
call :DeleteRegistryEntry "HKLM\SOFTWARE\RegisteredApplications" "VMware Host Open"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocURL" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\VMwareHostOpen.AssocFile" ""
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".shtml"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "mailto"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".xhtml"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "feed"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "https"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "telnet"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".xht"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\FileAssociations" ".html"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\UrlAssociations" "ssh"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMwareHostOpen\Capabilities\Startmenu" "StartmenuInternet"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmtools" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "Name"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "ProviderPath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\networkprovider" "DeviceName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\parameters" "ServerName"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmhgfs\parameters" "ShareName"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware VGAuth" "PreferencesFile"
call :DeleteRegistryEntry "HKLM\SOFTWARE\VMware, Inc.\VMware Tools" "InstallPath"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "Enabled"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vnetWFP" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "*"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\services\eventLog\System\vsepflt" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SOFTWARE\Classes\CLSID\{C73DA087-EDDB-4a7c-B216-8EF8A3B92C7B}\InprocServer32" ""
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "InputProvider"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\W32Time\TimeProviders\vmwTimeProvider" "DllName"
call :DeleteRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\VMUpgradeHelper" "-"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "CategoryMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\vmStatsProvider" "CategoryCount"
call :DeleteRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\GuestIntrospection" "-"
call :DeleteRegistryEntry "HKLM\Software\VMware, Inc.\VMware Tools\GuestStoreUpgrade" "-"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMUpgradeHelper" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VGAuth" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools" "EventMessageFile"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\Eventlog\Application\VMware Tools" "TypesSupported"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "CommonAppData"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "PrevBootMode"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "CacheDir"
call :DeleteRegistryEntry "HKLM\SYSTEM\CurrentControlSet\Services\vmrawdsk\Parameters" "Windir"

:: --- VMCI Registry Entries (from export files) ---
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

:: --- VMMouse Registry Entries ---
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

:: --- VMUsbMouse Registry Entries (from export files) ---
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

:: --- VMware Software Registry Entries (from vmwreg.txt) ---
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

echo.
echo ===============================================================
echo  Registry cleanup process finished
echo ===============================================================
echo.
echo Summary:
echo   Deleted: %DELETED% entries
echo   Errors: %ERRORS% entries
echo ===============================================================
goto :EOF

:: --- Subroutine: Delete registry entry ---
:DeleteRegistryEntry
setlocal enabledelayedexpansion
set "REG_PATH=%~1"
set "VAL_NAME=%~2"
set "RESULT_VAR=RESULT"

:: Handle registry deletion based on whether Name field is empty
if "!VAL_NAME!"=="" (
    :: Empty name means default value - delete the entire key
    echo [DELETE KEY] !REG_PATH!
    reg delete "!REG_PATH!" /f >nul 2>&1
    if !errorlevel! equ 0 (
        echo [SUCCESS] Deleted key: !REG_PATH!
        set "RESULT=DELETED"
    ) else (
        echo [ERROR] Failed to delete key: !REG_PATH!
        set "RESULT=ERROR"
    )
) else (
    :: Specific value name - delete just that value
    echo [DELETE VALUE] !REG_PATH!\!VAL_NAME!
    reg delete "!REG_PATH!" /v "!VAL_NAME!" /f >nul 2>&1
    if !errorlevel! equ 0 (
        echo [SUCCESS] Deleted value: !REG_PATH!\!VAL_NAME!
        set "RESULT=DELETED"
    ) else (
        :: If value deletion fails, try deleting parent key
        echo [WARNING] Value not found, attempting to delete parent key: !REG_PATH!
        reg delete "!REG_PATH!" /f >nul 2>&1
        if !errorlevel! equ 0 (
            echo [SUCCESS] Deleted parent key: !REG_PATH!
            set "RESULT=DELETED"
        ) else (
            echo [ERROR] Failed to delete: !REG_PATH!\!VAL_NAME!
            set "RESULT=ERROR"
        )
    )
)

:: Return result to parent scope and update counters
for %%R in ("!RESULT!") do (
    endlocal
    if "%%~R"=="DELETED" (
        set /a DELETED+=1
    ) else if "%%~R"=="ERROR" (
        set /a ERRORS+=1
    )
)
goto :eof
