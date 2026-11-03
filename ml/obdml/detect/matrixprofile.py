"""Matrix profile motifs and discords (no labels)."""

from __future__ import annotations

import warnings

import numpy as np
import polars as pl
import stumpy

from ..config import MP_SIGNALS

_MP_SCHEMA = {
    "signal": pl.Utf8,
    "window_s": pl.Int64,
    "kind": pl.Utf8,
    "start_ms": pl.Int64,
    "end_ms": pl.Int64,
    "score": pl.Float64,
}


def _fill_gaps(x: np.ndarray) -> np.ndarray | None:
    good = np.isfinite(x)
    if good.sum() < 4:
        return None
    if good.all():
        return x
    idx = np.arange(len(x))
    return np.interp(idx, idx[good], x[good])


def _non_overlapping(order: np.ndarray, radius: int, top: int) -> list[int]:
    picked: list[int] = []
    for raw in order:
        i = int(raw)
        if all(abs(i - j) >= radius for j in picked):
            picked.append(i)
        if len(picked) >= top:
            break
    return picked


def _rows_for(
    values: list[tuple[str, int, str, int, int, float]],
) -> pl.DataFrame:
    if not values:
        return pl.DataFrame(schema=_MP_SCHEMA)
    return pl.DataFrame(values, schema=_MP_SCHEMA, orient="row")


def matrix_profile(
    df: pl.DataFrame,
    signals: list[str] | None = None,
    windows: list[int] | None = None,
    top: int = 5,
) -> pl.DataFrame:
    if df.is_empty():
        return pl.DataFrame(schema=_MP_SCHEMA)

    signals = signals or MP_SIGNALS
    windows = windows or [10, 30]
    ms = df["ms"].to_numpy()
    rows: list[tuple[str, int, str, int, int, float]] = []

    for signal in signals:
        if signal not in df.columns:
            continue
        x = df[signal].to_numpy().astype(float)
        x = _fill_gaps(x)
        if x is None:
            continue
        n = len(x)
        for window_s in windows:
            m = int(window_s)
            if m < 3 or m > n // 2:
                continue
            try:
                with warnings.catch_warnings():
                    warnings.filterwarnings("ignore", message="A large number of values in `P`")
                    mp = stumpy.stump(x, m, ignore_trivial=True)
            except Exception:
                continue
            profile = np.asarray(mp[:, 0], dtype=float)
            if not np.isfinite(profile).any():
                continue

            order = np.argsort(profile)[::-1]
            for i in _non_overlapping(order, max(1, m // 2), top):
                start = int(ms[i])
                end = int(ms[min(i + m - 1, n - 1)])
                rows.append((signal, window_s, "discord", start, end, float(profile[i])))

            order = np.argsort(profile)
            for i in _non_overlapping(order, max(1, m // 2), max(1, top // 2)):
                start = int(ms[i])
                end = int(ms[min(i + m - 1, n - 1)])
                rows.append((signal, window_s, "motif", start, end, float(profile[i])))

    return _rows_for(rows)
