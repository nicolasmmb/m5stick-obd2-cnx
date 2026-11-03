"""Markdown + PNG reports with readable charts."""

from __future__ import annotations

from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as mticker
import numpy as np
import polars as pl
from matplotlib.patches import Patch

from .config import GLOSSARY, PID_NAMES, PID_UNITS, feature_label, format_features
from .dataset import Session

REGIME_COLORS = {
    "no_data": "#d9d9d9",
    "engine_off": "#9e9e9e",
    "idle": "#4c78a8",
    "idle_cold": "#83b6e8",
    "moving_cold": "#f58518",
    "moving_warm": "#54a24b",
}
REGIME_LABELS = {
    "no_data": "sem dados",
    "engine_off": "motor desligado",
    "idle": "marcha lenta",
    "idle_cold": "marcha lenta fria",
    "moving_cold": "andando frio",
    "moving_warm": "andando quente",
}


def _mmss(ms: int | float) -> str:
    s = max(0, int(ms) // 1000)
    return f"{s // 60:02d}:{s % 60:02d}"


def _time_fmt(x, _pos) -> str:
    return _mmss(float(x) * 1000)


def _regime_spans(df: pl.DataFrame) -> list[tuple[int, int, str]]:
    if "regime" not in df.columns or df.is_empty():
        return []
    ms = df["ms"].to_numpy()
    names = df["regime"].to_list()
    spans: list[tuple[int, int, str]] = []
    start = int(ms[0])
    current = names[0]
    for i in range(1, len(ms)):
        if names[i] != current:
            spans.append((start, int(ms[i]), current))
            start = int(ms[i])
            current = names[i]
    spans.append((start, int(ms[-1]) + 1000, current))
    return spans


def _event_counts(session: Session, max_ms: int) -> tuple[np.ndarray, np.ndarray]:
    if session.events is None or session.events.is_empty():
        return np.zeros(0), np.zeros(0)
    ev = session.events
    if "type" not in ev.columns or "ms" not in ev.columns:
        return np.zeros(0), np.zeros(0)
    ev = ev.filter(pl.col("type").is_in(["NO_DATA", "PARSE_ERROR"]))
    if ev.is_empty():
        return np.zeros(0), np.zeros(0)
    seconds = ev["ms"].to_numpy() / 1000.0
    max_s = max(1.0, max_ms / 1000.0)
    bins = np.arange(0, max_s + 2, 1.0)
    counts, edges = np.histogram(seconds, bins=bins)
    return edges[:-1], counts


def _score_plot(
    session: Session,
    anomalies: pl.DataFrame,
    out: Path,
) -> str:
    df = session.df
    max_ms = int(df["ms"].max())
    fig = plt.figure(figsize=(11, 6.2), constrained_layout=True)
    gs = fig.add_gridspec(3, 1, height_ratios=[3.2, 0.9, 0.9])

    ax = fig.add_subplot(gs[0])
    full = df.select("ms").join(
        anomalies.select("ms", "score"), on="ms", how="left"
    )
    ams = full["ms"].to_numpy() / 1000.0
    vals = full["score"].to_numpy()
    ax.plot(ams, vals, lw=1.0, color="#1f77b4", label="score")
    finite = vals[np.isfinite(vals)]
    if len(finite):
        p95 = float(np.percentile(finite, 95))
        ax.axhline(p95, color="#d62728", ls="--", lw=0.9, label=f"p95 = {p95:.1f}")
        top = anomalies.sort("score", descending=True).head(5)
        ax.scatter(
            top["ms"].to_numpy() / 1000.0,
            top["score"].to_numpy(),
            color="#d62728",
            s=28,
            zorder=3,
            label="top 5",
        )
        for r in top.head(3).iter_rows(named=True):
            x = r["ms"] / 1000.0
            y = r["score"]
            label = (
                f"{_mmss(r['ms'])} · {r.get('regime', '-')}\n"
                f"{feature_label(r['top_features'].split(';')[0], short=True)}"
            )
            ax.annotate(
                label,
                (x, y),
                xytext=(6, 10),
                textcoords="offset points",
                fontsize=7,
                arrowprops={"arrowstyle": "-", "color": "#d62728", "lw": 0.6},
            )
    ax.set_ylabel("score (maior = mais fora do normal)")
    if len(finite):
        ax.set_ylim(0, max(1.0, float(np.max(finite))) * 1.35)
    ax.grid(alpha=0.25)
    ax.legend(loc="upper right", fontsize=8)
    ax.xaxis.set_major_formatter(mticker.FuncFormatter(_time_fmt))
    ax.set_xlim(0, max_ms / 1000.0)

    axr = fig.add_subplot(gs[1], sharex=ax)
    handles: dict[str, Patch] = {}
    for start, end, name in _regime_spans(df):
        color = REGIME_COLORS.get(name, "#cccccc")
        axr.axvspan(start / 1000.0, end / 1000.0, color=color, alpha=0.85)
        handles.setdefault(name, Patch(color=color, label=REGIME_LABELS.get(name, name)))
    axr.set_yticks([])
    axr.set_ylabel("regime")
    if handles:
        axr.legend(handles=list(handles.values()), loc="upper left", ncol=3, fontsize=7, frameon=False)
    axr.xaxis.set_major_formatter(mticker.FuncFormatter(_time_fmt))
    axr.set_xlim(0, max_ms / 1000.0)

    axe = fig.add_subplot(gs[2], sharex=ax)
    seconds, counts = _event_counts(session, max_ms)
    if len(counts):
        axe.bar(seconds, counts, width=0.95, color="#d62728", alpha=0.7)
        axe.set_ylabel("eventos/s")
    else:
        axe.text(0.5, 0.5, "sem eventos NO_DATA/PARSE_ERROR", transform=axe.transAxes, ha="center", va="center")
        axe.set_yticks([])
    axe.set_xlabel("tempo (min:seg)")
    axe.grid(alpha=0.2, axis="y")
    axe.xaxis.set_major_formatter(mticker.FuncFormatter(_time_fmt))
    axe.set_xlim(0, max_ms / 1000.0)

    fig.suptitle(f"Sessão {session.id} — score de anomalia", fontsize=12)
    path = out / "score.png"
    fig.savefig(path, dpi=130)
    plt.close(fig)
    return str(path.relative_to(out.parent))


def _signals_plot(
    session: Session,
    anomalies: pl.DataFrame,
    out: Path,
) -> str:
    df = session.df
    max_ms = int(df["ms"].max())
    ms = df["ms"].to_numpy() / 1000.0
    top = anomalies.sort("score", descending=True).head(5) if not anomalies.is_empty() else anomalies
    top_x = top["ms"].to_numpy() / 1000.0 if not top.is_empty() else np.array([])

    panels = [
        ("maf", "MAF — fluxo de ar (g/s)", "maf"),
        ("map", "MAP — pressão do coletor (kPa)", "map"),
        ("pedal_d", "Pedal D/E — acelerador (%)", "pedal_d"),
        ("coolant", "Coolant — temp. do líquido (°C)", "coolant"),
    ]
    fig, axes = plt.subplots(2, 2, figsize=(11, 6.2), sharex=True, constrained_layout=True)
    for ax, (column, label, kind) in zip(axes.flat, panels):
        if column not in df.columns:
            ax.set_visible(False)
            continue
        if kind == "pedal_d":
            for c, color, name in (("pedal_d", "#1f77b4", "pedal D"), ("pedal_e", "#ff7f0e", "pedal E")):
                if c in df.columns:
                    ax.plot(ms, df[c].to_numpy(), lw=0.9, color=color, label=name)
            ax.legend(loc="upper right", fontsize=7)
        else:
            ax.plot(ms, df[column].to_numpy(), lw=0.9, color="#2ca02c")
        for i, x in enumerate(top_x):
            ax.axvline(
                x,
                color="#d62728",
                alpha=0.25,
                lw=0.7,
                label="top 5 anomalias" if i == 0 else None,
            )
        ax.set_ylabel(label)
        if len(top_x):
            ax.legend(loc="upper right", fontsize=7)
        ax.grid(alpha=0.25)
        ax.set_xlim(0, max_ms / 1000.0)
        ax.xaxis.set_major_formatter(mticker.FuncFormatter(_time_fmt))
    fig.suptitle(f"Sinais e top anomalias — {session.id}", fontsize=11)
    for ax in axes[1]:
        ax.set_xlabel("tempo (min:seg)")
    path = out / "signals.png"
    fig.savefig(path, dpi=130)
    plt.close(fig)
    return str(path.relative_to(out.parent))


def _plot(session: Session, anomalies: pl.DataFrame, out_dir: Path) -> list[str]:
    plots = out_dir / "plots"
    plots.mkdir(parents=True, exist_ok=True)
    if session.df.is_empty():
        return []
    return [
        _score_plot(session, anomalies, plots),
        _signals_plot(session, anomalies, plots),
    ]


def _glossary_md(df: pl.DataFrame) -> str:
    lines = ["| Sigla | Significado |", "|---|---|"]
    lines += [f"| {a} | {b} |" for a, b in GLOSSARY]
    lines += ["", "### PIDs usados nesta sessão", "", "| PID | Nome | Unidade |", "|---|---|---|"]
    lines += [
        f"| `{pid}` | {feature_label(name)} | {PID_UNITS.get(name, '-')} |"
        for pid, name in PID_NAMES.items()
        if name in df.columns
    ]
    lines += [
        "",
        "### Como ler os campos",
        "",
        "| Campo | Significado |",
        "|---|---|",
        "| `score` | maior valor entre `recon_z`, `mahal_z` e missing; quanto maior, mais fora do normal |",
        "| `recon_z` | erro de reconstrução do PCA, em z-score robusto |",
        "| `mahal_z` | distância de Mahalanobis; outlier multivariado |",
        "| `regime` | `engine_off`, `idle`, `idle_cold`, `moving_cold`, `moving_warm` ou `no_data` |",
        "| `NO_DATA` | o PID não respondeu naquela consulta |",
        "| `PARSE_ERROR` | a resposta do adaptador veio em formato inválido |",
        "",
        "### Regimes",
        "",
        "| Regime | Significado |",
        "|---|---|",
        "| `engine_off` | motor desligado |",
        "| `idle` | motor ligado e carro parado |",
        "| `idle_cold` | marcha lenta com coolant abaixo de 70 °C |",
        "| `moving_cold` | carro andando com coolant abaixo de 70 °C |",
        "| `moving_warm` | carro andando com coolant em 70 °C ou mais |",
        "| `no_data` | sem leitura de RPM; normalmente perda de link |",
    ]
    return "\n".join(lines)


def write_session_report(
    session: Session,
    anomalies: pl.DataFrame,
    mp: pl.DataFrame,
    out_dir: Path,
) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    rel_plots = _plot(session, anomalies, out_dir)

    df = session.df
    regimes = {}
    for name in ("engine_on", "idle", "moving", "moving_cold", "moving_warm", "no_data"):
        if name in df.columns:
            regimes[name] = int(df[name].sum())

    top = anomalies.sort("score", descending=True).head(10)
    top_rows = "\n".join(
        f"| {_mmss(r['ms'])} | {r['ms']} | {r.get('regime', '-')} | {r['score']:.2f} | {format_features(r['top_features'])} |"
        for r in top.iter_rows(named=True)
    )
    if not top_rows:
        top_rows = "| - | - | - | - |"

    disc = mp.filter(pl.col("kind") == "discord").sort("score", descending=True).head(10)
    disc_rows = "\n".join(
        f"| {feature_label(r['signal'])} | {r['window_s']} s | {_mmss(r['start_ms'])} | {r['score']:.2f} |"
        for r in disc.iter_rows(named=True)
    )
    if not disc_rows:
        disc_rows = "| - | - | - | - |"

    events = session.events
    ev_rows = ""
    if events is not None and not events.is_empty():
        counts = events.group_by("type").len().sort("len", descending=True)
        ev_rows = "\n".join(
            f"| {r['type']} | {r['len']} |" for r in counts.iter_rows(named=True)
        )

    note = ""
    if regimes.get("engine_on", 0) == 0:
        note = "\n> Sessão sem motor ligado; o score não é comparável com o regime normal.\n"

    plots_md = "\n".join(f"![{Path(p).stem}]({p})" for p in rel_plots)
    md = f"""# Sessão {session.id}

- **início:** {session.metadata.get('started_at', '-')}
- **linhas 1 Hz:** {df.height}
- **duração:** {df['ms'].max() / 1000:.0f} s
- **fim:** {session.metadata.get('end_reason', '-')}
- **VIN:** {session.metadata.get('vin') or '-'}
{note}
## Regimes

| Regime | Segundos |
|---|---:|
""" + "\n".join(f"| {k} | {v} |" for k, v in regimes.items()) + f"""

## Top anomalias

| tempo | ms | regime | score | features responsáveis |
|---|---:|---|---:|---|
{top_rows}

## Matrix profile — discords

| sinal | janela | início | score |
|---|---|---:|---:|
{disc_rows}

## Eventos

| tipo | quantidade |
|---|---:|
{ev_rows or '| - | - |'}

## Glossário

{_glossary_md(df)}

## Gráficos

{plots_md or '- '}
"""
    path = out_dir / "summary.md"
    path.write_text(md)
    anomalies.write_csv(out_dir / "anomalies.csv")
    mp.write_csv(out_dir / "matrix_profile.csv")
    return path


def write_index(rows: list[dict], out_dir: Path) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    body = "\n".join(
        f"| {r['session']} | {r['rows']} | {r['duration_s']:.0f} | {r['top_score']:.2f} | {r['top_time']} |"
        for r in rows
    )
    path = out_dir / "index.md"
    path.write_text(
        "# Índice de sessões\n\n"
        "| sessão | linhas | duração (s) | maior score | tempo |\n"
        "|---|---:|---:|---:|---:|\n" + body + "\n"
    )
    return path


def write_synthetic(rows: list[dict], out_dir: Path) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    body = "\n".join(
        f"| {r['session']} | {r['kind']} | {r['column']} | {r['rank']} | {r['score']:.2f} |"
        for r in rows
    )
    path = out_dir / "synthetic_eval.md"
    path.write_text(
        "# Avaliação com falhas sintéticas\n\n"
        "| sessão | falha | sensor | rank | score |\n"
        "|---|---|---|---:|---:|\n" + body + "\n"
    )
    return path
