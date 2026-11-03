"""Injected values and synthetic sensor faults."""

from __future__ import annotations

from pathlib import Path

import numpy as np
import polars as pl

from ..config import NAME_PIDS, PID_NAMES
from .decode import decode_pid
from .files import RawSession


def _wide_to_long(df: pl.DataFrame) -> pl.DataFrame:
    if "pid" in df.columns and "value" in df.columns:
        return df.select(
            pl.col("ms").cast(pl.Int64),
            pl.col("pid").cast(pl.Utf8),
            pl.col("value").cast(pl.Float64),
        )

    if "raw" in df.columns and "pid" in df.columns:
        rows = []
        for row in df.iter_rows(named=True):
            decoded = decode_pid(str(row["pid"]), str(row["raw"]))
            if decoded is not None:
                rows.append((int(row["ms"]), str(row["pid"]), float(decoded[0])))
        return pl.DataFrame(rows, schema=["ms", "pid", "value"], orient="row")

    if "ms" not in df.columns:
        raise ValueError("injected file needs ms and pid/value (or wide columns)")

    value_cols = [c for c in df.columns if c != "ms" and c in NAME_PIDS]
    if not value_cols:
        raise ValueError("no known PID columns in injected file")

    long = df.select("ms", *value_cols).unpivot(
        index="ms", variable_name="name", value_name="value"
    )
    long = long.with_columns(
        pl.col("name").replace_strict(NAME_PIDS, default=None).alias("pid"),
        pl.col("ms").cast(pl.Int64),
        pl.col("value").cast(pl.Float64),
    ).drop_nulls(["pid", "value"]).select("ms", "pid", "value")
    return long


def load_injected(path: str | Path) -> RawSession:
    path = Path(path)
    if path.suffix == ".ndjson":
        df = pl.read_ndjson(path)
    else:
        df = pl.read_csv(path)
    long = _wide_to_long(df).sort("ms")
    return RawSession(
        id=path.stem,
        path=path,
        long=long,
        metadata={"started_at": None, "session_id": path.stem},
    )


def apply_fault(
    df: pl.DataFrame,
    column: str,
    kind: str,
    start_ms: int,
    duration_ms: int,
    magnitude: float = 5.0,
) -> tuple[pl.DataFrame, tuple[int, int]]:
    """Return a copy with one synthetic fault and the affected window."""
    if column not in df.columns:
        raise ValueError(f"missing column {column}")
    end_ms = start_ms + duration_ms
    mask = (pl.col("ms") >= start_ms) & (pl.col("ms") < end_ms)
    series = df[column].to_numpy().astype(float)
    std = float(np.nanstd(series))
    if not np.isfinite(std) or std < 1e-9:
        std = 1.0
    before = df.filter(pl.col("ms") <= start_ms)[column].drop_nulls()
    base = float(before[-1]) if before.len() else float(np.nanmedian(series))

    if kind == "flatline":
        expr = pl.lit(base)
    elif kind == "spike":
        expr = pl.col(column) + magnitude * std
    elif kind == "drift":
        span = max(duration_ms, 1)
        expr = pl.col(column) + (
            (pl.col("ms") - start_ms).cast(pl.Float64) / span * magnitude * std
        )
    elif kind == "dropout":
        expr = pl.lit(None, dtype=pl.Float64)
    elif kind == "scale":
        expr = pl.col(column) * magnitude
    else:
        raise ValueError(f"unknown fault kind {kind}")

    out = df.with_columns(
        pl.when(mask).then(expr).otherwise(pl.col(column)).alias(column)
    )
    return out, (start_ms, end_ms)


def inject_from_long(
    long: pl.DataFrame,
    pid: str,
    kind: str,
    start_ms: int,
    duration_ms: int,
    magnitude: float = 5.0,
) -> tuple[pl.DataFrame, tuple[int, int]]:
    """Inject a fault into a long PID table by PID code."""
    name = PID_NAMES.get(pid)
    if name is None:
        raise ValueError(f"unknown pid {pid}")
    wide = (
        long.select("ms", "pid", "value")
        .pivot(on="pid", index="ms", values="value", aggregate_function="last")
    )
    wide = wide.rename({k: v for k, v in PID_NAMES.items() if k in wide.columns})
    wide, window = apply_fault(wide, name, kind, start_ms, duration_ms, magnitude)
    long_out = wide.unpivot(index="ms", variable_name="name", value_name="value")
    long_out = long_out.with_columns(
        pl.col("name").replace_strict(NAME_PIDS, default=None).alias("pid"),
        pl.col("value").cast(pl.Float64),
    ).drop_nulls("pid").select("ms", "pid", "value").sort("ms")
    return long_out, window
