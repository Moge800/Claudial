#pragma once
#include <stdbool.h>

void ble_init(const char *device_name);
bool ble_is_connected();

// daemonから受信したデータ
struct BleData {
    int  session_pct;   // s
    int  week_pct;      // w
    int  session_reset; // sr (分)
    int  week_reset;    // wr (分)
    bool ok;
};

// 最新受信データを取得（未受信時は ok=false）
BleData ble_get_data();
