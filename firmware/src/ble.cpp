#include "ble.h"
#include <NimBLEDevice.h>
#include <ArduinoJson.h>

// Clawdmeterと共通のUUID
#define SVC_UUID  "4c41555a-4465-7669-6365-000000000001"
#define RX_UUID   "4c41555a-4465-7669-6365-000000000002"  // write (daemon→device)
#define TX_UUID   "4c41555a-4465-7669-6365-000000000003"  // notify (device→daemon)

static BleData   latest_data = {0, 0, 0, 0, false};
static bool      connected   = false;

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

        latest_data.session_pct   = doc["s"]  | 0;
        latest_data.week_pct      = doc["w"]  | 0;
        latest_data.session_reset = doc["sr"] | 0;
        latest_data.week_reset    = doc["wr"] | 0;
        latest_data.ok            = doc["ok"] | false;
    }
};

void ble_init(const char *device_name) {
    NimBLEDevice::init(device_name);

    NimBLEServer *server = NimBLEDevice::createServer();
    server->setCallbacks(new ServerCB());

    NimBLEService *svc = server->createService(SVC_UUID);

    // RX: daemonから書き込まれる
    NimBLECharacteristic *rx = svc->createCharacteristic(
        RX_UUID, NIMBLE_PROPERTY::WRITE | NIMBLE_PROPERTY::WRITE_NR);
    rx->setCallbacks(new RxCB());

    // TX: notify用（将来的にリミット値をdaemonへ返す用途）
    svc->createCharacteristic(TX_UUID, NIMBLE_PROPERTY::NOTIFY);

    svc->start();

    NimBLEAdvertising *adv = NimBLEDevice::getAdvertising();
    adv->addServiceUUID(SVC_UUID);
    adv->start();
}

bool ble_is_connected() { return connected; }

BleData ble_get_data() { return latest_data; }
