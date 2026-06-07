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

:: Auto-detect COM port via WMI.
:: First try devices matching known USB-serial chips; if none found, list ALL COM ports
:: so the user can pick without needing to know the chip name.
:: Temp-file approach: PowerShell writes port numbers to a file; for /f reads from the
:: file (not a pipe).  for /f from a file strips \r correctly, so PORT_VAL tokens are
:: always clean -- no hidden CR regardless of PowerShell's stdout text-mode translation.
echo [INFO] Scanning for connected devices...
set "CWPORTS_TMP=%TEMP%\cwports_%RANDOM%.tmp"
powershell -NoProfile -Command ^
    "$all = Get-WmiObject Win32_PnPEntity | Where-Object { $_.Name -match 'COM\d+' }; $known = $all | Where-Object { $_.Name -match 'CP210|CH34|CH910|FTDI|USB Serial|Silicon Labs' }; $src = if ($known) { $known } else { $all }; $src | ForEach-Object { if ($_.Name -match 'COM(\d+)') { [int]$Matches[1] } } | Sort-Object -Unique" ^
    > "!CWPORTS_TMP!"
set PORT_COUNT=0
for /f "usebackq tokens=*" %%L in ("!CWPORTS_TMP!") do (
    set /a PORT_COUNT+=1
    set "PORT_VAL_!PORT_COUNT!=COM%%L"
)
if exist "!CWPORTS_TMP!" del "!CWPORTS_TMP!" 2>nul

if !PORT_COUNT!==0 (
    echo [WARN] No COM port found at all.
    echo        Connect M5Stack Dial via USB-C and check Device Manager ^> Ports.
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
    :: Normalize to a plain integer without expanding user input into the command line.
    :: set /a reads CHOICE by name so metacharacters in the value are never shell-expanded.
    set /a CHOICE=CHOICE 2>nul
    :: for /l with %%i avoids %CHOICE% parse-time expansion inside a block.
    set PORT=
    for /l %%i in (1,1,!PORT_COUNT!) do (
        if "%%i"=="!CHOICE!" set PORT=!PORT_VAL_%%i!
    )
    if "!PORT!"=="" (
        echo [WARN] Invalid choice - enter COM port manually.
        set /p PORT="COM port (e.g. COM3): "
    )
)

:: Normalize PORT: strip surrounding quotes and all spaces (handles leading, trailing, embedded).
:: COM port names never contain spaces, so this is safe and simpler than a partial trim.
set "PORT=!PORT:"=!"
set "PORT=!PORT: =!"

:: Validate PORT: prefix must be COM (case-insensitive), suffix must be digits only.
:: PORT_VAL entries are already clean "COMn" (reconstructed at capture time).
:: For manually entered PORT (set /p), digit-strip catches arithmetic operators
:: like '+' or '*' that set /a would silently evaluate to a wrong port number.
set "PORT_PREFIX=!PORT:~0,3!"
set "PORT_SUFFIX=!PORT:~3!"
set "PORT_VALID=1"
if /i not "!PORT_PREFIX!"=="COM" set PORT_VALID=0
if "!PORT_SUFFIX!"==""           set PORT_VALID=0
if "!PORT_VALID!"=="1" (
    set "PORT_CHECK=!PORT_SUFFIX!"
    for %%D in (0 1 2 3 4 5 6 7 8 9) do set "PORT_CHECK=!PORT_CHECK:%%D=!"
    if not "!PORT_CHECK!"=="" set PORT_VALID=0
)
if "!PORT_VALID!"=="0" (
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
echo  Done^! Unplug and replug to reboot.
echo ========================================
pause
