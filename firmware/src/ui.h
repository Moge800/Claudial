#pragma once
#include <lvgl.h>

typedef enum {
    EDIT_SESSION = 0,
    EDIT_WEEK    = 1,
} edit_target_t;

void ui_init(int w, int h);
void ui_update(int session_pct, int week_pct, int session_limit, int week_limit, edit_target_t target);
void ui_set_alert(bool active);    // true=赤フラッシュ, false=通常
void ui_set_offline(bool offline); // true=グレー+メッセージ, false=通常
