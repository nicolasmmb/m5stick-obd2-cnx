"""Decode the compact P line emitted by the firmware."""

from __future__ import annotations

from ..config import PID_UNITS


def _bytes(raw_hex: str) -> bytes | None:
    raw_hex = raw_hex.strip().replace(" ", "")
    if len(raw_hex) < 2 or len(raw_hex) % 2:
        return None
    try:
        return bytes.fromhex(raw_hex)
    except ValueError:
        return None


def decode_pid(pid: str, raw_hex: str) -> tuple[float, str] | None:
    """Return (value, unit) for a raw PID payload, or None if unknown."""
    data = _bytes(raw_hex)
    if not data:
        return None
    unit = PID_UNITS.get(pid, "")

    try:
        if pid == "0C":
            if len(data) < 2:
                return None
            return (data[0] * 256 + data[1]) / 4.0, "rpm"
        if pid == "0D":
            return float(data[0]), "km/h"
        if pid in {"05", "0F", "46"}:
            return float(data[0]) - 40.0, "C"
        if pid in {"11", "04", "2F", "52", "45", "49", "4A", "4C"}:
            return data[0] * 100.0 / 255.0, "%"
        if pid in {"0B", "33"}:
            return float(data[0]), "kPa"
        if pid == "10":
            if len(data) < 2:
                return None
            return (data[0] * 256 + data[1]) / 100.0, "g/s"
        if pid == "42":
            if len(data) < 2:
                return None
            return (data[0] * 256 + data[1]) / 1000.0, "V"
        if pid == "3C":
            if len(data) < 2:
                return None
            return (data[0] * 256 + data[1]) / 10.0 - 40.0, "C"
        if pid == "44":
            if len(data) < 2:
                return None
            return 2.0 * (data[0] * 256 + data[1]) / 65536.0, "lambda"
        if pid in {"06", "07"}:
            return (data[0] - 128.0) * 100.0 / 128.0, "%"
        if pid == "0E":
            return data[0] / 2.0 - 64.0, "deg"
        if pid == "1F":
            if len(data) < 2:
                return None
            return float(data[0] * 256 + data[1]), "s"
        if pid in {"21", "31"}:
            if len(data) < 2:
                return None
            return float(data[0] * 256 + data[1]), "km"
        if pid == "43":
            if len(data) < 2:
                return None
            return (data[0] * 256 + data[1]) * 100.0 / 255.0, "%"
    except IndexError:
        return None

    return None if unit == "" else (float(data[0]), unit)
