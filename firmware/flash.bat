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
    :: %CHOICE% はブロック内でparse時展開されて空になるため、
    :: for /l の %%i（ループ変数）を使って遅延展開と組み合わせる。
    :: Use for /l with %%i to avoid %CHOICE% being expanded at parse time inside the block.
    set PORT=
    for /l %%i in (1,1,!PORT_COUNT!) do (
        if "%%i"=="!CHOICE!" set PORT=!PORT_VAL_%%i!
    )
    if "!PORT!"=="" (
        echo [WARN] Invalid choice — enter COM port manually.
        set /p PORT="COM port (e.g. COM3): "
    )
)

:: Trim leading/trailing spaces safely (no echo|pipe — avoids metacharacter injection)
:: set "VAR=value" strips surrounding quotes and is safe for user input.
for /f "tokens=* delims= " %%P in ("!PORT!") do set "PORT=%%P"

:: Validate PORT using pure string operations — no pipeline involved.
:: 1) Prefix must be COM (case-insensitive)  2) Suffix must be non-empty digits only.
:: echo|findstr は !PORT! 展開でメタ文字が実行される危険があるため文字列操作で代替。
set "PORT_PREFIX=!PORT:~0,3!"
set "PORT_SUFFIX=!PORT:~3!"
set "PORT_VALID=1"
if /i not "!PORT_PREFIX!"=="COM" set PORT_VALID=0
if "!PORT_SUFFIX!"==""           set PORT_VALID=0
if "!PORT_VALID!"=="1" (
    :: Remove all digits from the suffix; if anything remains it's an invalid character.
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
echo  Done! Unplug and replug to reboot.
echo ========================================
pause
