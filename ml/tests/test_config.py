from obdml.config import feature_label, format_features


def test_feature_labels_explain_acronyms():
    assert "fluxo" in feature_label("maf")
    assert "pressão" in feature_label("map")
    assert "arrefecimento" in feature_label("coolant")
    assert "pedal" in feature_label("pedal_diff")
    assert "sem variação" in feature_label("flat_maf")


def test_format_features():
    text = format_features("maf;coolant")
    assert "MAF" in text
    assert "arrefecimento" in text
    assert ";" not in text
