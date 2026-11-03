from pathlib import Path

from obdml.ingest.raw import load_raw_log


def test_load_raw_log(tmp_path: Path):
    raw = tmp_path / "raw_elm.log"
    raw.write_text(
        "\n".join(
            [
                "2026-01-01T00:00:00\tQ,1000,010C",
                "2026-01-01T00:00:00\tR,1100,34313043313139374141414141410D",
                "2026-01-01T00:00:00\tR,1110,0D3E",
                "2026-01-01T00:00:00\tQ,2000,010D",
                "2026-01-01T00:00:00\tR,2100,34313044314541414141414141410D0D3E",
            ]
        )
    )
    session = load_raw_log(raw)
    assert session.long.height == 2
    rows = {r["pid"]: r for r in session.long.to_dicts()}
    assert abs(rows["0C"]["value"] - 1125.75) < 1e-9
    assert rows["0D"]["value"] == 30.0
