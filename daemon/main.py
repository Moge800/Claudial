"""Clawdial daemon — Claude Code usage monitor via BLE."""

import asyncio
import json
import os
import time
from pathlib import Path

import httpx
from bleak import BleakClient, BleakScanner
from dotenv import load_dotenv

load_dotenv()

DEVICE_NAME      = os.getenv("BLE_DEVICE_NAME", "Clawdial")
CLAUDE_API_BASE  = os.getenv("CLAUDE_API_BASE", "https://api.anthropic.com")
SESSION_DURATION = int(os.getenv("SESSION_DURATION", "300"))

RX_UUID = "4c41555a-4465-7669-6365-000000000002"

CREDS_PATH = Path.home() / ".claude" / ".credentials.json"


def load_token() -> str:
    """~/.claude/.credentials.json からOAuthトークンを読む。"""
    data = json.loads(CREDS_PATH.read_text())
    return data.get("claudeAiOauth", {}).get("accessToken", "")


async def fetch_usage(token: str) -> dict:
    """Claude Code の使用量を取得する。"""
    headers = {
        "Authorization": f"Bearer {token}",
        "anthropic-version": "2023-06-01",
    }
    async with httpx.AsyncClient(base_url=CLAUDE_API_BASE) as client:
        r = await client.get("/api/organizations/usage", headers=headers, timeout=10)
        r.raise_for_status()
        return r.json()


def parse_usage(data: dict, session_start: float) -> dict:
    """APIレスポンスから s/sr/w/wr を計算する。"""
    now = time.time()

    session_used  = data.get("session_tokens_used", 0)
    session_limit = data.get("session_token_limit", 1) or 1
    week_used     = data.get("weekly_tokens_used", 0)
    week_limit    = data.get("weekly_token_limit", 1) or 1

    s  = min(int(session_used / session_limit * 100), 100)
    w  = min(int(week_used   / week_limit   * 100), 100)
    sr = max(0, int((session_start + SESSION_DURATION - now) / 60))
    wr = data.get("weekly_reset_minutes_remaining", 0)

    return {"s": s, "sr": sr, "w": w, "wr": wr, "ok": True}


async def find_device():
    print(f"Scanning for '{DEVICE_NAME}'...")
    device = await BleakScanner.find_device_by_name(DEVICE_NAME, timeout=15)
    if device is None:
        raise RuntimeError(f"Device '{DEVICE_NAME}' not found")
    print(f"Found: {device.address}")
    return device


async def run():
    token = load_token()
    session_start = time.time()
    device = await find_device()

    async with BleakClient(device) as client:
        print("Connected!")
        while True:
            try:
                data = await fetch_usage(token)
                payload = parse_usage(data, session_start)
            except Exception as e:
                print(f"Usage fetch error: {e}")
                payload = {"s": 0, "sr": 0, "w": 0, "wr": 0, "ok": False}

            msg = json.dumps(payload).encode()
            await client.write_gatt_char(RX_UUID, msg, response=False)
            print(f"Sent: {payload}")

            await asyncio.sleep(30)


if __name__ == "__main__":
    asyncio.run(run())
