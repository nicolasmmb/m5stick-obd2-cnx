"""Live serial ingestion of the compact I/P/E/X protocol."""

from __future__ import annotations

import time
from datetime import datetime

import polars as pl
import serial

from .decode import decode_pid
from .files import RawSession


def read_serial_session(
    port: str,
    seconds: int = 60,
    follow: bool = False,
    baud: int = 115200,
) -> RawSession | None:
    rows: list[tuple[int, str, float, str]] = []
    events: list[dict] = []
    meta: dict = {
        "started_at": datetime.now().isoformat(),
        "session_id": "serial-" + datetime.now().strftime("%Y%m%d-%H%M%S"),
        "port": port,
    }

    deadline = None if follow else time.monotonic() + seconds
    buf = b""
    with serial.Serial(port, baud, timeout=0.2) as ser:
        while True:
            if deadline is not None and time.monotonic() >= deadline:
                break
            chunk = ser.read(4096)
            if not chunk:
                continue
            buf += chunk
            while b"\n" in buf:
                raw_line, buf = buf.split(b"\n", 1)
                line = raw_line.decode("utf-8", "replace").strip().strip("\r")
                if not line:
                    continue
                parts = line.split(",")
                kind = parts[0]
                if kind == "P" and len(parts) >= 5:
                    try:
                        ms = int(parts[1])
                    except ValueError:
                        continue
                    pid = parts[2].upper()
                    decoded = decode_pid(pid, parts[3])
                    if decoded is None:
                        continue
                    quality = parts[5] if len(parts) > 5 else "OK"
                    rows.append((ms, pid, float(decoded[0]), quality))
                elif kind == "I" and len(parts) >= 5:
                    meta["vin"] = parts[3]
                    meta["tank_dl"] = parts[4]
                elif kind == "E" and len(parts) >= 4:
                    events.append({"t": "E", "ms": parts[1], "type": parts[2], "msg": parts[3]})
                elif kind == "X" and len(parts) >= 3:
                    events.append({"t": "X", "ms": parts[1], "type": "END", "msg": parts[2]})
                    if not follow:
                        deadline = time.monotonic()

    if not rows:
        return None
    long = pl.DataFrame(
        rows, schema=["ms", "pid", "value", "quality"], orient="row"
    ).sort("ms")
    ev = pl.DataFrame(events) if events else None
    return RawSession(
        id=meta["session_id"],
        path=None,  # type: ignore[arg-type]
        long=long,
        metadata=meta,
        events=ev,
    )
