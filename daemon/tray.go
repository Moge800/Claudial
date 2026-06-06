//go:build windows

// tray.go — Windows システムトレイ UI
// Windows system tray UI for Clawdial daemon.

package main

import (
	_ "embed"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

// runWithTray はシステムトレイを起動し、daemonをバックグラウンドで実行する。
// runWithTray starts the system tray and runs the daemon in the background.
func runWithTray(cfg config) {
	systray.Run(
		func() { onReady(cfg) },
		func() { log.Println("Tray exiting") },
	)
}

func onReady(cfg config) {
	systray.SetIcon(iconData)
	systray.SetTitle("Clawdial")

	mLog := systray.AddMenuItem("Open Log", "Open daemon.log in default editor")
	mConfig := systray.AddMenuItem("Open Config", "Open .env in default editor")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop Clawdial daemon")

	// daemonのメインループをgoroutineで実行 / Run daemon loop in background goroutine.
	go func() {
		if err := run(cfg); err != nil {
			log.Printf("daemon error: %v", err)
		}
	}()

	// メニュークリック処理 / Handle menu clicks.
	for {
		select {
		case <-mLog.ClickedCh:
			openFile(logPath())
		case <-mConfig.ClickedCh:
			// .envがなければデフォルト内容で作成してから開く（GUIなのでログだけでは気づけない）
			// Create .env with defaults if absent — log-only is invisible with windowsgui.
			openOrCreateConfig()
		case <-mQuit.ClickedCh:
			// systray.Quit()を呼んでRunが正常にアンワインドするのを待つ
			// Call systray.Quit() and return so systray.Run() unwinds normally.
			systray.Quit()
			return
		}
	}
}

// logPath はexeと同じフォルダのdaemon.logパスを返す。
// logPath returns the path to daemon.log next to the executable.
func logPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "daemon.log"
	}
	return filepath.Join(filepath.Dir(exe), "daemon.log")
}

// configPath は.envのパスを返す（exe隣を優先）。
// configPath returns the .env path (next to exe preferred).
func configPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ".env"
	}
	return filepath.Join(filepath.Dir(exe), ".env")
}

// openFile はOSのデフォルトアプリでファイルを開く。
// openFile opens a file with the OS default application.
func openFile(path string) {
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start(); err != nil {
		log.Printf("Failed to open %s: %v", path, err)
	}
}

// defaultEnvContent は.envが存在しない場合に書き込むデフォルト内容。
// defaultEnvContent is written to .env when the file does not exist.
const defaultEnvContent = `# Clawdial daemon configuration
# BLE_DEVICE_NAME: name of your M5Dial (set in firmware via long-press)
BLE_DEVICE_NAME=Clawdial

# POLL_INTERVAL_SECONDS: how often to query Anthropic API (seconds, default 60)
POLL_INTERVAL_SECONDS=60
`

// openOrCreateConfig は.envが存在しなければデフォルト内容で作成してから開く。
// openOrCreateConfig creates .env with defaults if absent, then opens it.
func openOrCreateConfig() {
	p := configPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if werr := os.WriteFile(p, []byte(defaultEnvContent), 0600); werr != nil {
			log.Printf("Failed to create config: %v", werr)
			return
		}
		log.Printf("Created default config: %s", p)
	}
	openFile(p)
}
