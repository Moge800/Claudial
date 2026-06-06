# Clawdial

**[日本語版 README はこちら](README_jp.md)**

> Inspired by [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) by [@HermannBjorgvin](https://github.com/HermannBjorgvin).
> BLE UUID / payload format and the rate-limit header approach are derived from that project.

A **Claude Code usage monitor** running on M5Stack Dial (ESP32-S3).

Just place it on your desk and turn the dial. Session and weekly usage are displayed in real time, with audio alerts as you approach your limit.

---

## Hardware

| Item | Details |
|------|---------|
| Board | M5Stack Dial v1.1 |
| MCU | ESP32-S3 (M5StampS3) |
| Display | 1.28" Round IPS LCD 240×240 |
| Input | Rotary encoder + touch |
| Connectivity | BLE 5.0 |

---

## Repository Structure

```
Clawdial/
├── firmware/   PlatformIO project (M5Stack Dial firmware)
└── daemon/     PC-side daemon (Go, Windows / macOS / Linux)
```

---

## Requirements

| Item | Requirement |
|------|------------|
| [PlatformIO](https://platformio.org/) | Firmware build & flash |
| [Go 1.26+](https://go.dev/dl/) | Daemon build |
| [Claude Code](https://claude.ai/code) | Auth credentials (`claude login`) |
| Bluetooth LE 5.0 adapter | PC-side BLE communication |

---

## Setup

### Firmware

Connect M5Stack Dial to your PC via USB-C (**USB is only needed for flashing**).

```bash
cd firmware
pio run -t upload
```

After flashing, you can unplug the USB cable. Normal use is USB-C power (charger, etc.) + BLE.

**Display orientation**

Adjust `DISPLAY_ROTATION` in `src/main.cpp` to match your cable routing:

```cpp
#define DISPLAY_ROTATION 2   // 0 = USB port down, 2 = USB port up
```

Re-flash after changing: `pio run -t upload`

### Daemon (PC side)

**Prerequisite:** Install [Claude Code](https://claude.ai/code) and run `claude login` before starting the daemon.

Using the install script (recommended):

```
# Windows — double-click or run in terminal
daemon\install.bat

# macOS / Linux
chmod +x daemon/install.sh
./daemon/install.sh
```

Manual build:

```bash
cd daemon
go build -o clawdial-daemon .
./clawdial-daemon       # Linux / macOS
clawdial-daemon.exe     # Windows
```

The daemon must **stay running** while you use Claude Code.
Use the startup registration option in `install.bat` / `install.sh` to launch it automatically on boot.

> **Token expiry**
> Claude Code auth tokens expire after a few hours. If the daemon logs a 401 error, run `claude login` again.

**Configuration (optional)**

Copy `daemon/.env.example` to `daemon/.env` and edit:

```env
CLAWDIAL_DEVICE_NAME=Clawdial    # BLE device name (must match firmware)
CLAWDIAL_POLL_INTERVAL=60        # Polling interval in seconds
CLAWDIAL_SCAN_TIMEOUT=15         # BLE scan timeout in seconds
```

---

## Usage

| Action | Behavior |
|--------|----------|
| Rotate dial | Adjust active limit ±1% |
| Touch (normal) | Switch edit target: Session ↔ Week |
| Touch (during alert) | Mute alert |

---

## Alert Behavior

| Usage | Action |
|-------|--------|
| Limit − 5% | Double beep (once) |
| Limit reached | Red screen flash + continuous beeping |
| Touch | Mute until usage drops below limit |

---

## BLE Protocol

Shares the same UUID as Clawdmeter.

| Item | UUID |
|------|------|
| Service | `4c41555a-4465-7669-6365-000000000001` |
| RX Characteristic (write) | `4c41555a-4465-7669-6365-000000000002` |

JSON payload (daemon → device):

```json
{ "s": 45, "sr": 120, "w": 28, "wr": 7200, "ok": true }
```

| Field | Meaning |
|-------|---------|
| `s` | Session usage (%) |
| `sr` | Minutes until session reset |
| `w` | Weekly usage (%) |
| `wr` | Minutes until weekly reset |
| `ok` | Fetch success flag |

---

## License

No license is set on this repository (All Rights Reserved).

The core implementation (BLE UUIDs, API polling approach, rate-limit header reading) is derived from [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter), which itself has no license. Because the upstream project carries no explicit permission to use or redistribute its code, this repository follows the same stance and is published for **personal use and reference only**.

Please do not reuse or redistribute the code.
