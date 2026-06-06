//go:build !windows

// tray_stub.go — 非Windows用スタブ（systrayなしでdaemonを直接実行）
// tray_stub.go — non-Windows stub: runs the daemon directly without a systray.

package main

import "log"

func runWithTray(cfg config) {
	if err := run(cfg); err != nil {
		log.Fatalf("daemon error: %v", err)
	}
}
