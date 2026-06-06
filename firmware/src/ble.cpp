#include "ble.h"
#include <NimBLEDevice.h>
#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>

// Clawdmeterと共通のUUID / UUIDs shared with Clawdmeter
#define SVC_UUID  "4c41555a-4465-7669-6365-000000000001"
#define RX_UUID   "4c41555a-4465-7669-6365-000000000002"
#define TX_UUID   "4c41555a-4465-7669-6365-000000000003"

// connected / latest_data 両方を同じ mutex で保護 / Protect both connected and latest_data with the same mutex
static portMUX_TYPE data_mux   = portMUX_INITIALIZER_UNLOCKED;
static BleData      latest_data = {0, 0, 0, 0, 0, false, false, 0};
static bool         connected   = false;

// コールバックオブジェクトをstatic化してヒープ確保を回避 / Use static callback instances to avoid heap allocation for the objects themselves
static class ServerCB : public NimBLEServerCallbacks {
    void onConnect(NimBLEServer *) override {
        taskENTER_CRITICAL(&data_mux);
        connected = true;
        taskEXIT_CRITICAL(&data_mux);
    }
    void onDisconnect(NimBLEServer *) override {
        taskENTER_CRITICAL(&data_mux);
        connected = false;
        taskEXIT_CRITICAL(&data_mux);
        NimBLEDevice::startAdvertising();
    }
} serverCB;

static class RxCB : public NimBLECharacteristicCallbacks {
    void onWrite(NimBLECharacteristic *c) override {
        std::string val = c->getValue();
        JsonDocument doc;
        if (deserializeJson(doc, val) != DeserializationError::Ok) return;

        BleData d;
        d.session_pct   = doc["s"]  | 0;
        d.week_pct      = doc["w"]  | 0;
        d.session_reset = doc["sr"] | 0;
        d.week_reset    = doc["wr"] | 0;
        d.poll_interval = doc["pi"] | 0;
        d.ok            = doc["ok"] | false;
        d.stale         = doc["st"] | false;
        d.received_ms   = millis();

        taskENTER_CRITICAL(&data_mux);
        latest_data = d;
        taskEXIT_CRITICAL(&data_mux);
    }
} rxCB;

void ble_init(const char *device_name) {
    NimBLEDevice::init(device_name);
    // デフォルトMTU=23では20byteしか送れないため拡張を要求
    // Default MTU=23 allows only 20 bytes, so request a larger one.
    // 実際のMTUは接続時に双方でネゴシエートされる
    // The actual MTU is negotiated between both sides on connection.
    NimBLEDevice::setMTU(512);

    NimBLEServer *server = NimBLEDevice::createServer();
    server->setCallbacks(&serverCB);

    NimBLEService *svc = server->createService(SVC_UUID);

    NimBLECharacteristic *rx = svc->createCharacteristic(
        RX_UUID, NIMBLE_PROPERTY::WRITE | NIMBLE_PROPERTY::WRITE_NR);
    rx->setCallbacks(&rxCB);

    svc->createCharacteristic(TX_UUID, NIMBLE_PROPERTY::NOTIFY);

    svc->start();

    NimBLEAdvertising *adv = NimBLEDevice::getAdvertising();
    adv->addServiceUUID(SVC_UUID);
    adv->start();
}

bool ble_is_connected() {
    taskENTER_CRITICAL(&data_mux);
    bool c = connected;
    taskEXIT_CRITICAL(&data_mux);
    return c;
}

BleData ble_get_data() {
    taskENTER_CRITICAL(&data_mux);
    BleData d = latest_data;
    taskEXIT_CRITICAL(&data_mux);
    return d;
}
