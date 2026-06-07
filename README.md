# Clawdial

**[日本語版 README はこちら](README_jp.md)**

A **Claude Code usage monitor** running on M5Stack Dial (ESP32-S3).

Just place it on your desk. Session and weekly API usage are displayed in real time — a double beep warns you as you approach your limit, and a continuous alert fires when you reach it. Rotate the dial to adjust the threshold on the fly.

<table role="presentation"><tr>
<td><img src="assets/device.jpg" width="180" alt="Clawdial on desk"></td>
<td><img src="assets/alert_demo.gif" width="180" alt="Alert demo (red flash + beep)"></td>
</tr></table>

---

## Stand

A 3D-printed stand keeps the dial upright with the USB port at the bottom.

> 🖨️ Stand design: [MakerWorld: M5Stack Dial Rotary Knob Stand](https://makerworld.com/ja/models/763395-m5stack-dial-rotary-knob-stand) by [DaNi 3D Lab](https://makerworld.com/en/@DaNi3DLab) — thanks!

With the stand, long-press the touch to toggle orientation — the default is "USB at bottom".

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
| [Go 1.26+](https://go.dev/dl/) | Daemon build (not required if using pre-built binary) |
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
4. Under `m5stack-stamps3 → General`, click **Upload**
5. Wait for `SUCCESS` in the terminal — this takes about a minute on first run (downloading toolchain)

> **Port not found?** On Windows, you may need the [CP210x USB driver](https://www.silabs.com/developers/usb-to-uart-bridge-vcp-drivers). Install it, then replug the cable.

After flashing, you can unplug the USB cable. Normal use is USB-C power (charger, etc.) + BLE.

### Daemon (PC side)

**Prerequisite:** Install [Claude Code](https://claude.ai/code) and run `claude login` before starting the daemon.

#### Option A — Download pre-built binary (no Go required)

Download `clawdial-daemon.exe` from the [latest release](https://github.com/Moge800/Clawdial/releases/latest), place it anywhere you like, and run it. That's it.

#### Option B — Install script (build or download, your choice)

```
# Windows — double-click or run in terminal
daemon\install.bat
```

The script prompts you to either download the pre-built binary or build from source (Go required for the build option). It also handles startup registration and auth check.

```
# macOS / Linux (untested — testing planned)
chmod +x daemon/install.sh
./daemon/install.sh
```

#### Option C — Build manually

```bash
cd daemon
go build -ldflags "-H=windowsgui" -o clawdial-daemon.exe .   # Windows
go build -o clawdial-daemon .                                  # macOS / Linux (untested)
```

> **macOS / Linux:** The code compiles and runs, but has not been tested on these platforms yet. Testing is planned for the near future.

> **Token usage**
> The daemon fetches usage by making a minimal 1-token API call (`claude-haiku-4-5-20251001`) every poll interval and reading the rate-limit headers from the response. This costs roughly $0.03/day at the default 60-second interval — well within the noise of a normal Claude Code session.

The daemon must **stay running** while you use Claude Code.
Use the startup registration option in `install.bat` / `install.sh` to launch it automatically on boot.

> **Windows:** The daemon runs without a console window (built with `-H=windowsgui`). It appears only as a system tray icon. The installer (`install.bat`) itself opens a temporary console window, which closes when installation is complete.

**Uninstall (Windows)**

1. Right-click the tray icon → **Quit**
2. Delete `clawdial-daemon.exe` and `daemon.log` from the install folder
3. Delete the startup shortcut: open `shell:startup` in Explorer and remove `Clawdial.lnk`

> **Token expiry**
> Claude Code auth tokens expire after a few hours. If the daemon logs a 401 error, run `claude login` again.

**Configuration (optional)**

Copy `daemon/.env.example` to `daemon/.env` and edit:

```env
CLAWDIAL_DEVICE_NAME=Clawdial    # BLE device name (must match firmware)
CLAWDIAL_POLL_INTERVAL=60        # Polling interval in seconds
CLAWDIAL_SCAN_TIMEOUT=15         # BLE scan timeout in seconds
```

> **Config changes take effect on restart.** On Windows, use Quit → relaunch (or the systray **Open Config** tooltip is a reminder of this).

---

## Usage

### Setting Warning Limits

The rotary dial is the core interaction — it lets you set the usage threshold at which Clawdial warns you, directly on the device with no app or config file needed.

1. **Short-press** the touch screen to select which limit to edit — **Session** or **Week** (the active one is highlighted on the display)
2. **Rotate the dial** to raise or lower the threshold **±1% per click**
3. The limit is **saved to non-volatile storage ~1 second after you stop turning** — it persists across reboots and power cuts, no reflashing needed (avoid power-cycling immediately after adjusting)

> **Example:** Set Session limit to 80% → Clawdial double-beeps when you hit 75%, then flashes red and beeps continuously at 80%.

### Controls

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

Uses Clawdial-specific UUIDs (RFC 4122 v4, base `29590732-a70c-4ea9-a739-…`).

| Item | UUID |
|------|------|
| Service | `29590732-a70c-4ea9-a739-000000000001` |
| RX Characteristic (write) | `29590732-a70c-4ea9-a739-000000000002` |
| TX Characteristic (notify) | `29590732-a70c-4ea9-a739-000000000003` |

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

Inspired by [Clawdmeter](https://github.com/HermannBjorgvin/Clawdmeter) by [@HermannBjorgvin](https://github.com/HermannBjorgvin). The rate-limit header polling approach is inspired by that project; BLE UUIDs and all other implementation details are original.
