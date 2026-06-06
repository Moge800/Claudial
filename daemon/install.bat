@echo off
setlocal EnableDelayedExpansion

echo ========================================
echo  Clawdial Daemon Installer (Windows)
echo ========================================
echo.

:: Check Go
where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found. Install from https://go.dev/dl/
    pause
    exit /b 1
)
for /f "tokens=3" %%v in ('go version') do set GO_VER=%%v
echo [OK] Go %GO_VER% found.
echo.

:: Build
echo [1/3] Building...
cd /d "%~dp0"
go build -ldflags "-H=windowsgui" -o clawdial-daemon.exe .
if errorlevel 1 (
    echo [ERROR] Build failed.
    pause
    exit /b 1
)
echo [OK] clawdial-daemon.exe created.
echo.

:: Startup registration
echo [2/3] Startup registration
set /p STARTUP="Launch automatically on Windows startup? [y/N]: "
if /i "!STARTUP!"=="y" (
    set STARTUP_DIR=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
    set SHORTCUT=!STARTUP_DIR!\clawdial-daemon.bat

    echo @echo off > "!SHORTCUT!"
    echo start "" /min "%~dp0clawdial-daemon.exe" >> "!SHORTCUT!"

    echo [OK] Registered to startup.
    echo      !SHORTCUT!
) else (
    echo [SKIP] Startup registration skipped.
)
echo.

:: Claude login check
echo [3/3] Claude auth check
set CRED=%USERPROFILE%\.claude\.credentials.json
if exist "%CRED%" (
    echo [OK] Credentials found.
) else (
    echo [WARN] Credentials not found. Run "claude login" first.
)
echo.

echo ========================================
echo  Done! Run: clawdial-daemon.exe
echo ========================================
pause
