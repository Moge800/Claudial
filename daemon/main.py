"""Clawdial daemon — Claude Code usage monitor via BLE.

Usage data is read from rate-limit headers returned by the Anthropic API,
following the approach used by Clawdmeter (github.com/HermannBjorgvin/Clawdmeter).
"""

import asyncio
import json
import os
import re
import time
from pathlib import Path

import httpx
from bleak import BleakClient, BleakScanner
from bleak.exc import BleakError
from dotenv import load_dotenv

load_dotenv()

DEVICE_NAME   = os.getenv("BLE_DEVICE_NAME", "Clawdial")
POLL_INTERVAL = int(os.getenv("POLL_INTERVAL", "60"))

CREDENTIALS_PATH = Path.home() / ".claude" / ".credentials.json"

API_URL = "https://api.anthropic.com/v1/messages"
API_HEADERS = {
    "anthropic-version": "2023-06-01",
    "anthropic-beta": "oauth-2025-04-20",
    "Content-Type": "application/json",
    "User-Agent": "claude-code/2.1.5",
}
API_BODY = {
    "model": "claude-haiku-4-5-20251001",
    "max_tokens": 1,
    "messages": [{"role": "user", "content": "hi"}],
}

RX_UUID = "4c41555a-4465-7669-6365-000000000002"


def load_token() -> str:
    raw = CREDENTIALS_PATH.read_text()
    data = json.loads(raw)
    if isinstance(data.get("accessToken"), str):
        return data["accessToken"]
    for v in data.values():
        if isinstance(v, dict) and isinstance(v.get("accessToken"), str):
            return v["accessToken"]
    m = re.search(r'"accessToken"\s*:\s*"([^"]+)"', raw)
    if m:
        return m.group(1)
    raise RuntimeError("accessToken not found in credentials")


def clamp(val: int, lo: int = 0, hi: int = 100) -> int:
    return max(lo, min(hi, val))


async def fetch_usage(token: str) -> tuple[dict | None, float]:
    """使用量を取得する。戻り値は (payload, retry_after_seconds)。
    429 のときは payload=None, retry_after>0 を返す。"""
    headers = dict(API_HEADERS)
    headers["Authorization"] = f"Bearer {token}"
    try:
        async with httpx.AsyncClient(timeout=20.0) as client:
            resp = await client.post(API_URL, headers=headers, json=API_BODY)
    except httpx.HTTPError as e:
        print(f"API error: {e}")
        return None, 0.0

    if resp.status_code == 429:
        retry_after = float(resp.headers.get("retry-after", POLL_INTERVAL))
        print(f"Rate limited. Retry after {retry_after:.0f}s")
        return None, retry_after

    if resp.status_code >= 400:
        print(f"API HTTP {resp.status_code}: {resp.text[:200]}")
        return None, 0.0

    now = time.time()

    def hdr(name: str, default: str = "0") -> str:
        return resp.headers.get(name, default)

    def pct(util: str) -> int:
        try:
            return clamp(int(round(float(util) * 100)))
        except ValueError:
            return 0

    def reset_minutes(reset_ts: str) -> int:
        try:
            mins = (float(reset_ts) - now) / 60.0
            return int(round(mins)) if mins > 0 else 0
        except ValueError:
            return 0

    return {
        "s":  pct(hdr("anthropic-ratelimit-unified-5h-utilization")),
        "sr": reset_minutes(hdr("anthropic-ratelimit-unified-5h-reset")),
        "w":  pct(hdr("anthropic-ratelimit-unified-7d-utilization")),
        "wr": reset_minutes(hdr("anthropic-ratelimit-unified-7d-reset")),
        "ok": True,
    }, 0.0


async def find_device():
    print(f"Scanning for '{DEVICE_NAME}'...")
    device = await BleakScanner.find_device_by_name(DEVICE_NAME, timeout=15)
    if device is None:
        raise RuntimeError(f"Device '{DEVICE_NAME}' not found")
    print(f"Found: {device.address}")
    return device


async def run():
    token = load_token()
    device = await find_device()

    while True:
        try:
            async with BleakClient(device) as client:
                print("Connected!")
                cached: dict | None = None  # 直前の成功ペイロードをキャッシュ

                while client.is_connected:
                    payload, retry_after = await fetch_usage(token)

                    if payload is not None:
                        cached = payload
                    elif cached is not None:
                        # 429 や一時エラー時は直前の値を維持して送り続ける
                        payload = cached
                        print(f"Using cached: {payload}")
                    else:
                        payload = {"s": 0, "sr": 0, "w": 0, "wr": 0, "ok": False}

                    msg = json.dumps(payload, separators=(",", ":")).encode()
                    await client.write_gatt_char(RX_UUID, msg, response=False)
                    print(f"Sent: {payload}")

                    # retry-after は参考値。最大5分でリトライして早期回復を検出
                    if retry_after > 0:
                        wait = min(retry_after, 300)
                        print(f"Waiting {wait:.0f}s before retry...")
                    else:
                        wait = POLL_INTERVAL
                    await asyncio.sleep(wait)

                print("Disconnected. Reconnecting...")
        except BleakError as e:
            print(f"BLE error: {e}. Retrying in 5s...")
            await asyncio.sleep(5)


if __name__ == "__main__":
    asyncio.run(run())
