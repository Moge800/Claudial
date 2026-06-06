#include "ble.h"
#include <NimBLEDevice.h>
#include <ArduinoJson.h>
#include <freertos/FreeRTOS.h>
#include <freertos/task.h>

// Clawdmeterと共通のUUID
#define SVC_UUID  "4c41555a-4465-7669-6365-000000000001"
#define RX_UUID   "4c41555a-4465-7669-6365-000000000002"  // write (daemon→device)
#define TX_UUID   "4c41555a-4465-7669-6365-000000000003"  // notify (device→daemon)

// NimBLEコールバック（別タスク）とloop()の競合をFreeRTOSクリティカルセクションで防ぐ
static portMUX_TYPE   data_mux  = portMUX_INITIALIZER_UNLOCKED;
static BleData        latest_data = {0, 0, 0, 0, false};
static bool           connected  = false;

class ServerCB : public NimBLEServerCallbacks {
    void onConnect(NimBLEServer *s)    override { connected = true;  }
    void onDisconnect(NimBLEServer *s) override {
        connected = false;
        NimBLEDevice::startAdvertising();
    }
};

class RxCB : public NimBLECharacteristicCallbacks {
    void onWrite(NimBLECharacteristic *c) override {
        std::string val = c->getValue();
        JsonDocument doc;
        if (deserializeJson(doc, val) != DeserializationError::Ok) return;

        BleData d;
        d.session_pct   = doc["s"]  | 0;
        d.week_pct      = doc["w"]  | 0;
        d.session_reset = doc["sr"] | 0;
        d.week_reset    = doc["wr"] | 0;
        d.ok            = doc["ok"] | false;

        taskENTER_CRITICAL(&data_mux);
        latest_data = d;
        taskEXIT_CRITICAL(&data_mux);
    }
};

void ble_init(const char *device_name) {
    NimBLEDevice::init(device_name);

    NimBLEServer *server = NimBLEDevice::createServer();
    server->setCallbacks(new ServerCB());

    NimBLEService *svc = server->createService(SVC_UUID);

    NimBLECharacteristic *rx = svc->createCharacteristic(
        RX_UUID, NIMBLE_PROPERTY::WRITE | NIMBLE_PROPERTY::WRITE_NR);
    rx->setCallbacks(new RxCB());

    svc->createCharacteristic(TX_UUID, NIMBLE_PROPERTY::NOTIFY);

    svc->start();

    NimBLEAdvertising *adv = NimBLEDevice::getAdvertising();
    adv->addServiceUUID(SVC_UUID);
    adv->start();
}

bool ble_is_connected() { return connected; }

BleData ble_get_data() {
    taskENTER_CRITICAL(&data_mux);
    BleData d = latest_data;
    taskEXIT_CRITICAL(&data_mux);
    return d;
}
