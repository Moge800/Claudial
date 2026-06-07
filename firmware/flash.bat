@echo off
setlocal EnableDelayedExpansion

echo ========================================
echo  Clawdial Firmware Flasher (Windows)
echo ========================================
echo.

:: Check Python
where python >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Python not found. Install from https://www.python.org/
    pause
    exit /b 1
)
for /f "tokens=*" %%v in ('python --version 2^>^&1') do set PY_VER=%%v
echo [OK] !PY_VER! found.
echo.

:: Check / install esptool
python -m esptool version >nul 2>&1
if errorlevel 1 (
    echo [INFO] Installing esptool...
    python -m pip install esptool --quiet
    if errorlevel 1 (
        echo [ERROR] Failed to install esptool.
        pause
        exit /b 1
    )
    echo [OK] esptool installed.
) else (
    for /f "tokens=2" %%v in ('python -m esptool version 2^>^&1') do (
        set ESP_VER=%%v
        goto :esptool_done
    )
    :esptool_done
    echo [OK] esptool !ESP_VER! found.
)
echo.

:: Locate firmware binary (same directory as this script)
set BIN=%~dp0clawdial-firmware.bin
if not exist "!BIN!" (
    echo [ERROR] clawdial-firmware.bin not found next to this script.
    echo         Download it from:
    echo         https://github.com/Moge800/Clawdial/releases/latest
    pause
    exit /b 1
)
echo [OK] Found: !BIN!
echo.

:: Auto-detect COM port via WMI
:: Matches CP210x / CH340 / CH341 / CH9102 / FTDI / generic USB Serial
echo [INFO] Scanning for connected devices...
set PORT_COUNT=0
for /f "tokens=*" %%L in ('powershell -NoProfile -Command ^
    "Get-WmiObject Win32_PnPEntity | Where-Object { $_.Name -match 'COM\d+' -and ($_.Name -match 'CP210|CH34|CH910|FTDI|USB Serial|Silicon Labs') } | ForEach-Object { if ($_.Name -match 'COM(\d+)') { $_.Name + '|COM' + $Matches[1] } } | Sort-Object"') do (
    set /a PORT_COUNT+=1
    for /f "tokens=1,2 delims=|" %%A in ("%%L") do (
        set PORT_LABEL_!PORT_COUNT!=%%A
        set PORT_VAL_!PORT_COUNT!=%%B
    )
)

if !PORT_COUNT!==0 (
    echo [WARN] No device auto-detected.
    echo        Connect M5Stack Dial via USB-C and check Device Manager ^> Ports.
    echo.
    set /p PORT="Enter COM port manually (e.g. COM3): "
) else if !PORT_COUNT!==1 (
    echo [OK] Auto-detected: !PORT_LABEL_1!
    set PORT=!PORT_VAL_1!
) else (
    echo Found multiple devices:
    for /l %%i in (1,1,!PORT_COUNT!) do (
        echo   [%%i] !PORT_LABEL_%%i!
    )
    echo.
    set /p CHOICE="Select device number [1-!PORT_COUNT!]: "
    if "!CHOICE!" GEQ "1" if "!CHOICE!" LEQ "!PORT_COUNT!" (
        set PORT=!PORT_VAL_%CHOICE%!
    ) else (
        echo [WARN] Invalid choice — enter COM port manually.
        set /p PORT="COM port (e.g. COM3): "
    )
)

:: Trim whitespace and validate PORT (must match COMn / COMnn)
:: トリムしてCOMポート形式か検証（スペースや不正文字による誤動作を防ぐ）
for /f "tokens=* delims= " %%P in ("!PORT!") do set PORT=%%P
echo !PORT! | findstr /i /r "^COM[0-9][0-9]*$" >nul 2>&1
if errorlevel 1 (
    echo [ERROR] "!PORT!" does not look like a valid COM port.
    pause
    exit /b 1
)
echo.

:: Flash
echo [INFO] Flashing to !PORT! at 921600 baud...
echo        This takes about 30 seconds.
echo.
python -m esptool --chip esp32s3 --port "!PORT!" --baud 921600 ^
    write_flash 0x0 "!BIN!"
if errorlevel 1 (
    echo.
    echo [ERROR] Flash failed.
    echo         - Check the COM port is correct
    echo         - Try a different USB cable or replug the device
    echo         - Hold the boot button while plugging in if auto-reset fails
    pause
    exit /b 1
)

echo.
echo ========================================
echo  Done! Unplug and replug to reboot.
echo ========================================
pause
