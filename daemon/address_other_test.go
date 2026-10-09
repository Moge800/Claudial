//go:build !darwin

package main

import (
	"testing"

	"tinygo.org/x/bluetooth"
)

func testAddress(t *testing.T) bluetooth.Address {
	t.Helper()
	var addr bluetooth.Address
	addr.Set("AA:BB:CC:DD:EE:FF")
	if addr.String() != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("test MAC address was not accepted: %q", addr.String())
	}
	return addr
}
