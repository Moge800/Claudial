//go:build windows

// tray.go — Windows システムトレイ UI
// Windows system tray UI for Claudial daemon.

package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"time"

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
	systray.SetTitle("Claudial")

	mLog := systray.AddMenuItem("Open Log", "Open daemon.log in default editor")
	mConfig := systray.AddMenuItem("Open Config", "Open .env in default editor (restart to apply changes)")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop Claudial daemon")

	// Quitで再接続ループをキャンセルできるようcontextを渡す。
	// Pass a cancellable context so Quit stops the reconnect loop cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	// daemonのメインループをgoroutineで実行 / Run daemon loop in background goroutine.
	// tinygo-bluetooth は DiscoverServices の WinRT 呼び出しで panic することがある
	// (status 2 Canceled の直後)。panic はプロセス全体を巻き込みトレイアイコンごと消すため、
	// recover してループを再開する。
	// tinygo-bluetooth can panic inside the DiscoverServices WinRT call (right after a
	// status-2 Canceled). A panic kills the whole process and the tray icon with it, so
	// recover it and restart the loop instead.
	go func() {
		defer close(done)

		// BLEスタックは起動直後（shell:startup等）に未準備のことがあるので、
		// 成功するまでEnableをリトライする。成功後は再呼び出ししない
		// （Windowsは再Enableでエラーになる）。
		// The BLE stack may not be ready right after boot (e.g. shell:startup), so retry
		// Enable until it succeeds. It is never called again afterward (re-Enable errors on Windows).
		for {
			err := adapter.Enable()
			if err == nil {
				break
			}
			log.Printf("enable adapter: %v. Retrying in 5s...", err)
			t := time.NewTimer(5 * time.Second)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return
			}
		}

		for ctx.Err() == nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("daemon panic recovered: %v\n%s", r, debug.Stack())
					}
				}()
				if err := run(ctx, cfg); err != nil {
					log.Printf("daemon error: %v", err)
				}
			}()
			if ctx.Err() != nil {
				return
			}
			log.Println("daemon loop exited; restarting in 5s")
			t := time.NewTimer(5 * time.Second)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return
			}
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
			// daemonのgoroutineが終了してからsystrayを閉じる。
			// Cancel the daemon goroutine and wait for it to finish before quitting the tray.
			cancel()
			<-done
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
// Keys must match what loadConfig() reads via os.Getenv.
// defaultEnvContent is written to .env when the file does not exist.
const defaultEnvContent = `# Claudial daemon configuration
# CLAUDIAL_DEVICE_NAME: BLE name of your M5Dial (set in firmware via long-press)
CLAUDIAL_DEVICE_NAME=Claudial

# CLAUDIAL_POLL_INTERVAL: how often to query Anthropic API, in whole seconds
CLAUDIAL_POLL_INTERVAL=60

# CLAUDIAL_SCAN_TIMEOUT: BLE scan timeout per attempt, in whole seconds
CLAUDIAL_SCAN_TIMEOUT=15
`

// openOrCreateConfig は.envが存在しなければデフォルト内容で作成してから開く。
// openOrCreateConfig creates .env with defaults if absent, then opens it.
func openOrCreateConfig() {
	p := configPath()
	// O_EXCL でアトミックに作成 — stat と write の間に別プロセスが作った場合も上書きしない。
	// Use O_EXCL for atomic creation — avoids clobbering a config written between stat and write.
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if !os.IsExist(err) {
			// ErrExist 以外（パーミッション拒否など）は開けないのでログだけ出して終了。
			// Non-ErrExist errors (e.g. permission denied) mean we can't open the file either.
			log.Printf("Cannot create config %s: %v", p, err)
			return
		}
		// ErrExist: ファイルが既存 → そのまま開く / File already exists — open it as-is.
	} else {
		_, werr := f.WriteString(defaultEnvContent)
		cerr := f.Close()
		if werr != nil || cerr != nil {
			// 書き込み/クローズ失敗時は壊れたファイルを残さず削除する。
			// Remove the file on write/close failure to avoid leaving a broken config.
			os.Remove(p)
			log.Printf("Failed to write config (write=%v close=%v); removed partial file", werr, cerr)
			return
		}
		log.Printf("Created default config: %s", p)
	}
	openFile(p)
}
