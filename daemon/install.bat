@echo off
setlocal EnableDelayedExpansion

echo ========================================
echo  Claudial Daemon Installer (Windows)
echo ========================================
echo.

:: Method selection
echo How would you like to get claudial-daemon.exe?
echo   [D] Download pre-built binary from GitHub Releases  (no Go required)
echo   [B] Build from source                               (requires Go)
echo.
set /p METHOD="Choice [D/b]: "
if /i "!METHOD:~0,1!"=="b" (
    set DO_BUILD=1
) else (
    set DO_BUILD=0
)
echo.

if "!DO_BUILD!"=="1" (
    :: Check Go
    where go >nul 2>&1
    if errorlevel 1 (
        echo [ERROR] Go not found. Install from https://go.dev/dl/
        pause
        exit /b 1
    )
    for /f "tokens=3" %%v in ('go version') do set GO_VER=%%v
    echo [OK] Go !GO_VER! found.
    echo.

    :: Build
    echo [1/3] Building from source...
    cd /d "%~dp0"
    go build -ldflags "-H=windowsgui" -o claudial-daemon.exe .
    if errorlevel 1 (
        echo [ERROR] Build failed.
        pause
        exit /b 1
    )
    echo [OK] claudial-daemon.exe created.
) else (
    :: Download pre-built binary
    echo [1/3] Downloading latest release...
    set DEST=%~dp0claudial-daemon.exe
    powershell -NoProfile -Command ^
        "try { Invoke-WebRequest -Uri 'https://github.com/Moge800/Claudial/releases/latest/download/claudial-daemon.exe' -OutFile '!DEST!' -UseBasicParsing; Write-Host '[OK] Downloaded.' } catch { Write-Host ('[ERROR] Download failed: ' + $_.Exception.Message); exit 1 }"
    if errorlevel 1 (
        echo [ERROR] Download failed. Check your internet connection or download manually from:
        echo         https://github.com/Moge800/Claudial/releases/latest
        pause
        exit /b 1
    )
)
echo.

:: Startup registration
echo [2/3] Startup registration
set /p STARTUP="Launch automatically on Windows startup? [y/N]: "
if /i "!STARTUP!"=="y" (
    set STARTUP_DIR=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
    set SHORTCUT=!STARTUP_DIR!\Claudial.lnk
    set EXE=%~dp0claudial-daemon.exe

    powershell -NoProfile -Command ^
        "$s=(New-Object -ComObject WScript.Shell).CreateShortcut('!SHORTCUT!');" ^
        "$s.TargetPath='!EXE!';" ^
        "$s.WorkingDirectory='%~dp0';" ^
        "$s.Description='Claudial daemon';" ^
        "$s.Save()"

    if errorlevel 1 (
        echo [WARN] Shortcut creation failed. Add manually to startup if needed.
    ) else (
        echo [OK] Shortcut registered to startup.
        echo      !SHORTCUT!
    )
) else (
    echo [SKIP] Startup registration skipped.
)
echo.

:: Claude login check
echo [3/3] Claude auth check
set CRED1=%USERPROFILE%\.claude\.credentials.json
set CRED2=%LOCALAPPDATA%\Claude\.credentials.json
set CRED3=%APPDATA%\Claude\.credentials.json
if exist "%CRED1%" (
    echo [OK] Credentials found.
) else if exist "%CRED2%" (
    echo [OK] Credentials found.
) else if exist "%CRED3%" (
    echo [OK] Credentials found.
) else (
    echo [WARN] Credentials not found. Run "claude login" first.
)
echo.

echo ========================================
echo  Done!
echo ========================================
echo.
set /p LAUNCH="Launch Claudial now? [Y/n]: "
if /i not "!LAUNCH:~0,1!"=="n" (
    start "" "%~dp0claudial-daemon.exe"
    echo [OK] Claudial started.
) else (
    echo [SKIP] Run claudial-daemon.exe to start manually.
)
echo.
pause
