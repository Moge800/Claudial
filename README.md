# Clawdial

**[日本語版 README はこちら](README_jp.md)**

> Inspired by [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) by [@HermannBjorgvin](https://github.com/HermannBjorgvin).
> BLE UUID / payload format and the rate-limit header approach are derived from that project.

A **Claude Code usage monitor** running on M5Stack Dial (ESP32-S3).

Just place it on your desk and turn the dial. Session and weekly usage are displayed in real time, with audio alerts as you approach your limit.

![Clawdial on desk](assets/device.jpg)

![Alert demo (red flash + beep)](assets/alert_demo.gif)

---

## Stand

A 3D-printed stand keeps the dial upright with the USB port at the bottom.

> 🖨️ Stand design: [MakerWorld: M5Stack Dial Rotary Knob Stand](https://makerworld.com/ja/models/763395-m5stack-dial-rotary-knob-stand)

With the stand, long-press the touch to set orientation to "USB at bottom" (the default).

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

**1. Install VS Code and PlatformIO**

1. Install [Visual Studio Code](https://code.visualstudio.com/)
2. Open the Extensions panel (`Ctrl+Shift+X`), search for **PlatformIO IDE**, and install it
3. Restart VS Code when prompted

**2. Flash the firmware**

1. Connect M5Stack Dial to your PC with a USB-C cable
2. Open this repository folder in VS Code (`File → Open Folder`)
3. Click the **PlatformIO icon** in the left sidebar (the alien head icon)
4. Under `m5stack-dial → General`, click **Upload**
5. Wait for `SUCCESS` in the terminal — this takes about a minute on first run (downloading toolchain)

> **Port not found?** On Windows, you may need the [CP210x USB driver](https://www.silabs.com/developers/usb-to-uart-bridge-vcp-drivers). Install it, then replug the cable.

After flashing, you can unplug the USB cable. Normal use is USB-C power (charger, etc.) + BLE.

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

> **Token usage**
> The daemon fetches usage by making a minimal 1-token API call (claude-haiku) every poll interval and reading the rate-limit headers from the response. This costs roughly $0.03/day at the default 60-second interval — well within the noise of a normal Claude Code session.

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
| Touch (short) | Switch edit target: Session ↔ Week |
| Touch (during alert) | Mute alert |
| Touch (hold 1 sec) | Flip screen orientation 180° and reboot |

### Screen Orientation

The device stores the screen orientation in NVS (non-volatile storage), so it persists across reboots without reflashing.

| Orientation | When to use |
|-------------|-------------|
| USB at bottom (default) | With 3D-printed stand |
| USB at top | Cable-hanging / direct USB connection |

Long-press the touch screen for 1 second to toggle between orientations. The device beeps and reboots automatically.

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
{ "s": 45, "sr": 120, "w": 28, "wr": 7200, "pi": 60, "ok": true, "st": false }
```

| Field | Meaning |
|-------|---------|
| `s` | Session usage (%) |
| `sr` | Minutes until session reset |
| `w` | Weekly usage (%) |
| `wr` | Minutes until weekly reset |
| `pi` | Poll interval (seconds) — used by the device to compute the offline timeout (`pi×2+30s`) |
| `ok` | Fetch success flag (`false` = token error; triggers immediate offline screen) |
| `st` | Stale flag — `true` when sending a cached previous value (e.g. during rate limiting); device dims the gauge colors |

---

## License

MIT — see [LICENSE](LICENSE).

Inspired by [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter). The BLE UUIDs and rate-limit header polling approach are derived from that project; all code is original.
