//go:build !windows

// mutex_other.go — 多重起動防止（~/.claudial/daemon.lock の flock）
// mutex_other.go — Single-instance guard via flock on ~/.claudial/daemon.lock.

package main

import (
	"log"
	"os"
	"path/filepath"
	"syscall"
)

// lockFile はプロセスが生きている間ロックを保持するため開いたままにする。
// lockFile stays open for the lifetime of the process so the lock is held until exit.
var lockFile *os.File

// ensureSingleInstance は他のインスタンスがロックを保持している場合にメッセージを表示して終了する。
// ensureSingleInstance exits if another instance already holds the lock.
// ロックはプロセス終了時にOSが自動解放する（kill -9 でも残らない）。
// The OS releases the lock on process exit, including after kill -9.
func ensureSingleInstance() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".claudial")
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("Cannot create %s (single-instance guard unavailable): %v", dir, err)
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		log.Printf("Cannot open lock file (single-instance guard unavailable): %v", err)
		return
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		log.Fatalf("Claudial daemon is already running (lock held on %s). Exiting.", f.Name())
	}
	lockFile = f
}
