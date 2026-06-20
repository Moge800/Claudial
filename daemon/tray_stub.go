//go:build !windows

// tray_stub.go — 非Windows用スタブ（systrayなしでdaemonを直接実行）
// tray_stub.go — non-Windows stub: runs the daemon directly without a systray.

package main

import (
	"context"
	"log"
)

func runWithTray(cfg config) {
	// 非Windowsはコンソール実行なので、アダプタ有効化失敗はクリーンに終了する。
	// Non-Windows runs in a console — fail fast on adapter enable error.
	if err := adapter.Enable(); err != nil {
		log.Fatalf("enable adapter: %v", err)
	}
	if err := run(context.Background(), cfg); err != nil {
		log.Fatalf("daemon error: %v", err)
	}
}
