"""Load saved logger sessions from data/<session>/."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

import polars as pl


@dataclass
class RawSession:
    id: str
    path: Path
    long: pl.DataFrame
    metadata: dict = field(default_factory=dict)
    events: pl.DataFrame | None = None

    @property
    def started_at(self) -> datetime | None:
        raw = self.metadata.get("started_at")
        if not raw:
            return None
        try:
            return datetime.fromisoformat(raw)
        except ValueError:
            return None


def _read_events(path: Path) -> pl.DataFrame | None:
    if not path.exists() or path.stat().st_size == 0:
        return None
    rows: list[tuple[str, int, str, str]] = []
    for raw in path.read_text().splitlines():
        line = raw.strip()
        if not line:
            continue
        if line.startswith("{"):
            try:
                obj = json.loads(line)
                rows.append(
                    (
                        str(obj.get("t", "E")),
                        int(obj.get("ms", 0)),
                        str(obj.get("type", "")),
                        str(obj.get("msg", "")),
                    )
                )
            except Exception:
                continue
            continue
        parts = line.split(",", 3)
        if len(parts) >= 3:
            try:
                rows.append((parts[0], int(parts[1]), parts[2], parts[3] if len(parts) > 3 else ""))
            except ValueError:
                continue
    if not rows:
        return None
    return pl.DataFrame(rows, schema=["t", "ms", "type", "msg"], orient="row")


def load_session(path: Path) -> RawSession | None:
    csv = path / "samples_long.csv"
    if not csv.exists() or csv.stat().st_size == 0:
        return None
    try:
        long = pl.read_csv(csv)
    except Exception:
        return None
    if long.is_empty() or "pid" not in long.columns or "value" not in long.columns:
        return None

    long = long.select(
        pl.col("ms").cast(pl.Int64),
        pl.col("pid").cast(pl.Utf8),
        pl.col("value").cast(pl.Float64),
        pl.col("quality").cast(pl.Utf8) if "quality" in long.columns else pl.lit("OK").alias("quality"),
    ).drop_nulls(["ms", "pid", "value"]).sort("ms")

    metadata: dict = {}
    meta_path = path / "metadata.json"
    if meta_path.exists():
        try:
            metadata = json.loads(meta_path.read_text())
        except Exception:
            metadata = {}

    sid = str(metadata.get("session_id") or path.name)
    return RawSession(
        id=sid,
        path=path,
        long=long,
        metadata=metadata,
        events=_read_events(path / "events.jsonl"),
    )


def load_sessions(data_dir: str | Path, session: str | None = None) -> list[RawSession]:
    root = Path(data_dir)
    if not root.exists():
        return []
    out: list[RawSession] = []
    for path in sorted(root.iterdir()):
        if not path.is_dir():
            continue
        if session and path.name != session and session not in path.name:
            continue
        raw = load_session(path)
        if raw is not None:
            out.append(raw)
    return out
