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

:: COM port
echo Connect M5Stack Dial via USB-C, then enter the COM port.
echo (Check Device Manager ^> Ports if unsure)
echo.
set /p PORT="COM port (e.g. COM3): "
echo.

:: Flash
echo [INFO] Flashing to !PORT! at 921600 baud...
echo        This takes about 30 seconds.
echo.
python -m esptool --chip esp32s3 --port !PORT! --baud 921600 ^
    write_flash 0x0 "!BIN!"
if errorlevel 1 (
    echo.
    echo [ERROR] Flash failed.
    echo         - Check the COM port is correct
    echo         - Try a different USB cable
    echo         - Hold the boot button while plugging in if auto-reset fails
    pause
    exit /b 1
)

echo.
echo ========================================
echo  Done! Unplug and replug to reboot.
echo ========================================
pause
