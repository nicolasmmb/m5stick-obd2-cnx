import numpy as np
import polars as pl

from obdml.detect.multivariate import fit, score


def test_outlier_gets_highest_score():
    rng = np.random.default_rng(7)
    n = 240
    rpm = 1000 + 200 * rng.normal(size=n)
    speed = rpm / 40 + 5 * rng.normal(size=n)
    train = pl.DataFrame(
        {"ms": np.arange(n) * 1000, "rpm": rpm, "speed": speed}
    )
    model = fit(train, features=["rpm", "speed"])

    df = pl.concat(
        [
            train,
            pl.DataFrame({"ms": [n * 1000], "rpm": [9000.0], "speed": [220.0]}),
        ]
    )
    out = score(df, model).sort("score", descending=True)
    assert out["ms"][0] == n * 1000
    assert out["score"][0] > out["score"][1]
    assert "rpm" in out["top_features"][0] or "speed" in out["top_features"][0]
