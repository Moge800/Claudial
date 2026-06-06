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

	mLog    := systray.AddMenuItem("Open Log", "Open daemon.log in default editor")
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
			// .envが存在しない場合はログに記録するだけ / Log if .env is absent.
			p := configPath()
			if _, err := os.Stat(p); err != nil {
				log.Printf("Config file not found: %s", p)
			} else {
				openFile(p)
			}
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
