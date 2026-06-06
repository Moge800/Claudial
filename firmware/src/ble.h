#pragma once
#include <stdbool.h>

void ble_init(const char *device_name);
bool ble_is_connected();

// daemonから受信したデータ / Data received from the daemon
struct BleData {
    int           session_pct;   // s
    int           week_pct;      // w
    int           session_reset; // sr (分) / sr (minutes)
    int           week_reset;    // wr (分) / wr (minutes)
    int           poll_interval; // pi (秒) daemonのポーリング間隔 / pi (seconds) daemon's poll interval
    bool          ok;
    bool          stale;         // st 値が古い（cachedフォールバック） / st value is stale (cached fallback)
    unsigned long received_ms;   // onWrite が呼ばれた millis() / millis() when onWrite was called
};

// 最新受信データを取得（未受信時は ok=false）
// Get the latest received data (ok=false when nothing received yet).
BleData ble_get_data();
