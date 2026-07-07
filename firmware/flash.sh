#!/usr/bin/env bash
set -u

echo "========================================"
echo " Claudial Firmware Flasher (Linux)"
echo "========================================"
echo

# Check Python
if command -v python3 >/dev/null 2>&1; then
    PYTHON=python3
elif command -v python >/dev/null 2>&1; then
    PYTHON=python
else
    echo "[ERROR] Python not found. Install it with your package manager"
    echo "        (e.g. sudo pacman -S python)."
    exit 1
fi
echo "[OK] $("$PYTHON" --version 2>&1) found."
echo

# Check / install esptool
find_esptool() {
    if command -v esptool >/dev/null 2>&1; then
        ESPTOOL=(esptool)
    elif command -v esptool.py >/dev/null 2>&1; then
        ESPTOOL=(esptool.py)
    elif "$PYTHON" -m esptool version >/dev/null 2>&1; then
        ESPTOOL=("$PYTHON" -m esptool)
    else
        return 1
    fi
}

if ! find_esptool; then
    echo "[INFO] esptool not found. Installing..."
    if command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --needed --noconfirm esptool
    else
        "$PYTHON" -m pip install --user --quiet esptool
    fi
    if ! find_esptool; then
        echo "[ERROR] Failed to install esptool."
        echo "        On Arch:   sudo pacman -S esptool"
        echo "        Elsewhere: pipx install esptool"
        exit 1
    fi
    echo "[OK] esptool installed."
else
    ESP_VER=$("${ESPTOOL[@]}" version 2>/dev/null | awk 'NR==1 {print $2}')
    echo "[OK] esptool ${ESP_VER:-} found."
fi
echo

# Locate firmware binary
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
BIN="$SCRIPT_DIR/claudial-firmware.bin"
if [[ ! -f "$BIN" ]]; then
    echo "[ERROR] claudial-firmware.bin not found next to this script."
    echo "        Download it from:"
    echo "        https://github.com/Moge800/Claudial/releases/latest"
    exit 1
fi
echo "[OK] Found: $BIN"
echo

# Auto-detect serial port
echo "[INFO] Scanning for connected devices..."
PORTS=()
LABELS=()
ESPRESSIF=()
for dev in /dev/ttyACM* /dev/ttyUSB*; do
    [[ -e "$dev" ]] || continue
    PORTS+=("$dev")
    VENDOR_ID=""
    LABEL=""
    if command -v udevadm >/dev/null 2>&1; then
        PROPS=$(udevadm info -q property "$dev" 2>/dev/null)
        VENDOR_ID=$(grep -m1 '^ID_VENDOR_ID=' <<<"$PROPS" | cut -d= -f2)
        VENDOR=$(grep -m1 '^ID_VENDOR=' <<<"$PROPS" | cut -d= -f2)
        MODEL=$(grep -m1 '^ID_MODEL=' <<<"$PROPS" | cut -d= -f2)
        [[ -n "$VENDOR$MODEL" ]] && LABEL=" ($VENDOR $MODEL)"
    fi
    LABELS+=("$LABEL")
    # Espressif USB vendor ID - what the Dial's ESP32-S3 enumerates as
    [[ "$VENDOR_ID" == "303a" ]] && ESPRESSIF+=("$dev")
done

if [[ ${#ESPRESSIF[@]} -eq 1 ]]; then
    PORT=${ESPRESSIF[0]}
    echo "[OK] Auto-detected Espressif device: $PORT"
elif [[ ${#PORTS[@]} -eq 0 ]]; then
    echo "[WARN] No serial port found. Connect M5Stack Dial via USB-C."
    echo "       (Check 'ls /dev/ttyACM* /dev/ttyUSB*' after plugging in.)"
    echo
    read -rp "Enter port manually (e.g. /dev/ttyACM0): " PORT
elif [[ ${#PORTS[@]} -eq 1 ]]; then
    PORT=${PORTS[0]}
    echo "[OK] Auto-detected: $PORT"
else
    echo "Found multiple devices:"
    for i in "${!PORTS[@]}"; do
        echo "  [$((i + 1))] ${PORTS[$i]}${LABELS[$i]}"
    done
    echo
    read -rp "Select device number [1-${#PORTS[@]}]: " CHOICE
    if [[ "$CHOICE" =~ ^[0-9]+$ ]] && (( CHOICE >= 1 && CHOICE <= ${#PORTS[@]} )); then
        PORT=${PORTS[$((CHOICE - 1))]}
    else
        echo "[WARN] Invalid choice - enter port manually."
        read -rp "Port (e.g. /dev/ttyACM0): " PORT
    fi
fi

# Normalize / validate PORT
PORT=${PORT// /}
if [[ ! "$PORT" =~ ^/dev/tty(ACM|USB)[0-9]+$ ]]; then
    echo "[ERROR] \"$PORT\" does not look like a valid serial port."
    exit 1
fi

# Check we can actually open the port
if [[ ! -r "$PORT" || ! -w "$PORT" ]]; then
    echo "[ERROR] No permission to access $PORT."
    if command -v pacman >/dev/null 2>&1; then
        echo "        Add yourself to the uucp group, then log out and back in:"
        echo "          sudo usermod -aG uucp $USER"
    else
        echo "        Add yourself to the dialout group, then log out and back in:"
        echo "          sudo usermod -aG dialout $USER"
    fi
    exit 1
fi
echo

# Flash
echo "[INFO] Flashing to $PORT at 921600 baud..."
echo "       This takes about 30 seconds."
echo
if ! "${ESPTOOL[@]}" --chip esp32s3 --port "$PORT" --baud 921600 \
    write-flash 0x0 "$BIN"; then
    echo
    echo "[ERROR] Flash failed."
    echo "        - Check the port is correct"
    echo "        - Try a different USB cable or replug the device"
    echo "        - Hold the boot button while plugging in if auto-reset fails"
    exit 1
fi

echo
echo "========================================"
echo " Done! Unplug and replug to reboot."
echo "========================================"
