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
    python -m pip install esptool --quiet || python -m pip install --user esptool --quiet
    if errorlevel 1 (
        echo [ERROR] Failed to install esptool.
        pause
        exit /b 1
    )
    echo [OK] esptool installed.
) else (
    for /f "tokens=2" %%v in ('python -m esptool version 2^>^&1') do set "ESP_VER=%%v"
    echo [OK] esptool !ESP_VER! found.
)
echo.

:: Locate firmware binary
set "BIN=%~dp0clawdial-firmware.bin"
if not exist "!BIN!" (
    echo [ERROR] clawdial-firmware.bin not found next to this script.
    echo         Download it from:
    echo         https://github.com/Moge800/Clawdial/releases/latest
    pause
    exit /b 1
)
echo [OK] Found: !BIN!
echo.

:: Auto-detect COM port
echo [INFO] Scanning for connected devices...
set "CWPORTS_TMP=%TEMP%\cwports_%RANDOM%_%RANDOM%.tmp"
reg query "HKLM\HARDWARE\DEVICEMAP\SERIALCOMM" 2>nul | findstr "REG_SZ" > "!CWPORTS_TMP!"
set PORT_COUNT=0
if exist "!CWPORTS_TMP!" (
    for /f "usebackq tokens=3" %%P in ("!CWPORTS_TMP!") do (
        set /a PORT_COUNT+=1
        set "PORT_VAL_!PORT_COUNT!=%%P"
    )
    del "!CWPORTS_TMP!" 2>nul
)

if !PORT_COUNT!==0 (
    echo [WARN] No COM port found. Connect M5Stack Dial via USB-C and check Device Manager.
    echo.
    set /p PORT="Enter COM port manually (e.g. COM3): "
) else if !PORT_COUNT!==1 (
    echo [OK] Auto-detected: !PORT_VAL_1!
    set PORT=!PORT_VAL_1!
) else (
    echo Found multiple devices:
    for /l %%i in (1,1,!PORT_COUNT!) do (
        echo   [%%i] !PORT_VAL_%%i!
    )
    echo.
    set /p CHOICE="Select device number [1-!PORT_COUNT!]: "
    set /a CHOICE=CHOICE >nul 2>nul
    set "PORT="
    for /l %%i in (1,1,!PORT_COUNT!) do (
        if "%%i"=="!CHOICE!" set "PORT=!PORT_VAL_%%i!"
    )
    if "!PORT!"=="" (
        echo [WARN] Invalid choice - enter COM port manually.
        set /p PORT="COM port (e.g. COM3): "
    )
)

:: Normalize PORT
set "PORT=%PORT:^"=%"
set "PORT=%PORT: =%"

:: Validate PORT — pure string ops, no metacharacter injection risk
set "PORT_PREFIX=!PORT:~0,3!"
set "PORT_SUFFIX=!PORT:~3!"
if /i not "!PORT_PREFIX!"=="COM" goto :invalid_port
if "!PORT_SUFFIX!"=="" goto :invalid_port
set "PORT_CHECK=!PORT_SUFFIX!"
set "PORT_CHECK=!PORT_CHECK:0=!"
set "PORT_CHECK=!PORT_CHECK:1=!"
set "PORT_CHECK=!PORT_CHECK:2=!"
set "PORT_CHECK=!PORT_CHECK:3=!"
set "PORT_CHECK=!PORT_CHECK:4=!"
set "PORT_CHECK=!PORT_CHECK:5=!"
set "PORT_CHECK=!PORT_CHECK:6=!"
set "PORT_CHECK=!PORT_CHECK:7=!"
set "PORT_CHECK=!PORT_CHECK:8=!"
set "PORT_CHECK=!PORT_CHECK:9=!"
if not "!PORT_CHECK!"=="" goto :invalid_port
goto :port_ok
:invalid_port
echo [ERROR] "!PORT!" does not look like a valid COM port.
pause
exit /b 1
:port_ok
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
echo  Done^! Unplug and replug to reboot.
echo ========================================
pause
