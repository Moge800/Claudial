#include <M5Unified.h>
#include <lvgl.h>
#include "ui.h"
#include "ble.h"
#include "storage.h"

// 画面向き: 0=USB下, 2=USB上 — NVSから読み取り、デフォルト0
// Screen orientation: 0=USB at bottom, 2=USB at top — read from NVS, default 0
// storage_set_rotation(2) で書き換え可能（要再起動、reflash不要）
// Can be changed via storage_set_rotation(2) without reflashing (reboot required).

// M5Dial ピン定義 / M5Dial pin definitions
static const int BUZZER_PIN  = 3;
static const int ENC_A_PIN   = 40;
static const int ENC_B_PIN   = 41;
static const int ENC_BTN_PIN = 42;

static volatile int enc_count = 0;

static int last_enc = 0;
static unsigned long last_lvgl_tick;  // setup() 末尾で millis() 初期化（初回 delta=0） / initialized at end of setup() so first delta=0
static unsigned long last_alarm_ms  = 0;
static unsigned long last_ble_ms    = 0;
static bool          alert_flash      = false;
static bool          is_offline       = false;
// daemonから受け取るポーリング間隔(pi)で算出: pi×2+30秒。
// Timeout derived from daemon's poll interval (pi): pi*2+30s.
// pi未受信（pi=0 / 旧daemon）はこのデフォルトを使用
// Falls back to this default when pi is not received (pi=0 / old daemon).
static const unsigned long BLE_TIMEOUT_DEFAULT_MS = 150000UL;

static uint8_t display_rotation;  // NVSから読み取り / read from NVS
static int session_limit;
static int week_limit;
static int  session_pct = 0;
static int  week_pct    = 0;
static bool last_stale  = false;  // BLE受信時のstale状態を保持 / persist stale flag between BLE updates
static edit_target_t edit_target = EDIT_SESSION;

// 警告状態 / Warning state
typedef enum { WARN_NONE, WARN_NEAR, WARN_LIMIT } warn_state_t;
static warn_state_t warn_state = WARN_NONE;
static bool muted = false;

void IRAM_ATTR enc_isr() {
    // digitalRead はフラッシュキャッシュ無効時にISRセーフでないため gpio_get_level を使用
    // Use gpio_get_level instead of digitalRead — ISR-safe when flash cache is disabled.
    bool a = gpio_get_level((gpio_num_t)ENC_A_PIN);
    bool b = gpio_get_level((gpio_num_t)ENC_B_PIN);
    enc_count += (a == b) ? 1 : -1;
}

// tone(pin, freq, duration) は duration 後に自動停止するため delay 不要（非ブロッキング）
// tone() auto-stops after duration, so no delay needed (non-blocking).
void beep(int freq, int ms) {
    tone(BUZZER_PIN, freq, ms);
}

void beep_near() {
    // ピピッ: 1回目鳴らして少し待ち、2回目（delay は短くブロックを最小化）
    // Two short beeps; keep delay short to minimize blocking.
    tone(BUZZER_PIN, 1200, 80);
    delay(140);  // 80ms音 + 60ms間隔 / 80ms tone + 60ms gap
    tone(BUZZER_PIN, 1200, 80);
}

static warn_state_t calc_warn(int pct, int limit) {
    if (pct >= limit)             return WARN_LIMIT;
    if (pct >= max(0, limit - 5)) return WARN_NEAR;
    return WARN_NONE;
}

void setup() {
    Serial.begin(115200);
    storage_init();

    auto cfg = M5.config();
    M5.begin(cfg);
    display_rotation = storage_get_rotation();  // NVSから読み取り / read from NVS
    M5.Display.setRotation(display_rotation);

    pinMode(ENC_A_PIN,   INPUT_PULLUP);
    pinMode(ENC_B_PIN,   INPUT_PULLUP);
    pinMode(ENC_BTN_PIN, INPUT_PULLUP);
    attachInterrupt(digitalPinToInterrupt(ENC_A_PIN), enc_isr, CHANGE);

    // NVS値を[1,100]にクランプ（破損や旧スキーマによる0値でWARN_LIMITに入り込むのを防ぐ）
    // Clamp NVS values to [1,100] to guard against corruption or schema changes yielding 0.
    session_limit = constrain((int)storage_get_session_limit(), 1, 100);
    week_limit    = constrain((int)storage_get_week_limit(),    1, 100);

    ui_init(M5.Display.width(), M5.Display.height());
    ui_update(session_pct, week_pct, session_limit, week_limit, edit_target);

    String devName = storage_get_device_name();
    ble_init(devName.c_str());

    last_lvgl_tick = millis();  // 初回 delta=0 にするため setup 末尾で初期化 / init at end of setup so first delta=0

    beep(1000, 80);
}


