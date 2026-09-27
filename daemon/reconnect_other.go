//go:build !darwin

// reconnect_other.go — macOS 以外は従来どおり、再接続のたびにスキャンする。
// reconnect_other.go — Non-macOS platforms scan on every reconnect, as before.

package main

const pendingConnect = false
