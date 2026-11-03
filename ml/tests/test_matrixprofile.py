import numpy as np
import polars as pl

from obdml.detect.matrixprofile import matrix_profile


def test_matrix_profile_finds_spike():
    n = 200
    x = np.sin(np.arange(n) / 8.0)
    x[100:103] += 8.0
    df = pl.DataFrame({"ms": np.arange(n) * 1000, "rpm": x})
    out = matrix_profile(df, signals=["rpm"], windows=[10], top=3)
    disc = out.filter(pl.col("kind") == "discord")
    assert disc.height > 0
    assert any(90_000 <= v <= 110_000 for v in disc["start_ms"].to_list())
