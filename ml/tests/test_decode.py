from obdml.ingest.decode import decode_pid


def test_decode_common_pids():
    assert decode_pid("0C", "1AF8")[0] == 1726.0
    assert decode_pid("0D", "1E")[0] == 30.0
    assert decode_pid("05", "73")[0] == 75.0
    assert decode_pid("11", "50")[0] == 80 * 100 / 255
    assert round(decode_pid("42", "30FB")[0], 3) == 12.539
    assert round(decode_pid("3C", "1554")[0], 1) == 506.0
    assert decode_pid("ZZ", "00") is None