void loop() {
    M5.update();

    unsigned long now = millis();
    lv_tick_inc(now - last_lvgl_tick);
    last_lvgl_tick = now;
    uint32_t sleep_ms = lv_timer_handler();  // 次タイマーまでの推奨待機時間を返す / returns ms until next timer
    // CPUを100%使わないよう最低1ms・最大10msのyield / yield 1-10ms to avoid pegging the CPU
    delay(constrain(sleep_ms, 1, 10));

    // BLEデータを500msごとに反映 / Apply BLE data every 500ms
    if (now - last_ble_ms >= 500) {
        last_ble_ms = now;
        BleData d = ble_get_data();

        bool has_data = (d.received_ms > 0);
        // daemonのポーリング間隔から動的にタイムアウト算出（pi×2+30秒）
        // Derive timeout dynamically from daemon's poll interval (pi*2+30s).
        unsigned long timeout_ms = (d.poll_interval > 0)
            ? (unsigned long)d.poll_interval * 2000UL + 30000UL
            : BLE_TIMEOUT_DEFAULT_MS;
        // 未受信時は起動からの経過時間を使う。timeout_ms自体がブート猶予期間になる。
        // Use time-since-boot when no packet received; timeout_ms itself acts as the boot grace period.
        unsigned long elapsed = has_data ? (now - d.received_ms) : now;
        bool ble_timeout = (elapsed >= timeout_ms);
        bool daemon_err  = has_data && !d.ok;

        if (daemon_err || ble_timeout) {
            // daemon が ok:false を送った → 即オフライン
            // daemon sent ok:false → go offline immediately
            // BLE受信が途絶えた → タイムアウトでオフライン
            // BLE reception stopped → go offline on timeout
            if (!is_offline) {
                is_offline = true;
                ui_set_offline(true);
            }
        } else if (has_data && d.ok) {
            // ok:true かつ BLE受信あり → 正常 / ok:true with received data → normal
            session_pct = constrain(d.session_pct, 0, 100);
            week_pct    = constrain(d.week_pct,    0, 100);
            last_stale  = d.stale;  // 次回のUI更新でも使えるよう保持 / keep for subsequent UI calls
            if (is_offline) {
                is_offline = false;
                ui_set_offline(false);
            }
            // stale=true は前回cached値（レート制限中など）/ stale=true means cached value (e.g. rate-limited)
            ui_update(session_pct, week_pct, session_limit, week_limit, edit_target, last_stale);
        }
        // has_data==false のときは何もしない（初回接続待ち）
        // Do nothing while has_data==false (waiting for first connection).
    }

    // エンコーダでリミット調整（ISR競合防止のため割り込み停止してスナップショット）
    // Adjust limit via encoder (disable interrupts to snapshot, avoiding ISR race).
    // NVS書き込みは1秒無操作後にデバウンスして実行（フラッシュ書き込み寿命を保護）
    // NVS writes are debounced: save only after 1s of inactivity to protect flash endurance.
    static unsigned long last_enc_change_ms  = 0;
    static bool          session_limit_dirty = false;
    static bool          week_limit_dirty    = false;
    noInterrupts();
    int enc = enc_count;
    interrupts();
    if (enc != last_enc) {
        int delta = enc - last_enc;
        last_enc = enc;

        // 画面向きに関わらず常に反転（ハードウェアの配線によりCW=負のdelta）
        // Always invert: encoder hardware produces negative delta for clockwise rotation.
        int adj = -delta;
        if (edit_target == EDIT_SESSION) {
            // 最小値1: 0にするとpct>=0が常に真でWARN_LIMITが解除できなくなる
            // Min 1: limit=0 would make pct>=limit always true, trapping in WARN_LIMIT.
            session_limit = constrain(session_limit + adj, 1, 100);
            session_limit_dirty = true;
        } else {
            week_limit = constrain(week_limit + adj, 1, 100);
            week_limit_dirty = true;
        }
        last_enc_change_ms = now;
        ui_update(session_pct, week_pct, session_limit, week_limit, edit_target, last_stale);
        beep(adj > 0 ? 1200 : 800, 20);
    }
    // 1秒操作なしでNVSに保存（変更したlimitのみ書き込み）
    // Save to NVS after 1s of inactivity — write only the changed limit.
    if ((session_limit_dirty || week_limit_dirty) && (now - last_enc_change_ms >= 1000)) {
        if (session_limit_dirty) { storage_set_session_limit(session_limit); session_limit_dirty = false; }
        if (week_limit_dirty)    { storage_set_week_limit(week_limit);    week_limit_dirty    = false; }
    }

    // タッチ: 消音 or 編集対象切り替え、長押しで画面反転 / Touch: mute, switch edit target, long-press to flip screen
    static unsigned long touch_start_ms = 0;
    static bool          long_press_fired = false;
    if (M5.Touch.getCount() > 0) {
        auto t = M5.Touch.getDetail();
        if (t.wasPressed()) {
            touch_start_ms   = now;
            long_press_fired = false;
        }
        // 長押し判定（1秒）/ Long-press detection (1 second)
        if (!long_press_fired && touch_start_ms > 0 &&
            (now - touch_start_ms >= 1000)) {
            long_press_fired = true;
            // 未保存のリミット変更をflushしてから再起動（デバウンス中の設定を失わないため）
            // Flush any pending limit save before rebooting so debounced changes are not lost.
            if (session_limit_dirty) { storage_set_session_limit(session_limit); session_limit_dirty = false; }
            if (week_limit_dirty)    { storage_set_week_limit(week_limit);    week_limit_dirty    = false; }
            // rotation トグル → NVS保存 → 再起動 / toggle rotation, save to NVS, reboot
            uint8_t new_rot = (display_rotation == 0) ? 2 : 0;
            storage_set_rotation(new_rot);
            beep(800, 300);
            delay(350);
            ESP.restart();
        }
    } else {
        // 指を離したとき、長押しでなければ短押し処理 / On release, handle short-press only if long-press didn't fire
        if (touch_start_ms > 0 && !long_press_fired) {
            if (warn_state == WARN_LIMIT && !muted) {
                muted = true;
                ui_set_alert(false);
                noTone(BUZZER_PIN);
            } else {
                edit_target = (edit_target == EDIT_SESSION) ? EDIT_WEEK : EDIT_SESSION;
                ui_update(session_pct, week_pct, session_limit, week_limit, edit_target, last_stale);
                beep(1500, 40);
            }
        }
        touch_start_ms = 0;
    }

    // 警告レベル判定 / Determine warning level
    // オフライン中は警告を抑制してブザーも止める / Suppress all warnings while offline
    if (is_offline) {
        if (warn_state != WARN_NONE) {
            warn_state = WARN_NONE;
            muted = false;
            ui_set_alert(false);
            noTone(BUZZER_PIN);
        }
    } else {
        warn_state_t ws = max(calc_warn(session_pct, session_limit),
                              calc_warn(week_pct, week_limit));
        if (ws > warn_state) {
            muted = false;
            if (ws == WARN_NEAR) beep_near();
        }
        if (ws < warn_state) {
            muted = false;
            ui_set_alert(false);
            noTone(BUZZER_PIN);
        }
        warn_state = ws;
    } // end !is_offline

    // WARN_LIMIT: 500ms周期でフラッシュ＋ビープ / WARN_LIMIT: flash + beep every 500ms
    if (!is_offline && warn_state == WARN_LIMIT && !muted) {
        if (now - last_alarm_ms >= 500) {
            last_alarm_ms = now;
            alert_flash = !alert_flash;
            ui_set_alert(alert_flash);
            if (alert_flash) tone(BUZZER_PIN, 880);
            else             noTone(BUZZER_PIN);
        }
    }
}
