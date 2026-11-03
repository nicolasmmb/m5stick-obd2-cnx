"""PCA reconstruction + Mahalanobis anomaly score (no labels)."""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np
import polars as pl
from sklearn.decomposition import PCA
from sklearn.preprocessing import RobustScaler

from ..config import BASE_FEATURES, FAST_FEATURES
from ..features import available_features


@dataclass
class Matrix:
    X: np.ndarray
    ms: np.ndarray
    features: list[str]


@dataclass
class Model:
    features: list[str]
    medians: np.ndarray
    scaler: RobustScaler
    pca: PCA
    mean: np.ndarray
    inv_cov: np.ndarray
    err_med: float
    err_mad: float
    mah_med: float
    mah_mad: float
    missing_rate: np.ndarray
    base_indices: list[int]
    fast_indices: list[int]


def _robust_z(x: np.ndarray, med: float, mad: float) -> np.ndarray:
    scale = 1.4826 * mad
    if not np.isfinite(scale) or scale <= 1e-9:
        scale = 1.0
    return np.clip((x - med) / scale, 0.0, 100.0)


def prepare(
    df: pl.DataFrame,
    features: list[str] | None = None,
    regime: str | None = None,
    min_coverage: float = 0.5,
) -> Matrix:
    if regime and regime in df.columns:
        sub = df.filter(pl.col(regime))
        if not sub.is_empty():
            df = sub

    candidates = available_features(df) if features is None else [
        f for f in features if f in df.columns
    ]
    keep: list[str] = []
    for name in candidates:
        if df[name].null_count() / max(df.height, 1) <= (1.0 - min_coverage):
            vals = df[name].to_numpy().astype(float)
            if np.isfinite(vals).sum() >= 4 and np.nanstd(vals) > 1e-9:
                keep.append(name)
    if len(keep) < 2:
        raise ValueError("not enough usable features")

    X = df.select(keep).to_numpy().astype(float)
    ms = df["ms"].to_numpy()
    return Matrix(X=X, ms=ms, features=keep)


def fit(
    df: pl.DataFrame,
    features: list[str] | None = None,
    regime: str | None = None,
    min_coverage: float = 0.5,
) -> Model:
    mat = prepare(df, features, regime, min_coverage)
    X = mat.X
    base_indices = [i for i, f in enumerate(mat.features) if f in BASE_FEATURES]
    fast_indices = [i for i, f in enumerate(mat.features) if f in FAST_FEATURES]
    missing_rate = np.isnan(X).mean(axis=0)
    medians = np.nanmedian(X, axis=0)
    medians = np.where(np.isfinite(medians), medians, 0.0)
    X = np.where(np.isfinite(X), X, medians)

    scaler = RobustScaler()
    Xs = np.clip(scaler.fit_transform(X), -5.0, 5.0)

    try:
        pca = PCA(n_components=0.95, svd_solver="full")
        scores = pca.fit_transform(Xs)
    except Exception:
        n_components = min(Xs.shape[1], max(1, Xs.shape[1] - 1))
        pca = PCA(n_components=n_components, svd_solver="full")
        scores = pca.fit_transform(Xs)
    recon = pca.inverse_transform(scores)
    err = np.sum((Xs - recon) ** 2, axis=1)

    mean = Xs.mean(axis=0)
    cov = np.atleast_2d(np.cov(Xs, rowvar=False))
    ridge = max(1e-6, float(np.trace(cov)) / max(cov.shape[0], 1) * 1e-3)
    cov = cov + np.eye(Xs.shape[1]) * ridge
    inv_cov = np.linalg.pinv(cov)
    diff = Xs - mean
    mah = np.einsum("ij,jk,ik->i", diff, inv_cov, diff)

    err_med = float(np.median(err))
    err_mad = float(np.median(np.abs(err - err_med)))
    mah_med = float(np.median(mah))
    mah_mad = float(np.median(np.abs(mah - mah_med)))

    return Model(
        features=mat.features,
        medians=medians,
        scaler=scaler,
        pca=pca,
        mean=mean,
        inv_cov=inv_cov,
        err_med=err_med,
        err_mad=err_mad,
        mah_med=mah_med,
        mah_mad=mah_mad,
        missing_rate=missing_rate,
        base_indices=base_indices,
        fast_indices=fast_indices,
    )


def score(df: pl.DataFrame, model: Model) -> pl.DataFrame:
    """Score every row of df against a fitted model."""
    if df.is_empty():
        return pl.DataFrame(
            schema={
                "ms": pl.Int64,
                "score": pl.Float64,
                "recon_z": pl.Float64,
                "mahal_z": pl.Float64,
                "top_features": pl.Utf8,
            }
        )

    X = df.select([pl.col(f) for f in model.features]).to_numpy().astype(float)
    nan_mask = np.isnan(X)
    X = np.where(np.isfinite(X), X, model.medians)
    Xs = np.clip(model.scaler.transform(X), -5.0, 5.0)
    scores = model.pca.transform(Xs)
    recon = model.pca.inverse_transform(scores)
    residual = Xs - recon
    err = np.sum(residual**2, axis=1)

    diff = Xs - model.mean
    mah = np.einsum("ij,jk,ik->i", diff, model.inv_cov, diff)

    recon_z = _robust_z(err, model.err_med, model.err_mad)
    mahal_z = _robust_z(mah, model.mah_med, model.mah_mad)
    fast = model.fast_indices
    rare = model.missing_rate[fast] < 0.01 if fast else np.array([], dtype=bool)
    if fast and rare.any():
        miss_count = nan_mask[:, np.array(fast)[rare]].sum(axis=1).astype(float)
    else:
        miss_count = np.zeros(len(X))
    # 1-2 missing fast PIDs is a dropout; a long run at session edges is not.
    miss_z = np.where(miss_count <= 2, miss_count * 10.0, 0.0)
    total = np.maximum(np.maximum(recon_z, mahal_z), miss_z)

    contrib = residual**2
    top_idx = np.argsort(contrib, axis=1)[:, ::-1][:, :3]
    top_names = [
        ";".join(model.features[j] for j in row if contrib[i, j] > 0)
        for i, row in enumerate(top_idx)
    ]

    return pl.DataFrame(
        {
            "ms": df["ms"].to_numpy(),
            "score": total,
            "recon_z": recon_z,
            "mahal_z": mahal_z,
            "top_features": top_names,
        }
    )
