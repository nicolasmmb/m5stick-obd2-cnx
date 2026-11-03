import polars as pl

from obdml.ingest.inject import apply_fault


def _df():
    return pl.DataFrame(
        {
            "ms": [0, 1000, 2000, 3000, 4000],
            "maf": [2.0, 4.0, 6.0, 8.0, 10.0],
        }
    )


def test_flatline_and_dropout():
    out, (lo, hi) = apply_fault(_df(), "maf", "flatline", 2000, 2000)
    assert lo == 2000 and hi == 4000
    assert out["maf"].to_list()[2:4] == [6.0, 6.0]

    out, _ = apply_fault(_df(), "maf", "dropout", 2000, 2000)
    assert out["maf"].null_count() == 2


def test_spike_and_scale():
    out, _ = apply_fault(_df(), "maf", "spike", 1000, 1000, magnitude=10)
    assert out["maf"][1] > _df()["maf"][1]

    out, _ = apply_fault(_df(), "maf", "scale", 1000, 2000, magnitude=2)
    assert out["maf"][1] == 8.0
