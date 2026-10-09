//go:build darwin

package main

import (
	"strings"
	"testing"

	"tinygo.org/x/bluetooth"
)

func testAddress(t *testing.T) bluetooth.Address {
	t.Helper()
	const want = "00112233-4455-6677-8899-aabbccddeeff"
	var addr bluetooth.Address
	addr.Set(want)
	if got := strings.ToLower(addr.String()); got != want {
		t.Fatalf("macOS test UUID was not parsed: got %q want %q", got, want)
	}
	return addr
}
