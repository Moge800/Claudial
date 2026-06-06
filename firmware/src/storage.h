#pragma once
#include <Arduino.h>
#include <stdint.h>

void     storage_init();
// デバイス名（BLEアドバタイズ名）
void     storage_set_device_name(const char *name);
String   storage_get_device_name();
// 画面向き（0 or 2）
void     storage_set_rotation(uint8_t rotation);
uint8_t  storage_get_rotation();
// セッション/週間リミット
void     storage_set_session_limit(uint8_t pct);
uint8_t  storage_get_session_limit();
void     storage_set_week_limit(uint8_t pct);
uint8_t  storage_get_week_limit();
