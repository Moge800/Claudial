//go:build windows

// mutex_windows.go — 多重起動防止（名前付きMutex）
// mutex_windows.go — Single-instance guard via named mutex.

package main

import (
	"log"
	"os"

	"golang.org/x/sys/windows"
)

// ensureSingleInstance は同名のMutexが既に存在する場合にメッセージを表示して終了する。
// ensureSingleInstance exits if another instance is already running.
// Mutexはプロセス終了時にOSが自動解放するため明示的なCloseは不要。
// The OS releases the mutex automatically on process exit — no explicit Close needed.
func ensureSingleInstance() {
	_, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr("Local\\ClaudialDaemon"))
	if err == windows.ERROR_ALREADY_EXISTS {
		windows.MessageBox(
			0,
			windows.StringToUTF16Ptr("Claudial is already running.\nCheck the system tray."),
			windows.StringToUTF16Ptr("Claudial"),
			windows.MB_OK|windows.MB_ICONINFORMATION,
		)
		os.Exit(0)
	}
	if err != nil {
		// Mutexの作成自体は失敗しても致命的ではない — ログだけ出して続行。
		// CreateMutex failure is not fatal — log and continue without the guard.
		log.Printf("CreateMutex failed (single-instance guard unavailable): %v", err)
	}
}
