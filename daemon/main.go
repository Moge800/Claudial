// Clawdial daemon — Claude Code usage monitor via BLE.
//
// Usage data is read from rate-limit headers returned by the Anthropic API,
// following the approach used by Clawdmeter (github.com/HermannBjorgvin/Clawdmeter).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"tinygo.org/x/bluetooth"
)

const (
	apiURL       = "https://api.anthropic.com/v1/messages"
	rxUUID       = "4c41555a-4465-7669-6365-000000000002"
	maxRetryWait = 5 * time.Minute
)

var apiBody = []byte(`{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)

// ---- config ----

type config struct {
	deviceName   string
	pollInterval time.Duration
	scanTimeout  time.Duration
}

func loadConfig() config {
	// 実行ファイルと同じディレクトリの .env を読む（なければ無視）
	exe, _ := os.Executable()
	_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
	// カレントディレクトリの .env も読む（開発時用）
	_ = godotenv.Load()

	cfg := config{
		deviceName:   "Clawdial",
		pollInterval: 60 * time.Second,
		scanTimeout:  15 * time.Second,
	}
	if v := os.Getenv("CLAWDIAL_DEVICE_NAME"); v != "" {
		cfg.deviceName = v
	}
	if v := os.Getenv("CLAWDIAL_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.pollInterval = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("CLAWDIAL_SCAN_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.scanTimeout = time.Duration(n) * time.Second
		}
	}
	return cfg
}

// ---- credentials ----

func loadToken() (string, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".claude", ".credentials.json"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Claude", ".credentials.json"),
		filepath.Join(os.Getenv("APPDATA"), "Claude", ".credentials.json"),
	}
	for _, p := range candidates {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if tok := extractToken(raw); tok != "" {
			return tok, nil
		}
	}
	return "", fmt.Errorf("accessToken not found in credentials")
}

func extractToken(raw []byte) string {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err == nil {
		if tok, ok := data["accessToken"].(string); ok && tok != "" {
			return tok
		}
		for _, v := range data {
			if m, ok := v.(map[string]any); ok {
				if tok, ok := m["accessToken"].(string); ok && tok != "" {
					return tok
				}
			}
		}
	}
	re := regexp.MustCompile(`"accessToken"\s*:\s*"([^"]+)"`)
	if m := re.FindSubmatch(raw); m != nil {
		return string(m[1])
	}
	return ""
}

// ---- API ----

type payload struct {
	S  int  `json:"s"`
	SR int  `json:"sr"`
	W  int  `json:"w"`
	WR int  `json:"wr"`
	Ok bool `json:"ok"`
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func fetchUsage(token string, cfg config) (p *payload, retryAfter time.Duration) {
	req, _ := http.NewRequest("POST", apiURL, bytes.NewReader(apiBody))
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-code/2.1.5")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("API error: %v", err)
		return nil, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		wait := cfg.pollInterval
		if ra := resp.Header.Get("retry-after"); ra != "" {
			if secs, err := strconv.ParseFloat(ra, 64); err == nil {
				wait = time.Duration(secs) * time.Second
			}
		}
		if wait > maxRetryWait {
			wait = maxRetryWait
		}
		log.Printf("Rate limited. Retry after %.0fs", wait.Seconds())
		return nil, wait
	}
	if resp.StatusCode >= 400 {
		log.Printf("API HTTP %d", resp.StatusCode)
		return nil, 0
	}

	now := float64(time.Now().Unix())

	hdr := func(name string) string { return resp.Header.Get(name) }

	pct := func(util string) int {
		f, err := strconv.ParseFloat(strings.TrimSpace(util), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0
		}
		return clamp(int(math.Round(f*100)), 0, 100)
	}

	resetMin := func(ts string) int {
		f, err := strconv.ParseFloat(strings.TrimSpace(ts), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0
		}
		mins := (f - now) / 60.0
		if mins < 0 {
			return 0
		}
		return int(math.Round(mins))
	}

	return &payload{
		S:  pct(hdr("anthropic-ratelimit-unified-5h-utilization")),
		SR: resetMin(hdr("anthropic-ratelimit-unified-5h-reset")),
		W:  pct(hdr("anthropic-ratelimit-unified-7d-utilization")),
		WR: resetMin(hdr("anthropic-ratelimit-unified-7d-reset")),
		Ok: true,
	}, 0
}

// ---- BLE ----

var adapter = bluetooth.DefaultAdapter

func findDevice(cfg config) (bluetooth.ScanResult, error) {
	log.Printf("Scanning for '%s'...", cfg.deviceName)
	if err := adapter.Enable(); err != nil {
		return bluetooth.ScanResult{}, fmt.Errorf("enable adapter: %w", err)
	}

	found := make(chan bluetooth.ScanResult, 1)
	err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
		if r.LocalName() == cfg.deviceName {
			a.StopScan()
			// non-blocking: 複数回検出されても最初の1件だけ確定
			select {
			case found <- r:
			default:
			}
		}
	})
	if err != nil {
		return bluetooth.ScanResult{}, err
	}

	select {
	case r := <-found:
		log.Printf("Found: %s", r.Address)
		return r, nil
	case <-time.After(cfg.scanTimeout):
		adapter.StopScan() // タイムアウト時も必ずスキャンを停止
		return bluetooth.ScanResult{}, fmt.Errorf("device '%s' not found", cfg.deviceName)
	}
}

func run(cfg config) error {
	token, err := loadToken()
	if err != nil {
		return err
	}

	log.Printf("Config: device=%s poll=%s scan_timeout=%s",
		cfg.deviceName, cfg.pollInterval, cfg.scanTimeout)

	for {
		result, err := findDevice(cfg)
		if err != nil {
			log.Printf("Scan error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		dev, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			log.Printf("Connect error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Connected!")

		if err := runSession(&dev, token, cfg); err != nil {
			log.Printf("Session error: %v", err)
		}
		dev.Disconnect()
		log.Println("Disconnected. Reconnecting...")
	}
}

func mustUUID(s string) bluetooth.UUID {
	u, err := bluetooth.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

func runSession(dev *bluetooth.Device, token string, cfg config) error {
	svc, err := dev.DiscoverServices([]bluetooth.UUID{
		mustUUID("4c41555a-4465-7669-6365-000000000001"),
	})
	if err != nil || len(svc) == 0 {
		return fmt.Errorf("discover service: %w", err)
	}

	chars, err := svc[0].DiscoverCharacteristics([]bluetooth.UUID{
		mustUUID(rxUUID),
	})
	if err != nil || len(chars) == 0 {
		return fmt.Errorf("discover characteristic: %w", err)
	}
	rx := chars[0]

	var cached *payload
	for {
		p, retryAfter := fetchUsage(token, cfg)

		var send *payload
		switch {
		case p != nil:
			cached = p
			send = p
		case cached != nil:
			send = cached
			log.Printf("Using cached: %+v", *send)
		default:
			send = &payload{Ok: false}
		}

		data, _ := json.Marshal(send)
		if _, err := rx.WriteWithoutResponse(data); err != nil {
			return fmt.Errorf("BLE write: %w", err)
		}
		log.Printf("Sent: %s", data)

		wait := cfg.pollInterval
		if retryAfter > 0 {
			wait = retryAfter
			log.Printf("Waiting %.0fs before retry...", wait.Seconds())
		}
		time.Sleep(wait)
	}
}

func main() {
	log.SetFlags(log.Ltime)
	cfg := loadConfig()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}
