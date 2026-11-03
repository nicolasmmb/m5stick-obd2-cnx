from pathlib import Path

import polars as pl

from obdml.dataset import RawSession, add_regimes, build_session, build_wide
from obdml.features import add_features


def _long():
    rows = []
    for i, ms in enumerate([0, 1000, 2000, 3000]):
        rows.extend(
            [
                (ms, "0C", 800.0 + i * 10, "OK"),
                (ms, "0D", 0.0 if i < 2 else 20.0, "OK"),
                (ms, "05", 80.0, "OK"),
                (ms, "42", 14.0 if i else 0.0, "OK"),
                (ms, "49", 20.0, "OK"),
                (ms, "4A", 10.0, "OK"),
            ]
        )
    return pl.DataFrame(rows, schema=["ms", "pid", "value", "quality"], orient="row")


def test_build_wide_regimes_and_voltage():
    wide = build_wide(_long())
    assert wide.height == 4
    assert wide["rpm"].to_list() == [800.0, 810.0, 820.0, 830.0]
    assert wide["voltage"].null_count() == 1  # 0 V is invalid
    out = add_regimes(wide)
    assert out["engine_on"].to_list() == [True, True, True, True]
    assert out["moving"].to_list() == [False, False, True, True]
    assert out["normal"].to_list() == [True, True, True, True]
    assert out["regime"].to_list()[:2] == ["idle", "idle"]


def test_build_session_adds_timestamp_and_features():
    raw = RawSession(id="t", path=Path("."), long=_long(), metadata={})
    session = build_session(raw)
    assert "ts" in session.df.columns
    out = add_features(session.df)
    assert "accel" in out.columns
    assert "flat_rpm" in out.columns
    assert out["accel"].drop_nulls().len() == 3
