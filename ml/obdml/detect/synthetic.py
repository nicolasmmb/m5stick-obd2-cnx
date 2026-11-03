"""Synthetic fault injection to validate detectors without labels."""

from __future__ import annotations

import numpy as np
import polars as pl

from ..config import DERIVED_FEATURES
from ..dataset import Session
from ..features import add_features
from ..ingest.inject import apply_fault
from .multivariate import fit, score

DEFAULT_FAULTS = [
    ("flatline", "maf"),
    ("spike", "map"),
    ("drift", "coolant"),
    ("dropout", "rpm"),
    ("scale", "throttle"),
]


def _train_frame(sessions: list[Session], regime: str) -> pl.DataFrame:
    frames = []
    for s in sessions:
        if regime in s.df.columns:
            sub = s.df.filter(pl.col(regime))
            if not sub.is_empty():
                frames.append(sub)
    if frames:
        return pl.concat(frames)
    return pl.concat([s.df for s in sessions])


def evaluate(
    sessions: list[Session],
    regime: str = "normal",
    faults: list[tuple[str, str]] | None = None,
    duration_ms: int = 20_000,
    magnitude: float = 5.0,
) -> list[dict]:
    faults = faults or DEFAULT_FAULTS
    train = _train_frame(sessions, regime)
    model = fit(train, regime=None)

    rows: list[dict] = []
    for session in sessions:
        if session.df.is_empty():
            continue
        start = int(session.df["ms"].max() * 0.35)
        for kind, column in faults:
            if column not in session.df.columns:
                continue
            try:
                faulty, (lo, hi) = apply_fault(
                    session.df, column, kind, start, duration_ms, magnitude
                )
            except Exception:
                continue
            # Rebuild derived features after the injection.
            faulty = faulty.select(
                [c for c in faulty.columns if c not in DERIVED_FEATURES]
            )
            faulty = add_features(faulty)
            scored = score(faulty, model)
            values = scored["score"].to_numpy()
            ms = scored["ms"].to_numpy()
            valid = (ms > 5_000) & (ms < int(ms.max()) - 5_000)
            if "normal" in faulty.columns:
                valid &= faulty["normal"].to_numpy()
            if not valid.any():
                continue
            values = values[valid]
            ms = ms[valid]
            window = (ms >= lo) & (ms < hi)
            if not window.any():
                continue
            peak = float(np.max(values[window]))
            rank = int((values > peak).sum()) + 1
            rows.append(
                {
                    "session": session.id,
                    "kind": kind,
                    "column": column,
                    "rank": rank,
                    "score": peak,
                }
            )
    return rows
