"""Polars expressions for derived features."""

from __future__ import annotations

import polars as pl

from .config import FLAT_SIGNALS, MODEL_FEATURES


def _ratio(num: str, den: str, floor: float = 0.0) -> pl.Expr:
    return (
        pl.when(pl.col(den).is_not_null() & (pl.col(den) > floor))
        .then(pl.col(num) / pl.col(den))
        .otherwise(None)
    )


def add_features(df: pl.DataFrame) -> pl.DataFrame:
    if df.is_empty():
        return df

    cols = set(df.columns)
    exprs: list[pl.Expr] = []
    if "speed" in cols:
        exprs.append((pl.col("speed").diff() / 3.6).alias("accel"))
    if "fuel_pct" in cols:
        exprs.append(pl.col("fuel_pct").clip(0.0, 100.0).alias("fuel_pct"))
    out = df.with_columns(exprs) if exprs else df

    exprs = []
    if "accel" in out.columns:
        exprs.append(pl.col("accel").diff().alias("jerk"))
    pairs = [
        ("rpm", "speed", 1.0, "rpm_per_speed"),
        ("load", "rpm", 1.0, "load_per_rpm"),
        ("maf", "load", 0.1, "maf_per_load"),
        ("map", "baro", 1.0, "map_baro_ratio"),
        ("throttle", "pedal_d", 0.1, "throttle_per_pedal"),
    ]
    for num, den, floor, name in pairs:
        if num in out.columns and den in out.columns:
            exprs.append(_ratio(num, den, floor).alias(name))
    if "pedal_d" in out.columns and "pedal_e" in out.columns:
        exprs.append((pl.col("pedal_d") - pl.col("pedal_e")).alias("pedal_diff"))
    for name in ("coolant", "catalyst", "voltage"):
        if name in out.columns:
            exprs.append(pl.col(name).diff().alias(f"{name}_slope"))
    if exprs:
        out = out.with_columns(exprs)

    flat = [
        pl.col(name).rolling_std(window_size=10, min_samples=3).alias(f"flat_{name}")
        for name in FLAT_SIGNALS
        if name in out.columns
    ]
    if flat:
        out = out.with_columns(flat)

    # Ratios can produce inf when a denominator is tiny.
    exprs = []
    for name in out.columns:
        if out.schema[name].is_float():
            exprs.append(
                pl.when(pl.col(name).is_infinite()).then(None).otherwise(pl.col(name)).alias(name)
            )
    return out.with_columns(exprs) if exprs else out


def available_features(df: pl.DataFrame) -> list[str]:
    return [c for c in MODEL_FEATURES if c in df.columns]


def coverage(df: pl.DataFrame, features: list[str]) -> dict[str, float]:
    n = df.height
    if n == 0:
        return {f: 0.0 for f in features}
    return {
        f: float(n - df[f].null_count()) / n
        for f in features
        if f in df.columns
    }
