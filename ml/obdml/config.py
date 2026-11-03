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



# Nomes legíveis para relatórios e gráficos.
FEATURE_LABELS = {
    "rpm": ("RPM (rotações por minuto)", "RPM"),
    "speed": ("velocidade do veículo", "velocidade"),
    "load": ("carga calculada do motor", "carga"),
    "throttle": ("posição da borboleta", "borboleta"),
    "map": ("pressão absoluta do coletor (MAP)", "MAP"),
    "maf": ("fluxo de ar admitido (MAF)", "MAF"),
    "lambda": ("razão ar/combustível (lambda)", "lambda"),
    "stft": ("correção de combustível de curto prazo (STFT)", "STFT"),
    "ltft": ("correção de combustível de longo prazo (LTFT)", "LTFT"),
    "coolant": ("temperatura do líquido de arrefecimento", "coolant"),
    "catalyst": ("temperatura do catalisador", "catalisador"),
    "voltage": ("tensão da bateria/rede", "tensão"),
    "fuel_pct": ("nível de combustível", "combustível"),
    "relative_throttle": ("borboleta relativa", "borb. relativa"),
    "pedal_d": ("pedal D (posição do acelerador)", "pedal D"),
    "pedal_e": ("pedal E (posição do acelerador)", "pedal E"),
    "commanded_throttle": ("borboleta comandada", "borb. comandada"),
    "baro": ("pressão barométrica", "baro"),
    "ethanol": ("teor de etanol no combustível", "etanol"),
    "ambient": ("temperatura ambiente", "ambiente"),
    "intake": ("temperatura do ar admitido (IAT)", "IAT"),
    "accel": ("aceleração do veículo", "aceleração"),
    "jerk": ("variação da aceleração (jerk)", "jerk"),
    "rpm_per_speed": ("RPM dividido pela velocidade", "RPM/velocidade"),
    "load_per_rpm": ("carga dividida pelo RPM", "carga/RPM"),
    "maf_per_load": ("MAF dividido pela carga", "MAF/carga"),
    "map_baro_ratio": ("MAP dividido pela pressão barométrica", "MAP/baro"),
    "pedal_diff": ("diferença entre pedal D e pedal E", "pedal D − E"),
    "throttle_per_pedal": ("borboleta dividida pelo pedal", "borboleta/pedal"),
    "coolant_slope": ("variação do coolant por segundo", "Δ coolant"),
    "catalyst_slope": ("variação do catalisador por segundo", "Δ catalisador"),
    "voltage_slope": ("variação da tensão por segundo", "Δ tensão"),
}

GLOSSARY = [
    ("OBD-II", "padrão de diagnóstico de bordo do carro"),
    ("ECU", "unidade de controle do motor"),
    ("PID", "parâmetro identificado do OBD-II"),
    ("CAN", "rede interna do carro"),
    ("RPM", "rotações por minuto do motor"),
    ("MAF", "fluxo de massa de ar admitido"),
    ("MAP", "pressão absoluta do coletor de admissão"),
    ("IAT", "temperatura do ar admitido"),
    ("ECT", "temperatura do líquido de arrefecimento"),
    ("pedal D/E", "sensores redundantes da posição do acelerador"),
    ("STFT", "correção de combustível de curto prazo"),
    ("LTFT", "correção de combustível de longo prazo"),
    ("coolant", "temperatura do líquido de arrefecimento (ECT)"),
    ("lambda", "razão ar/combustível normalizada"),
    ("AFR", "razão ar/combustível"),
    ("DTC", "código de falha de diagnóstico"),
    ("MIL", "luz de anomalia do painel (check engine)"),
    ("readiness", "status dos monitores de emissão"),
    ("VIN", "número de identificação do veículo"),
    ("BLE", "Bluetooth Low Energy"),
]


def feature_label(name: str, short: bool = False) -> str:
    if name.startswith("flat_"):
        base = feature_label(name[5:], short=True)
        return f"{base} sem variação (10 s)"
    if name.startswith("age_"):
        base = feature_label(name[4:], short=True)
        return f"idade de {base}"
    pair = FEATURE_LABELS.get(name)
    if pair:
        return pair[1] if short else pair[0]
    unit = PID_UNITS.get(name, "")
    return f"{name} ({unit})" if unit else name


def format_features(expr: str) -> str:
    return ", ".join(feature_label(x.strip()) for x in expr.split(";") if x.strip())
