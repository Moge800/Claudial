#pragma once
#include <lvgl.h>

typedef enum {
    EDIT_SESSION = 0,
    EDIT_WEEK    = 1,
} edit_target_t;

void ui_init(int w, int h);
// stale=true のとき弱めの色で表示（レート制限中など前回cached値）
// stale=true renders in dimmed colors (e.g. rate-limited, showing previous cached value).
void ui_update(int session_pct, int week_pct, int session_limit, int week_limit, edit_target_t target, bool stale = false);
void ui_set_alert(bool active);    // true=赤フラッシュ, false=通常 / true=red flash, false=normal
void ui_set_offline(bool offline); // true=グレー+メッセージ, false=通常 / true=grey+message, false=normal
