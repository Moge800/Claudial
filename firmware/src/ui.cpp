#include "ui.h"
#include <M5Unified.h>

static lv_obj_t *arc_week;
static lv_obj_t *arc_session;
static lv_obj_t *arc_limit_session;  // 内円マーカー（セッション用） / inner-ring marker (session)
static lv_obj_t *arc_limit_week;     // 外円マーカー（週間用） / outer-ring marker (weekly)
static lv_obj_t *label_session;
static lv_obj_t *label_week;
static lv_obj_t *label_limit;
static lv_obj_t *label_offline;      // オフライン時メッセージ / message shown when offline

static lv_display_t *disp;
static lv_color_t    draw_buf_data[240 * 20];

static void flush_cb(lv_display_t *d, const lv_area_t *area, uint8_t *px_map) {
    int w = area->x2 - area->x1 + 1;
    int h = area->y2 - area->y1 + 1;
    M5.Display.startWrite();
    M5.Display.setAddrWindow(area->x1, area->y1, w, h);
    M5.Display.writePixels((lgfx::rgb565_t *)px_map, w * h);
    M5.Display.endWrite();
    lv_display_flush_ready(d);
}

static lv_obj_t *make_arc(lv_obj_t *parent, int diam, lv_color_t color, int width) {
    lv_obj_t *a = lv_arc_create(parent);
    lv_obj_set_size(a, diam, diam);
    lv_obj_center(a);
    lv_arc_set_bg_angles(a, 0, 360);
    lv_arc_set_angles(a, 0, 0);
    lv_arc_set_rotation(a, 270);
    lv_obj_remove_style(a, NULL, LV_PART_KNOB);
    lv_obj_remove_flag(a, LV_OBJ_FLAG_CLICKABLE);

    lv_obj_set_style_arc_width(a, width, LV_PART_INDICATOR);
    lv_obj_set_style_arc_color(a, color, LV_PART_INDICATOR);
    lv_obj_set_style_arc_width(a, width, LV_PART_MAIN);
    lv_obj_set_style_arc_color(a, lv_color_hex(0x222222), LV_PART_MAIN);
    lv_obj_set_style_bg_opa(a, LV_OPA_TRANSP, LV_PART_MAIN);
    return a;
}

void ui_init(int w, int h) {
    lv_init();

    disp = lv_display_create(w, h);
    lv_display_set_flush_cb(disp, flush_cb);
    lv_display_set_buffers(disp, draw_buf_data, NULL,
                           sizeof(draw_buf_data), LV_DISPLAY_RENDER_MODE_PARTIAL);

    lv_obj_t *scr = lv_screen_active();
    lv_obj_set_style_bg_color(scr, lv_color_black(), 0);

    // 外円: 週間 / 青 / 220px / 幅12 / Outer ring: weekly / blue / 220px / width 12
    arc_week          = make_arc(scr, 220, lv_color_hex(0x0080FF), 12);
    // 内円: セッション / 緑 / 170px / 幅12 / Inner ring: session / green / 170px / width 12
    arc_session       = make_arc(scr, 170, lv_color_hex(0x00CC44), 12);
    // セッションリミットマーカー: 内円寄り / 196px / 幅4 / Session limit marker: near inner ring / 196px / width 4
    arc_limit_session = make_arc(scr, 196, lv_color_hex(0xFF4400), 4);
    // 週間リミットマーカー: 外円寄り / 236px / 幅4 / Weekly limit marker: near outer ring / 236px / width 4
    arc_limit_week    = make_arc(scr, 236, lv_color_hex(0xFF4400), 4);

    // セッション% 大ラベル / Session % large label
    label_session = lv_label_create(scr);
    lv_obj_set_style_text_font(label_session, &lv_font_montserrat_48, 0);
    lv_obj_set_style_text_color(label_session, lv_color_white(), 0);
    lv_label_set_text(label_session, "0%");
    lv_obj_align(label_session, LV_ALIGN_CENTER, 0, -20);

    // 週間% 小ラベル / Weekly % small label
    label_week = lv_label_create(scr);
    lv_obj_set_style_text_font(label_week, &lv_font_montserrat_16, 0);
    lv_obj_set_style_text_color(label_week, lv_color_hex(0x0080FF), 0);
    lv_label_set_text(label_week, "week 0%");
    lv_obj_align(label_week, LV_ALIGN_CENTER, 0, 20);

    // リミット & 編集対象ラベル / Limit & edit-target label
    label_limit = lv_label_create(scr);
    lv_obj_set_style_text_font(label_limit, &lv_font_montserrat_16, 0);
    lv_obj_set_style_text_color(label_limit, lv_color_hex(0xFF4400), 0);
    lv_label_set_text(label_limit, "! S:80%");
    lv_obj_align(label_limit, LV_ALIGN_CENTER, 0, 45);

    // オフライン時メッセージ（通常は非表示） / Offline message (hidden by default)
    label_offline = lv_label_create(scr);
    lv_obj_set_style_text_font(label_offline, &lv_font_montserrat_16, 0);
    lv_obj_set_style_text_color(label_offline, lv_color_white(), 0);
    lv_label_set_text(label_offline, "No data\nCheck daemon\nor: claude login");
    lv_obj_set_style_text_align(label_offline, LV_TEXT_ALIGN_CENTER, 0);
    lv_obj_align(label_offline, LV_ALIGN_CENTER, 0, 0);
    lv_obj_add_flag(label_offline, LV_OBJ_FLAG_HIDDEN);
}

