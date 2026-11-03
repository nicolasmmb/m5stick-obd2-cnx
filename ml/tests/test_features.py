import polars as pl

from obdml.features import add_features


def test_accel_and_pedal_diff():
    df = pl.DataFrame(
        {
            "ms": [0, 1000, 2000],
            "speed": [0.0, 36.0, 72.0],
            "pedal_d": [20.0, 30.0, 40.0],
            "pedal_e": [10.0, 12.0, 15.0],
            "fuel_pct": [50.0, 50.5, 60.0],
            "rpm": [800.0, 1200.0, 1600.0],
            "load": [10.0, 20.0, 30.0],
            "maf": [2.0, 4.0, 6.0],
            "map": [40.0, 50.0, 60.0],
            "baro": [95.0, 95.0, 95.0],
            "throttle": [20.0, 30.0, 40.0],
            "coolant": [80.0, 80.0, 80.0],
            "catalyst": [400.0, 420.0, 440.0],
            "voltage": [13.0, 13.1, 13.2],
            "relative_throttle": [10.0, 20.0, 30.0],
            "commanded_throttle": [15.0, 25.0, 35.0],
            "stft": [0.0, 1.0, 2.0],
            "ltft": [-1.0, 0.0, 1.0],
            "intake": [30.0, 30.0, 30.0],
            "ethanol": [41.0, 41.0, 41.0],
            "ambient": [25.0, 25.0, 25.0],
        }
    )
    out = add_features(df)
    assert out["accel"].to_list()[1] == 10.0  # 36 km/h / 3.6
    assert out["pedal_diff"].to_list() == [10.0, 18.0, 25.0]
    assert out["map_baro_ratio"].to_list()[0] == 40.0 / 95.0
