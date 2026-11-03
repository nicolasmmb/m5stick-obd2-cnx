"""Import the raw bridge log (Q/R lines) into a long PID table."""

from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path

import polars as pl

from ..config import PID_NAMES
from .decode import decode_pid
from .files import RawSession

_RANGE_MARKERS = {"00", "20", "40", "60", "80", "A0", "C0", "E0"}


def _clean_ascii(data: bytes) -> str:
    text = data.decode("ascii", "ignore").upper()
    for ch in "\r\n>|":
        text = text.replace(ch, "")
    return text


def _parse_command(cmd: str, ms: int, data: bytes) -> list[tuple[int, str, float, str]]:
    text = _clean_ascii(data)
    if not text or "NO DATA" in text or "SEARCHING" in text or text.strip() == "?":
        return []
    rows: list[tuple[int, str, float, str]] = []
    for token in cmd.split():
        token = token.upper()
        if not token.startswith("01") or len(token) < 4:
            continue
        pid = token[2:4]
        if pid in _RANGE_MARKERS:
            continue
        header = "41" + pid
        i = text.find(header)
        if i < 0:
            continue
        tail = text[i + len(header):]
        hex_part = []
        for j in range(0, len(tail) - 1, 2):
            pair = tail[j:j + 2]
            try:
                int(pair, 16)
            except ValueError:
                break
            hex_part.append(pair)
        decoded = decode_pid(pid, "".join(hex_part))
        if decoded is None:
            continue
        rows.append((ms, pid, float(decoded[0]), "OK"))
    return rows


def load_raw_log(path: str | Path) -> RawSession:
    path = Path(path)
    rows: list[tuple[int, str, float, str]] = []
    q_ms = 0
    q_cmd = ""
    buf = bytearray()

    for raw_line in path.read_text(errors="replace").splitlines():
        line = raw_line.split("\t", 1)[-1].strip()
        if line.startswith("Q,"):
            parts = line.split(",", 2)
            if len(parts) < 3:
                continue
            try:
                q_ms = int(parts[1])
            except ValueError:
                continue
            q_cmd = parts[2]
            buf.clear()
        elif line.startswith("R,"):
            parts = line.split(",", 2)
            if len(parts) < 3:
                continue
            try:
                buf.extend(bytes.fromhex(parts[2]))
            except ValueError:
                continue
            if b">" in buf:
                rows.extend(_parse_command(q_cmd, q_ms, bytes(buf)))
                buf.clear()

    if not rows:
        raise ValueError(f"no PID responses in {path}")

    long = pl.DataFrame(
        rows, schema=["ms", "pid", "value", "quality"], orient="row"
    ).sort("ms")

    metadata: dict = {}
    meta_path = path.parent / "metadata.json"
    if meta_path.exists():
        try:
            metadata = json.loads(meta_path.read_text())
        except Exception:
            metadata = {}
    metadata.setdefault("session_id", path.parent.name)
    if "started_at" not in metadata:
        metadata["started_at"] = datetime.fromtimestamp(
            path.stat().st_mtime
        ).isoformat()

    return RawSession(
        id=str(metadata.get("session_id") or path.stem),
        path=path.parent,
        long=long,
        metadata=metadata,
    )


def raw_pid_names(long: pl.DataFrame) -> list[str]:
    pids = sorted(set(long["pid"].to_list()))
    return [PID_NAMES[p] for p in pids if p in PID_NAMES]