void ui_set_alert(bool active) {
    lv_obj_t *scr = lv_screen_active();
    lv_obj_set_style_bg_color(scr, active ? lv_color_hex(0xFF0000) : lv_color_black(), 0);
}

void ui_set_offline(bool offline) {
    lv_obj_t *scr = lv_screen_active();
    lv_obj_set_style_bg_color(scr, offline ? lv_color_hex(0x333333) : lv_color_black(), 0);

    // ゲージ・ラベルの表示/非表示を切り替え / Toggle visibility of gauges and labels
    if (offline) {
        lv_obj_add_flag(arc_session,       LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(arc_week,          LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(arc_limit_session, LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(arc_limit_week,    LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(label_session,     LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(label_week,        LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(label_limit,       LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(label_offline,  LV_OBJ_FLAG_HIDDEN);
    } else {
        lv_obj_remove_flag(arc_session,    LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(arc_week,       LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(arc_limit_session, LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(arc_limit_week, LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(label_session,  LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(label_week,     LV_OBJ_FLAG_HIDDEN);
        lv_obj_remove_flag(label_limit,    LV_OBJ_FLAG_HIDDEN);
        lv_obj_add_flag(label_offline,     LV_OBJ_FLAG_HIDDEN);
    }
}

void ui_update(int session_pct, int week_pct, int session_limit, int week_limit, edit_target_t target, bool stale) {
    // BLE受信値など範囲外が来ても描画が崩れないようクランプ / Clamp so out-of-range values (e.g. from BLE) don't break rendering
    session_pct   = constrain(session_pct,   0, 100);
    week_pct      = constrain(week_pct,      0, 100);
    session_limit = constrain(session_limit, 0, 100);
    week_limit    = constrain(week_limit,    0, 100);

    // stale時はアーク色を暗くして「古い値」を示す / Dim arc colors when showing stale (cached) values
    lv_obj_set_style_arc_color(arc_session, stale ? lv_color_hex(0x006622) : lv_color_hex(0x00CC44), LV_PART_INDICATOR);
    lv_obj_set_style_arc_color(arc_week,    stale ? lv_color_hex(0x004080) : lv_color_hex(0x0080FF), LV_PART_INDICATOR);

    lv_arc_set_angles(arc_session, 0, session_pct * 360 / 100);
    lv_arc_set_angles(arc_week,    0, week_pct    * 360 / 100);

    auto set_marker = [](lv_obj_t *arc, int pct) {
        int start = min(pct * 360 / 100, 358);  // 常に2°のセグメントを確保 / always keep a 2° segment
        lv_arc_set_angles(arc, start, start + 2);
    };
    set_marker(arc_limit_session, session_limit);
    set_marker(arc_limit_week,    week_limit);

    // 編集中のマーカーを明るく、非編集を暗く / Brighten the marker being edited, dim the other
    lv_color_t active   = lv_color_hex(0xFF4400);
    lv_color_t inactive = lv_color_hex(0x662200);
    lv_obj_set_style_arc_color(arc_limit_session,
        target == EDIT_SESSION ? active : inactive, LV_PART_INDICATOR);
    lv_obj_set_style_arc_color(arc_limit_week,
        target == EDIT_WEEK ? active : inactive, LV_PART_INDICATOR);

    char buf[24];
    snprintf(buf, sizeof(buf), "%d%%", session_pct);
    lv_label_set_text(label_session, buf);

    snprintf(buf, sizeof(buf), "week %d%%", week_pct);
    lv_label_set_text(label_week, buf);

    // 編集中のリミット値を表示 / Show the limit value being edited
    if (target == EDIT_SESSION) {
        snprintf(buf, sizeof(buf), "! S:%d%%", session_limit);
    } else {
        snprintf(buf, sizeof(buf), "! W:%d%%", week_limit);
    }
    lv_label_set_text(label_limit, buf);
}
