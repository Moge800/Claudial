//go:build !windows

// mutex_other.go — 非Windowsではシングルインスタンスガードは行わない。
// mutex_other.go — No-op single-instance guard for non-Windows platforms.

package main

func ensureSingleInstance() {}
