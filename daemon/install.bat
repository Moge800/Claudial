@echo off
setlocal EnableDelayedExpansion
chcp 65001 >nul

echo ========================================
echo  Clawdial Daemon Installer (Windows)
echo ========================================
echo.

:: Go チェック
where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go が見つかりません。
    echo https://go.dev/dl/ からインストールしてください。
    pause
    exit /b 1
)
for /f "tokens=3" %%v in ('go version') do set GO_VER=%%v
echo [OK] Go %GO_VER% を確認しました。
echo.

:: ビルド
echo [1/3] ビルド中...
cd /d "%~dp0"
go build -ldflags "-H=windowsgui" -o clawdial-daemon.exe .
if errorlevel 1 (
    echo [ERROR] ビルドに失敗しました。
    pause
    exit /b 1
)
echo [OK] clawdial-daemon.exe を生成しました。
echo.

:: スタートアップ登録の確認
echo [2/3] スタートアップ登録
set /p STARTUP="Windows 起動時に自動起動しますか？ [y/N]: "
if /i "!STARTUP!"=="y" (
    set STARTUP_DIR=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
    set SHORTCUT=!STARTUP_DIR!\clawdial-daemon.bat

    echo @echo off > "!SHORTCUT!"
    echo start "" "%~dp0clawdial-daemon.exe" >> "!SHORTCUT!"

    echo [OK] スタートアップに登録しました。
    echo      !SHORTCUT!
) else (
    echo [SKIP] スタートアップ登録をスキップしました。
)
echo.

:: claude login チェック
echo [3/3] Claude 認証チェック
set CRED=%USERPROFILE%\.claude\.credentials.json
if exist "%CRED%" (
    echo [OK] 認証情報が見つかりました。
) else (
    echo [WARN] 認証情報が見つかりません。
    echo        先に "claude login" を実行してください。
)
echo.

echo ========================================
echo  インストール完了！
echo  起動: clawdial-daemon.exe
echo ========================================
pause
