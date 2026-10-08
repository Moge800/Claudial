//go:build darwin

// reconnect_darwin.go — macOS では既知デバイスへの接続待ち（pending connect）を使い、スキャンを避ける。
// reconnect_darwin.go — On macOS, wait for a known device with a pending connect instead of scanning.
//
// CoreBluetooth はデバイスが圏内に入るまで接続要求を保留するので、スキャン不要で再接続できる。
// スキャン中は近隣の全アドバタイズがコールバックされ、tinygo bluetooth/cbgo がその都度メモリを
// リークする（1 秒に数十回、デバイスが不在の間ずっと）ため、macOS ではスキャンを初回発見時に限る。
// CoreBluetooth keeps a connect request pending until the device comes into range, so no scan is
// needed to reconnect. While scanning, every advertisement from every nearby device is delivered
// as a callback and tinygo bluetooth/cbgo leaks memory on each one (tens per second, for as long as
// the device is absent), so on macOS scanning is limited to the first discovery.

package main

const pendingConnect = true
