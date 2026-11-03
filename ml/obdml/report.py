"""Markdown + PNG reports."""

from __future__ import annotations

from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np
import polars as pl

from .dataset import Session


def _mmss(ms: int) -> str:
    s = max(0, int(ms) // 1000)
    return f"{s // 60:02d}:{s % 60:02d}"


def _safe_name(name: str) -> str:
    return "".join(c if c.isalnum() or c in "-_." else "-" for c in name)


def _plot(session: Session, anomalies: pl.DataFrame, out_dir: Path) -> list[str]:
    plots = out_dir / "plots"
    plots.mkdir(parents=True, exist_ok=True)
    df = session.df
    if df.is_empty():
        return []
    ms = df["ms"].to_numpy() / 1000.0

    ams = anomalies["ms"].to_numpy() / 1000.0
    fig, ax = plt.subplots(figsize=(10, 3))
    ax.plot(ams, anomalies["score"].to_numpy(), lw=0.8, color="#1f77b4")
    vals = anomalies["score"].to_numpy()
    if len(vals):
        thresh = float(np.percentile(vals, 95))
        ax.axhline(thresh, color="red", ls="--", lw=0.7, label="p95")
        top = anomalies.sort("score", descending=True).head(10)
        ax.scatter(
            top["ms"].to_numpy() / 1000.0,
            top["score"].to_numpy(),
            color="red",
            s=12,
            zorder=3,
        )
    ax.set_title("anomaly score")
    ax.set_xlabel("tempo (s)")
    ax.grid(alpha=0.2)
    fig.tight_layout()
    score_png = plots / "score.png"
    fig.savefig(score_png, dpi=120)
    plt.close(fig)

    signal = "maf" if "maf" in df.columns else ("rpm" if "rpm" in df.columns else None)
    out = [str(score_png.relative_to(out_dir))]
    if signal:
        fig, ax = plt.subplots(figsize=(10, 3))
        ax.plot(ms, df[signal].to_numpy(), lw=0.8, color="#2ca02c")
        top = anomalies.sort("score", descending=True).head(10)
        if top.height:
            ax.scatter(
                top["ms"].to_numpy() / 1000.0,
                np.interp(top["ms"].to_numpy() / 1000.0, ms, np.nan_to_num(df[signal].to_numpy())),
                color="red",
                s=12,
                zorder=3,
            )
        ax.set_title(signal)
        ax.set_xlabel("tempo (s)")
        ax.grid(alpha=0.2)
        fig.tight_layout()
        sig_png = plots / f"signal_{signal}.png"
        fig.savefig(sig_png, dpi=120)
        plt.close(fig)
        out.append(str(sig_png.relative_to(out_dir)))
    return out


def write_session_report(
    session: Session,
    anomalies: pl.DataFrame,
    mp: pl.DataFrame,
    out_dir: Path,
) -> Path:
    out_dir.mkdir(parents=True, exist_ok=True)
    rel_plots = _plot(session, anomalies, out_dir)

    df = session.df
    total = max(df.height, 1)
    regimes = {}
    for name in ("engine_on", "idle", "moving", "moving_cold", "moving_warm"):
        if name in df.columns:
            regimes[name] = int(df[name].sum())

    top = anomalies.sort("score", descending=True).head(10)
    top_rows = "\n".join(
        f"| {_mmss(r['ms'])} | {r['ms']} | {r.get('regime', '-')} | {r['score']:.2f} | {r['top_features']} |"
        for r in top.iter_rows(named=True)
    )
    if not top_rows:
        top_rows = "| - | - | - | - |"

    disc = mp.filter(pl.col("kind") == "discord").sort("score", descending=True).head(10)
    disc_rows = "\n".join(
        f"| {r['signal']} | {r['window_s']} s | {_mmss(r['start_ms'])} | {r['score']:.2f} |"
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

    plots_md = "\n".join(f"![{Path(p).stem}]({p})" for p in rel_plots)
    md = f"""# Sessão {session.id}

- **início:** {session.metadata.get('started_at', '-')}
- **linhas 1 Hz:** {df.height}
- **duração:** {df['ms'].max() / 1000:.0f} s
- **fim:** {session.metadata.get('end_reason', '-')}
- **VIN:** {session.metadata.get('vin') or '-'}

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
