#!/bin/bash
set -e

echo "========================================"
echo " Clawdial Daemon Installer"
echo "========================================"
echo

# Go チェック
if ! command -v go &>/dev/null; then
    echo "[ERROR] Go が見つかりません。"
    echo "  https://go.dev/dl/ からインストールしてください。"
    exit 1
fi
GO_VER=$(go version | awk '{print $3}')
echo "[OK] ${GO_VER} を確認しました。"
echo

# ビルド
echo "[1/3] ビルド中..."
cd "$(dirname "$0")"
go build -o clawdial-daemon .
echo "[OK] clawdial-daemon を生成しました。"
echo

# スタートアップ登録
echo "[2/3] 自動起動登録"
DAEMON_PATH="$(pwd)/clawdial-daemon"

if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS: launchd
    PLIST="$HOME/Library/LaunchAgents/io.github.moge800.clawdial-daemon.plist"
    read -rp "macOS 起動時に自動起動しますか？ [y/N]: " STARTUP
    if [[ "$STARTUP" =~ ^[Yy]$ ]]; then
        cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.github.moge800.clawdial-daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>${DAEMON_PATH}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${HOME}/.clawdial/daemon.log</string>
    <key>StandardErrorPath</key>
    <string>${HOME}/.clawdial/daemon.log</string>
</dict>
</plist>
EOF
        mkdir -p "$HOME/.clawdial"
        launchctl load "$PLIST"
        echo "[OK] launchd に登録しました: $PLIST"
    else
        echo "[SKIP] 自動起動登録をスキップしました。"
    fi

elif command -v systemctl &>/dev/null; then
    # Linux: systemd (user)
    SERVICE_DIR="$HOME/.config/systemd/user"
    SERVICE="$SERVICE_DIR/clawdial-daemon.service"
    read -rp "systemd ユーザーサービスとして自動起動しますか？ [y/N]: " STARTUP
    if [[ "$STARTUP" =~ ^[Yy]$ ]]; then
        mkdir -p "$SERVICE_DIR"
        cat > "$SERVICE" <<EOF
[Unit]
Description=Clawdial BLE Daemon
After=network.target

[Service]
ExecStart="${DAEMON_PATH}"
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF
        systemctl --user daemon-reload
        systemctl --user enable --now clawdial-daemon.service
        echo "[OK] systemd に登録しました: $SERVICE"
    else
        echo "[SKIP] 自動起動登録をスキップしました。"
    fi
else
    echo "[SKIP] 自動起動の登録方法が不明です（systemd が見つかりません）。"
    echo "       手動で起動してください: $DAEMON_PATH"
fi
echo

# claude login チェック
echo "[3/3] Claude 認証チェック"
CRED="$HOME/.claude/.credentials.json"
if [[ -f "$CRED" ]]; then
    echo "[OK] 認証情報が見つかりました。"
else
    echo "[WARN] 認証情報が見つかりません。"
    echo "       先に 'claude login' を実行してください。"
fi
echo

echo "========================================"
echo " インストール完了！"
echo " 起動: ./clawdial-daemon"
echo "========================================"
