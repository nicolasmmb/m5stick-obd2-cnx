"""PID table and feature names shared by the Python pipeline."""

from __future__ import annotations

# PIDs polled in the fast loop and slow loop by the firmware.
FAST_PIDS = [
    "0C", "0D", "11", "04", "0B", "10", "44", "06", "07",
    "45", "49", "4A",
]
SLOW_PIDS = [
    "05", "0F", "2F", "3C", "42", "52", "46", "4C", "33",
]

PID_NAMES = {
    "0C": "rpm",
    "0D": "speed",
    "11": "throttle",
    "04": "load",
    "0B": "map",
    "10": "maf",
    "44": "lambda",
    "06": "stft",
    "07": "ltft",
    "45": "relative_throttle",
    "49": "pedal_d",
    "4A": "pedal_e",
    "05": "coolant",
    "0F": "intake",
    "2F": "fuel_pct",
    "3C": "catalyst",
    "42": "voltage",
    "52": "ethanol",
    "46": "ambient",
    "4C": "commanded_throttle",
    "33": "baro",
}

NAME_PIDS = {v: k for k, v in PID_NAMES.items()}
FAST_FEATURES = [PID_NAMES[p] for p in FAST_PIDS]

PID_UNITS = {
    "rpm": "rpm",
    "speed": "km/h",
    "throttle": "%",
    "load": "%",
    "map": "kPa",
    "maf": "g/s",
    "lambda": "lambda",
    "stft": "%",
    "ltft": "%",
    "relative_throttle": "%",
    "pedal_d": "%",
    "pedal_e": "%",
    "coolant": "C",
    "intake": "C",
    "fuel_pct": "%",
    "catalyst": "C",
    "voltage": "V",
    "ethanol": "%",
    "ambient": "C",
    "commanded_throttle": "%",
    "baro": "kPa",
}

# Raw signals plus derived values that are available from the current logs.
BASE_FEATURES = [
    "rpm", "speed", "load", "throttle", "map", "maf", "lambda",
    "stft", "ltft", "coolant", "catalyst", "voltage", "fuel_pct",
    "relative_throttle", "pedal_d", "pedal_e", "commanded_throttle",
    "baro", "ethanol", "ambient",
]

FLAT_SIGNALS = ["rpm", "speed", "load", "maf", "map", "throttle", "coolant", "voltage"]

DERIVED_FEATURES = [
    "accel", "jerk", "rpm_per_speed", "load_per_rpm", "maf_per_load",
    "map_baro_ratio", "pedal_diff", "throttle_per_pedal",
    "coolant_slope", "catalyst_slope", "voltage_slope",
] + [f"flat_{s}" for s in FLAT_SIGNALS]

MODEL_FEATURES = BASE_FEATURES + DERIVED_FEATURES

# Univariate signals used by the matrix profile.
MP_SIGNALS = [
    "rpm", "speed", "load", "maf", "map", "throttle", "coolant", "voltage",
]

# Forward-fill limits (ms) when building the 1 Hz grid.
FAST_TOLERANCE_MS = 10_000
SLOW_TOLERANCE_MS = 30_000


def is_fast_pid(pid: str) -> bool:
    return pid in FAST_PIDS


def feature_label(name: str) -> str:
    labels = {
        "accel": "aceleração (m/s²)",
        "jerk": "jerk (m/s³)",
        "rpm_per_speed": "RPM / velocidade",
        "load_per_rpm": "carga / RPM",
        "maf_per_load": "MAF / carga",
        "map_baro_ratio": "MAP / baro",
        "pedal_diff": "pedal D − pedal E",
        "throttle_per_pedal": "borboleta / pedal",
        "coolant_slope": "derivada coolant",
        "catalyst_slope": "derivada catalisador",
        "voltage_slope": "derivada tensão",
    }
    if name.startswith("flat_"):
        return f"{name[5:]} travado (std 10 s)"
    if name in labels:
        return labels[name]
    unit = PID_UNITS.get(name, "")
    return f"{name} ({unit})" if unit else name
