// Claudial daemon — Claude Code usage monitor via BLE.
//
// Usage data is read from rate-limit headers returned by the Anthropic API,
// following the approach used by Clawdmeter (github.com/HermannBjorgvin/Clawdmeter).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	rxUUID       = "29590732-a70c-4ea9-a739-000000000002"
	maxRetryWait = 5 * time.Minute
)

// retryExpired は fetchUsage が 401 を返すときの専用センチネル値。
// retryExpired is the sentinel returned by fetchUsage on HTTP 401.
// 負の Retry-After 日時と区別するため time.Duration の最小値を使う。
// Using MinInt64 distinguishes it from a legitimate negative Retry-After date.
const retryExpired = time.Duration(-1 << 63)

// errTokenExpired は401時にrunSession→runへ伝えるためのセンチネルエラー。
// errTokenExpired is the sentinel error propagated from runSession to run on HTTP 401.
var errTokenExpired = errors.New("token expired")

var apiBody = []byte(`{"model":"claude-haiku-4-5-20251001","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)

// ---- config ----

type config struct {
	deviceName   string
	pollInterval time.Duration
	scanTimeout  time.Duration
}

func loadConfig() config {
	// 実行ファイルと同じディレクトリの .env を読む（なければ無視）
	// Load .env from the executable's directory (ignored if absent).
	if exe, err := os.Executable(); err == nil {
		_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
	}
	// カレントディレクトリの .env も読む（開発時用）
	// Also load .env from the current directory (for development).
	_ = godotenv.Load()

	cfg := config{
		deviceName:   "Claudial",
		pollInterval: 60 * time.Second,
		scanTimeout:  15 * time.Second,
	}
	if v := os.Getenv("CLAUDIAL_DEVICE_NAME"); v != "" {
		cfg.deviceName = v
	}
	if v := os.Getenv("CLAUDIAL_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.pollInterval = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("CLAUDIAL_SCAN_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.scanTimeout = time.Duration(n) * time.Second
		}
	}
	return cfg
}

// ---- credentials ----

func loadToken() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	candidates := []string{
		filepath.Join(home, ".claude", ".credentials.json"),
	}
	// Windows 固有のフォールバックパス（環境変数が未設定の環境では追加しない）
	// Windows-specific fallback paths (skipped when the env var is unset).
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		candidates = append(candidates, filepath.Join(v, "Claude", ".credentials.json"))
	}
	if v := os.Getenv("APPDATA"); v != "" {
		candidates = append(candidates, filepath.Join(v, "Claude", ".credentials.json"))
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
	PI int  `json:"pi"` // ポーリング間隔（秒）— M5側のタイムアウト算出用 / poll interval (sec) — used by M5 to derive its timeout
	Ok bool `json:"ok"` // 取得成功フラグ / fetch success flag
	St bool `json:"st"` // 値が古い（cachedフォールバック）/ value is stale (cached fallback)
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

func fetchUsage(ctx context.Context, token string, cfg config) (p *payload, retryAfter time.Duration) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(apiBody))
	if err != nil {
		log.Printf("request build error: %v", err)
		return nil, 0
	}
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-code/2.1.5")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// コンテキストキャンセル（Quit）は正常終了 — エラーログを出さない。
		// Context cancellation (Quit) is a clean exit — don't log as an error.
		if ctx.Err() != nil {
			return nil, 0
		}
		log.Printf("API error: %v", err)
		return nil, 0
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) // keep-alive のためボディを読み切る / drain body to keep the connection alive
		resp.Body.Close()
	}()

	if resp.StatusCode == 429 {
		wait := cfg.pollInterval
		if ra := resp.Header.Get("retry-after"); ra != "" {
			// Retry-After は秒数またはHTTP-date形式 / Retry-After can be delta-seconds or HTTP-date.
			if secs, err := strconv.ParseFloat(ra, 64); err == nil {
				wait = time.Duration(secs) * time.Second
			} else if t, err := http.ParseTime(ra); err == nil {
				// 過去日時は負値になるので0にclamp（401センチネルと混同しないため）
				// Clamp past dates to 0 to avoid confusion with the 401 sentinel.
				if d := time.Until(t); d > 0 {
					wait = d
				}
			}
		}
		if wait > maxRetryWait {
			wait = maxRetryWait
		}
		log.Printf("Rate limited. Retry after %.0fs", wait.Seconds())
		return nil, wait
	}
	if resp.StatusCode == 401 {
		log.Printf("API HTTP 401: token expired — run 'claude login' to refresh, then daemon will retry automatically")
		return nil, retryExpired // 専用センチネルで401を通知 / dedicated sentinel for 401
	}
	if resp.StatusCode >= 400 {
		log.Printf("API HTTP %d", resp.StatusCode)
		return nil, 0
	}

	now := float64(time.Now().Unix())

	hdr := func(name string) string { return resp.Header.Get(name) }

	pct := func(util string) (int, bool) {
		f, err := strconv.ParseFloat(strings.TrimSpace(util), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		// float空間でクランプしてからintへ変換 — 極端な値でのオーバーフローを防ぐ。
		// Clamp in float space before converting to int to avoid overflow on extreme values.
		return int(math.Round(math.Max(0, math.Min(1, f)) * 100)), true
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

	sUtil := hdr("anthropic-ratelimit-unified-5h-utilization")
	wUtil := hdr("anthropic-ratelimit-unified-7d-utilization")
	// 主要ヘッダーが欠落または不正値の場合はキャッシュへフォールバック
	// Treat missing or unparseable key headers as a fetch failure.
	if sUtil == "" || wUtil == "" {
		log.Printf("Rate-limit headers missing in 2xx response — falling back to cache")
		return nil, 0
	}
	sPct, sOk := pct(sUtil)
	wPct, wOk := pct(wUtil)
	if !sOk || !wOk {
		log.Printf("Rate-limit headers unparseable (s=%q w=%q) — falling back to cache", sUtil, wUtil)
		return nil, 0
	}
	return &payload{
		S:  sPct,
		SR: resetMin(hdr("anthropic-ratelimit-unified-5h-reset")),
		W:  wPct,
		WR: resetMin(hdr("anthropic-ratelimit-unified-7d-reset")),
		Ok: true,
	}, 0
}

// ---- BLE ----

var adapter = bluetooth.DefaultAdapter

func findDevice(ctx context.Context, cfg config) (bluetooth.ScanResult, error) {
	log.Printf("Scanning for '%s'...", cfg.deviceName)

	found := make(chan bluetooth.ScanResult, 1)
	scanErr := make(chan error, 1)

	// adapter.Scan() はブロッキング呼び出しのため goroutine で実行する。
	// adapter.Scan() blocks until StopScan() is called, so run it in a goroutine.
	go func() {
		err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
			if r.LocalName() == cfg.deviceName {
				a.StopScan()
				// non-blocking: 複数回検出されても最初の1件だけ確定 / keep only the first hit
				select {
				case found <- r:
				default:
				}
			}
		})
		if err != nil {
			scanErr <- err
		}
	}()

	// タイムアウト・キャンセル側から StopScan() を呼ぶことで goroutine を解放する。
	// Call StopScan() from the timeout/cancel arm to unblock the scan goroutine.
	select {
	case r := <-found:
		log.Printf("Found: %s", r.Address)
		return r, nil
	case err := <-scanErr:
		return bluetooth.ScanResult{}, err
	case <-time.After(cfg.scanTimeout):
		adapter.StopScan()
		// タイムアウト直前にコールバックが結果を詰めていた場合を救う / Drain any result queued just before timeout.
		select {
		case r := <-found:
			log.Printf("Found (at timeout boundary): %s", r.Address)
			return r, nil
		default:
		}
		return bluetooth.ScanResult{}, fmt.Errorf("device '%s' not found", cfg.deviceName)
	case <-ctx.Done():
		adapter.StopScan()
		// キャンセル時はfoundを捨てる — 拾うとConnect（キャンセル不能）へ進んでしまう。
		// On cancel, discard any queued result — returning it would enter Connect, which ignores ctx.
		return bluetooth.ScanResult{}, ctx.Err()
	}
}

func run(ctx context.Context, cfg config) error {
	log.Printf("Config: device=%s poll=%s scan_timeout=%s",
		cfg.deviceName, cfg.pollInterval, cfg.scanTimeout)

	// adapter.Enable() は呼び出し側が一度だけ行う。run() は再開ループから
	// 複数回呼ばれうるため、ここで Enable すると Windows で再有効化エラーになる。
	// The caller enables the adapter once — run() may be called repeatedly by the
	// restart loop, and re-Enable would error on Windows.

	var cached *payload  // セッションをまたいで最後の正常値を保持 / keep last good value across sessions
	for {
		// キャンセル済みなら終了 / Exit if context was cancelled (e.g. tray Quit).
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		result, err := findDevice(ctx, cfg)
		if err != nil {
			if ctx.Err() != nil {
				return nil // キャンセルによる終了は正常 / clean exit on cancel
			}
			log.Printf("Scan error: %v. Retrying in 5s...", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return nil
			}
			continue
		}

		// findDevice成功後でもキャンセル済みならConnectをスキップする。
		// Connect は ctx 非対応のため、ここでガードしないとQuitが長引く。
		// Guard against ctx cancellation that arrived after findDevice returned —
		// Connect ignores ctx, so skipping it here keeps Quit responsive.
		if ctx.Err() != nil {
			return nil
		}

		// tinygo bluetooth の Connect/DiscoverServices はコンテキスト非対応のため、
		// Quit 時はこれらの完了を待つ必要がある（通常数秒以内）。
		// Connect/DiscoverServices do not support context cancellation in tinygo bluetooth;
		// Quit will wait for them to complete (normally within a few seconds).
		dev, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("Connect error: %v. Retrying in 5s...", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return nil
			}
			continue
		}
		log.Println("Connected!")

		token, err := loadToken()
		if err != nil {
			if ctx.Err() != nil {
				dev.Disconnect()
				return nil
			}
			// RX characteristicがまだ未取得のため ok:false は送れない。
			// Cannot send ok:false here — RX characteristic not yet discovered.
			// デバイスはBLEタイムアウト後にオフライン画面を表示する。
			// Device will show offline screen after BLE timeout.
			log.Printf("Token load error: %v. Retrying in 5s...", err)
			dev.Disconnect()
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return nil
			}
			continue
		}
		if err := runSession(ctx, &dev, token, cfg, &cached); err != nil {
			dev.Disconnect()
			if ctx.Err() != nil {
				return nil // キャンセルによるセッション終了は正常 / clean exit on cancel
			}
			if errors.Is(err, errTokenExpired) {
				// 401 は再接続してもすぐ失敗するため、60秒待ってからリトライ。
				// 401 will fail again immediately — wait 60s before reconnecting.
				// time.After はキャンセル時にタイマーが残るため NewTimer を使う。
				// Use NewTimer so we can stop it on cancel, avoiding timer leaks.
				log.Printf("Token expired. Waiting 60s before retry — run 'claude login' to refresh.")
				t401 := time.NewTimer(60 * time.Second)
				select {
				case <-t401.C:
				case <-ctx.Done():
					t401.Stop()
					return nil
				}
			} else {
				log.Printf("Session error: %v", err)
			}
		} else {
			dev.Disconnect()
		}
		log.Println("Disconnected. Reconnecting...")
	}
}

// sendError は ok:false をデバイスに送信してオフライン画面を表示させる。
// sendError sends ok:false to the device to trigger the offline screen.
func sendError(rx bluetooth.DeviceCharacteristic) {
	data, _ := json.Marshal(&payload{Ok: false})
	_, _ = rx.WriteWithoutResponse(data)
	log.Printf("Sent error: %s", data)
}

func mustUUID(s string) bluetooth.UUID {
	u, err := bluetooth.ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

func runSession(ctx context.Context, dev *bluetooth.Device, token string, cfg config, cached **payload) error {
	// Connect完了後にキャンセルが到着していた場合はDiscoverServices（非対応）の前に終了する。
	// Exit before DiscoverServices (non-cancellable) if ctx was cancelled after Connect returned.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// WinRT では Connect 直後に discover が失敗することがある → 最大3回リトライ
	// On WinRT, discovery can fail right after Connect → retry up to 3 times.
	var svc []bluetooth.DeviceService
	for attempt := 1; attempt <= 3; attempt++ {
		select {
		case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		// ウェイト完了後にキャンセルが来ていた場合もDiscoverServices（非対応）の前に終了する。
		// Also guard after the wait fires — DiscoverServices is non-cancellable.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var discErr error
		svc, discErr = dev.DiscoverServices([]bluetooth.UUID{
			mustUUID("29590732-a70c-4ea9-a739-000000000001"),
		})
		if discErr == nil && len(svc) > 0 {
			break
		}
		log.Printf("DiscoverServices attempt %d failed: %v", attempt, discErr)
		// status 2 = WinRT AsyncStatus::Canceled — retrying causes a CGO crash.
		// Return immediately and let the outer loop reconnect.
		if discErr != nil && strings.HasSuffix(discErr.Error(), "status 2") {
			return fmt.Errorf("discover service: canceled (status 2), reconnecting: %w", discErr)
		}
		if attempt == 3 {
			if discErr != nil {
				return fmt.Errorf("discover service: %w", discErr)
			}
			return fmt.Errorf("discover service: no matching service found")
		}
	}

	chars, err := svc[0].DiscoverCharacteristics([]bluetooth.UUID{
		mustUUID(rxUUID),
	})
	if err != nil {
		return fmt.Errorf("discover characteristic: %w", err)
	}
	if len(chars) == 0 {
		return fmt.Errorf("discover characteristic: RX characteristic not found")
	}
	rx := chars[0]

	// 接続直後、前セッションの cached があればすぐ送って No data を解消
	// Right after connecting, send the previous session's cached value to clear "No data".
	// キャンセル後はBLE書き込みをスキップ / Skip BLE write if already cancelled.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if *cached != nil {
		(*cached).PI = int(cfg.pollInterval.Seconds())
		(*cached).St = true // 再接続直後はまだフェッチ前 = 古い値 / not yet re-fetched = stale
		if data, err := json.Marshal(*cached); err == nil {
			if _, err := rx.WriteWithoutResponse(data); err == nil {
				log.Printf("Sent cached on connect: %s", data)
			}
		}
	}

	for {
		p, retryAfter := fetchUsage(ctx, token, cfg)

		// コンテキストキャンセルはfetchUsageがnil,0を返す — BLE書き込みへ進まず即リターン。
		// If ctx was cancelled, fetchUsage returns nil,0 — exit before any BLE write.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// retryExpired は 401 シグナル → ok:false を送ってセッション終了
		// retryExpired is the 401 signal → send ok:false then end session.
		if retryAfter == retryExpired {
			sendError(rx)
			return errTokenExpired
		}

		var send *payload
		switch {
		case p != nil:
			p.St = false // 取得成功 = 最新 / fetch succeeded = fresh
			*cached = p  // セッションをまたいで保持 / keep across sessions
			send = p
		case *cached != nil:
			send = *cached
			send.St = true // フォールバック = 古い値 / fallback = stale value
			log.Printf("Using cached (stale): %+v", *send)
		default:
			send = &payload{Ok: false}
		}

		// 実際の待機時間を先に確定してからPIに反映（rate limit中でもデバイスのタイムアウトがずれない）
		// Compute actual wait first so PI reflects the real sleep interval,
		// preventing the device from showing offline during long Retry-After backoffs.
		wait := cfg.pollInterval
		if retryAfter > 0 {
			wait = retryAfter
			log.Printf("Waiting %.0fs before retry...", wait.Seconds())
		}

		send.PI = int(wait.Seconds())
		data, _ := json.Marshal(send)
		if _, err := rx.WriteWithoutResponse(data); err != nil {
			return fmt.Errorf("BLE write: %w", err)
		}
		log.Printf("Sent: %s", data)
		// キャンセル時はポーリングスリープを中断 / Interrupt poll sleep on context cancel.
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	ensureSingleInstance() // 多重起動を防ぐ / Exit if another instance is already running.
	cfg := loadConfig()
	setupLogFile()
	runWithTray(cfg)
}

// setupLogFile はexeと同じフォルダにdaemon.logを作成しlogの出力先に追加する。
// setupLogFile creates daemon.log next to the executable and tees log output to it.
func setupLogFile() {
	// os.Executable()失敗時はカレントディレクトリの daemon.log にフォールバック。
	// tray.logPath() と同じフォールバックパスを使うことで Open Log が常に有効になる。
	// Fall back to "daemon.log" in cwd on failure, matching tray.logPath()'s fallback.
	logFile := "daemon.log"
	if exe, err := os.Executable(); err == nil {
		logFile = filepath.Join(filepath.Dir(exe), "daemon.log")
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Printf("Cannot open log file %s: %v", logFile, err)
		return
	}
	// ファイルを先にしてstdout失敗時もファイル書き込みを保証 / File first: ensures log is written even if stdout is invalid (windowsgui).
	log.SetOutput(io.MultiWriter(f, os.Stdout))
	log.Printf("Logging to %s", logFile)
}
