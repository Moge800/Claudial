#include <M5Unified.h>
#include <lvgl.h>
#include "ui.h"
#include "ble.h"
#include "storage.h"

// 画面向き: 0=USB下, 2=USB上（ケーブルが上から出るとき）
// 変更後は pio run -t upload で書き込み直してください
#ifndef DISPLAY_ROTATION
#define DISPLAY_ROTATION 2
#endif

// M5Dial ピン定義
static const int BUZZER_PIN  = 3;
static const int ENC_A_PIN   = 40;
static const int ENC_B_PIN   = 41;
static const int ENC_BTN_PIN = 42;

static volatile int enc_count = 0;

static int last_enc = 0;
static unsigned long last_lvgl_tick;  // setup() 末尾で millis() 初期化（初回 delta=0）
static unsigned long last_alarm_ms  = 0;
static unsigned long last_ble_ms    = 0;
static bool alert_flash = false;

static int session_limit;
static int week_limit;
static int session_pct = 0;
static int week_pct    = 0;
static edit_target_t edit_target = EDIT_SESSION;

// 警告状態
typedef enum { WARN_NONE, WARN_NEAR, WARN_LIMIT } warn_state_t;
static warn_state_t warn_state = WARN_NONE;
static bool muted = false;

void IRAM_ATTR enc_isr() {
    bool a = digitalRead(ENC_A_PIN);
    bool b = digitalRead(ENC_B_PIN);
    enc_count += (a == b) ? 1 : -1;
}

void beep(int freq, int ms) {
    tone(BUZZER_PIN, freq, ms);
    delay(ms);
    noTone(BUZZER_PIN);
}

void beep_near() {
    beep(1200, 80); delay(60); beep(1200, 80);
}

static warn_state_t calc_warn(int pct, int limit) {
    if (pct >= limit)             return WARN_LIMIT;
    if (pct >= max(0, limit - 5)) return WARN_NEAR;
    return WARN_NONE;
}

void setup() {
    storage_init();

    auto cfg = M5.config();
    M5.begin(cfg);
    M5.Display.setRotation(DISPLAY_ROTATION);

    pinMode(ENC_A_PIN,   INPUT_PULLUP);
    pinMode(ENC_B_PIN,   INPUT_PULLUP);
    pinMode(ENC_BTN_PIN, INPUT_PULLUP);
    attachInterrupt(digitalPinToInterrupt(ENC_A_PIN), enc_isr, CHANGE);

    session_limit = storage_get_session_limit();
    week_limit    = storage_get_week_limit();

    ui_init(M5.Display.width(), M5.Display.height());
    ui_update(session_pct, week_pct, session_limit, week_limit, edit_target);

    String devName = storage_get_device_name();
    ble_init(devName.c_str());

    last_lvgl_tick = millis();  // 初回 delta=0 にするため setup 末尾で初期化

    beep(1000, 80);
}


void loop() {
    M5.update();

    unsigned long now = millis();
    lv_tick_inc(now - last_lvgl_tick);
    last_lvgl_tick = now;
    lv_timer_handler();

    // BLEデータを500msごとに反映
    if (now - last_ble_ms >= 500) {
        last_ble_ms = now;
        BleData d = ble_get_data();
        if (d.ok) {
            session_pct = constrain(d.session_pct, 0, 100);
            week_pct    = constrain(d.week_pct,    0, 100);
            ui_update(session_pct, week_pct, session_limit, week_limit, edit_target);
        }
    }

    // エンコーダでリミット調整（ISR競合防止のため割り込み停止してスナップショット）
    noInterrupts();
    int enc = enc_count;
    interrupts();
    if (enc != last_enc) {
        int delta = enc - last_enc;
        last_enc = enc;

        // 画面180°回転時はエンコーダ方向も反転
        int adj = (DISPLAY_ROTATION == 2) ? -delta : delta;
        if (edit_target == EDIT_SESSION) {
            session_limit = constrain(session_limit + adj, 0, 100);
            storage_set_session_limit(session_limit);
        } else {
            week_limit = constrain(week_limit + adj, 0, 100);
            storage_set_week_limit(week_limit);
        }
        ui_update(session_pct, week_pct, session_limit, week_limit, edit_target);
        beep(adj > 0 ? 1200 : 800, 20);
    }

    // タッチ: 消音 or 編集対象切り替え
    if (M5.Touch.getCount() > 0) {
        auto t = M5.Touch.getDetail();
        if (t.wasPressed()) {
            if (warn_state == WARN_LIMIT && !muted) {
                muted = true;
                ui_set_alert(false);
                noTone(BUZZER_PIN);
            } else {
                edit_target = (edit_target == EDIT_SESSION) ? EDIT_WEEK : EDIT_SESSION;
                ui_update(session_pct, week_pct, session_limit, week_limit, edit_target);
                beep(1500, 40);
            }
        }
    }

    // 警告レベル判定
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

    // WARN_LIMIT: 500ms周期でフラッシュ＋ビープ
    if (warn_state == WARN_LIMIT && !muted) {
        if (now - last_alarm_ms >= 500) {
            last_alarm_ms = now;
            alert_flash = !alert_flash;
            ui_set_alert(alert_flash);
            if (alert_flash) tone(BUZZER_PIN, 880);
            else             noTone(BUZZER_PIN);
        }
    }
}
