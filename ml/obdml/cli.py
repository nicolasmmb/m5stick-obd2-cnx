"""Command line entry point."""

from __future__ import annotations

import argparse
from pathlib import Path

import polars as pl

from .dataset import Session, build_session, load_sessions
from .detect.matrixprofile import matrix_profile
from .detect.multivariate import fit, score
from .detect.synthetic import evaluate as synthetic_evaluate
from .features import add_features
from .ingest.files import load_sessions as load_raw_sessions
from .ingest.inject import load_injected
from .ingest.serial import read_serial_session
from .report import write_index, write_session_report, write_synthetic


def _default_data_dir() -> str:
    if Path("../data").exists():
        return "../data"
    return "data"


def _mmss(ms: int) -> str:
    s = max(0, int(ms) // 1000)
    return f"{s // 60:02d}:{s % 60:02d}"


def _prepare(sessions: list[Session]) -> list[Session]:
    out = []
    for session in sessions:
        if session.df.is_empty():
            continue
        session.df = add_features(session.df)
        out.append(session)
    return out


def _train_frame(sessions: list[Session], regime: str) -> pl.DataFrame:
    frames = []
    for session in sessions:
        if regime in session.df.columns:
            sub = session.df.filter(pl.col(regime))
            if not sub.is_empty():
                frames.append(sub)
    if frames:
        return pl.concat(frames)
    return pl.concat([s.df for s in sessions])


def _fit(sessions: list[Session], regime: str, min_coverage: float):
    train = _train_frame(sessions, regime)
    return fit(train, regime=None, min_coverage=min_coverage)


def _report_session(
    session: Session,
    model,
    out: Path,
    windows: list[int],
    top: int,
) -> dict:
    score_df = session.df.head(0)
    if "engine_on" in session.df.columns:
        engine = session.df.filter(pl.col("engine_on"))
        if not engine.is_empty():
            score_df = engine
    anomalies = score(score_df, model)
    if "regime" in session.df.columns:
        anomalies = anomalies.join(
            session.df.select("ms", "regime"), on="ms", how="left"
        )
    mp = matrix_profile(session.df, windows=windows, top=top)
    session_out = out / session.id
    write_session_report(session, anomalies, mp, session_out)
    best = anomalies.sort("score", descending=True).head(1)
    top_score = float(best["score"][0]) if best.height else 0.0
    top_ms = int(best["ms"][0]) if best.height else 0
    print(f"{session.id}: top score {top_score:.2f} at {_mmss(top_ms)} -> {session_out}")
    return {
        "session": session.id,
        "rows": session.df.height,
        "duration_s": session.df["ms"].max() / 1000.0,
        "top_score": top_score,
        "top_time": _mmss(top_ms),
    }


def run_data(args) -> int:
    raw = load_raw_sessions(args.data, args.session)
    sessions = _prepare([build_session(r) for r in raw])
    if not sessions:
        print(f"no sessions with samples in {args.data}")
        return 1

    model = _fit(sessions, args.regime, args.min_coverage)
    out = Path(args.out)
    rows = [
        _report_session(s, model, out, args.windows, args.top) for s in sessions
    ]
    write_index(rows, out)
    print(f"index -> {out / 'index.md'}")

    if args.synthetic:
        synth = synthetic_evaluate(sessions, args.regime)
        write_synthetic(synth, out)
        print(f"synthetic -> {out / 'synthetic_eval.md'}")
        for r in synth:
            print(
                f"  {r['session']:20} {r['kind']:8} {r['column']:10} "
                f"rank={r['rank']:4} score={r['score']:.2f}"
            )
    return 0


def run_inject(args) -> int:
    raw = load_injected(args.file)
    session = build_session(raw)
    session.df = add_features(session.df)
    sessions = [session]

    train_sessions = sessions
    if args.train_data:
        train_sessions = _prepare(
            [build_session(r) for r in load_raw_sessions(args.train_data)]
        )
        if not train_sessions:
            train_sessions = sessions
    model = _fit(train_sessions, args.regime, args.min_coverage)
    out = Path(args.out)
    _report_session(session, model, out, args.windows, args.top)
    return 0


def run_serial(args) -> int:
    raw = read_serial_session(args.port, seconds=args.seconds, follow=args.follow)
    if raw is None:
        print("no P lines received")
        return 1
    session = build_session(raw)
    session.df = add_features(session.df)

    train_sessions = _prepare(
        [build_session(r) for r in load_raw_sessions(args.train_data)]
    )
    if not train_sessions:
        print("warning: no training data; fitting on the captured stream")
        train_sessions = [session]
    model = _fit(train_sessions, args.regime, args.min_coverage)
    anomalies = score(session.df, model)
    top = anomalies.sort("score", descending=True).head(args.top)
    for r in top.iter_rows(named=True):
        print(f"{_mmss(r['ms'])} score={r['score']:.2f} {r['top_features']}")
    if args.out:
        _report_session(session, model, Path(args.out), args.windows, args.top)
    return 0


def parse_args(argv: list[str] | None = None):
    parser = argparse.ArgumentParser(description="OBD-II unsupervised anomaly analysis")
    parser.add_argument("--source", choices=["data", "inject", "serial"], default="data")
    parser.add_argument("--data", default=_default_data_dir(), help="sessions directory")
    parser.add_argument("--file", help="injected CSV/NDJSON")
    parser.add_argument("--port", help="serial port for --source serial")
    parser.add_argument("--baud", type=int, default=115200)
    parser.add_argument("--seconds", type=int, default=60)
    parser.add_argument("--follow", action="store_true")
    parser.add_argument("--train-data", default=None, help="baseline data for inject/serial")
    parser.add_argument("--out", default="reports")
    parser.add_argument("--session", default=None)
    parser.add_argument("--window", default="10,30")
    parser.add_argument("--top", type=int, default=10)
    parser.add_argument("--regime", default="normal")
    parser.add_argument("--min-coverage", type=float, default=0.5)
    parser.add_argument("--synthetic", action="store_true")
    args = parser.parse_args(argv)
    args.windows = [int(x) for x in str(args.window).split(",") if x.strip()]
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.source == "data":
        return run_data(args)
    if args.source == "inject":
        if not args.file:
            raise SystemExit("--file is required for --source inject")
        return run_inject(args)
    if args.source == "serial":
        if not args.port:
            raise SystemExit("--port is required for --source serial")
        return run_serial(args)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
