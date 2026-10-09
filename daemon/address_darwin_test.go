//go:build darwin

package main

import (
	"testing"

	"tinygo.org/x/bluetooth"
)

func testAddress(t *testing.T) bluetooth.Address {
	t.Helper()
	var addr bluetooth.Address
	addr.Set("00112233-4455-6677-8899-aabbccddeeff")
	if addr.String() == "" {
		t.Fatal("macOS test UUID was not accepted")
	}
	return addr
}
