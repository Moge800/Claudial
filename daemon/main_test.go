package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"tinygo.org/x/bluetooth"
)

func testAddress(t *testing.T) bluetooth.Address {
	t.Helper()
	var addr bluetooth.Address
	addr.Set("AA:BB:CC:DD:EE:FF")
	if addr.String() != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("unexpected test address: %q", addr.String())
	}
	return addr
}

func TestAddressRoundTripWithSpaceInDeviceName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := testAddress(t)

	saveAddress("Desk Dial", want)
	got, ok := loadAddress("Desk Dial")

	if !ok || got.String() != want.String() {
		t.Fatalf("round trip failed: ok=%v got=%q want=%q", ok, got.String(), want.String())
	}
}

func TestLoadAddressRejectsDifferentDeviceName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saveAddress("Desk Dial", testAddress(t))

	if _, ok := loadAddress("Other Dial"); ok {
		t.Fatal("address for a different device name was accepted")
	}
}

func TestLoadLegacyAddressWithSpaceInDeviceName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := testAddress(t)
	p, err := addressFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("Desk Dial "+want.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, ok := loadAddress("Desk Dial")
	if !ok || got.String() != want.String() {
		t.Fatalf("legacy load failed: ok=%v got=%q want=%q", ok, got.String(), want.String())
	}
}

func TestIsUnknownPeerError(t *testing.T) {
	if !isUnknownPeerError(errors.New("Connect failed: no peer with address: ABC")) {
		t.Fatal("unknown peer error was not recognized")
	}
	if isUnknownPeerError(errors.New("timeout on Connect")) {
		t.Fatal("ordinary out-of-range timeout must not discard the saved address")
	}
}

func TestClearAddressRemovesSavedDevice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saveAddress("Desk Dial", testAddress(t))

	clearAddress()

	if _, ok := loadAddress("Desk Dial"); ok {
		t.Fatal("cleared address was still loaded")
	}
}
