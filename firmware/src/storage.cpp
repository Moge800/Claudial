#include "storage.h"
#include <Preferences.h>

static Preferences prefs;

void storage_init() {
    prefs.begin("Claudial", false);
}

void storage_set_device_name(const char *name) {
    prefs.putString("dev_name", name);
}

String storage_get_device_name() {
    return prefs.getString("dev_name", "Claudial");
}

void storage_set_rotation(uint8_t rotation) {
    prefs.putUChar("rotation", rotation);
}

uint8_t storage_get_rotation() {
    return prefs.getUChar("rotation", 0);  // デフォルト: USB下（台座使用時）/ default: USB at bottom (with stand)
}

void storage_set_session_limit(uint8_t pct) {
    prefs.putUChar("s_limit", pct);
}

uint8_t storage_get_session_limit() {
    return prefs.getUChar("s_limit", 80);
}

void storage_set_week_limit(uint8_t pct) {
    prefs.putUChar("w_limit", pct);
}

uint8_t storage_get_week_limit() {
    return prefs.getUChar("w_limit", 80);
}
