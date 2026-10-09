package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func useTestAddressFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "device-address")
	previous := addressFilePath
	addressFilePath = func() (string, error) { return p, nil }
	t.Cleanup(func() { addressFilePath = previous })
	return p
}

func TestAddressRoundTripWithSpaceInDeviceName(t *testing.T) {
	useTestAddressFile(t)
	want := testAddress(t)

	saveAddress("Desk Dial", want)
	got, ok := loadAddress("Desk Dial")

	if !ok || got.String() != want.String() {
		t.Fatalf("round trip failed: ok=%v got=%q want=%q", ok, got.String(), want.String())
	}
}

func TestLoadAddressRejectsDifferentDeviceName(t *testing.T) {
	useTestAddressFile(t)
	saveAddress("Desk Dial", testAddress(t))

	if _, ok := loadAddress("Other Dial"); ok {
		t.Fatal("address for a different device name was accepted")
	}
}

func TestLoadLegacyAddressWithSpaceInDeviceName(t *testing.T) {
	useTestAddressFile(t)
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
	useTestAddressFile(t)
	saveAddress("Desk Dial", testAddress(t))

	if err := clearAddress(); err != nil {
		t.Fatal(err)
	}

	if _, ok := loadAddress("Desk Dial"); ok {
		t.Fatal("cleared address was still loaded")
	}
}

func TestAddressOperationsUseInjectedPath(t *testing.T) {
	production := filepath.Join(t.TempDir(), "production-device-address")
	if err := os.WriteFile(production, []byte("must stay"), 0600); err != nil {
		t.Fatal(err)
	}
	testPath := useTestAddressFile(t)

	saveAddress("Desk Dial", testAddress(t))
	if err := clearAddress(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Fatalf("injected address file was not removed: %v", err)
	}
	if got, err := os.ReadFile(production); err != nil || string(got) != "must stay" {
		t.Fatalf("unrelated production file changed: content=%q err=%v", got, err)
	}
}

func TestLoadAddressRejectsMissingAndInvalidFiles(t *testing.T) {
	p := useTestAddressFile(t)
	if _, ok := loadAddress("Desk Dial"); ok {
		t.Fatal("missing address file was accepted")
	}
	if err := os.WriteFile(p, []byte("not-json-or-legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadAddress("Desk Dial"); ok {
		t.Fatal("invalid address file was accepted")
	}
}

func TestNextScanRetryWait(t *testing.T) {
	tests := []struct {
		current time.Duration
		want    time.Duration
	}{
		{0, 5 * time.Second},
		{5 * time.Second, 10 * time.Second},
		{40 * time.Minute, time.Hour},
		{time.Hour, time.Hour},
	}
	for _, tc := range tests {
		if got := nextScanRetryWait(tc.current); got != tc.want {
			t.Errorf("nextScanRetryWait(%s)=%s, want %s", tc.current, got, tc.want)
		}
	}
}

func TestForgetDeviceRequested(t *testing.T) {
	if !forgetDeviceRequested([]string{"claudial-daemon", "--forget-device"}) {
		t.Fatal("--forget-device was not recognized")
	}
	for _, args := range [][]string{
		{"claudial-daemon"},
		{"claudial-daemon", "--forget-device", "extra"},
		{"claudial-daemon", "--other"},
	} {
		if forgetDeviceRequested(args) {
			t.Fatalf("unexpected forget request for %#v", args)
		}
	}
}
