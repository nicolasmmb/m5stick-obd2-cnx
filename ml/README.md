# OBD-II ML — anomalias não supervisionadas

Ferramenta **host-only**: o M5Stick só envia `I/P/E/X`; todo o cálculo
(dataset, matrix profile, PCA/Mahalanobis, relatórios) roda no PC.

## Setup

```bash
cd ml
uv sync
```

Python 3.12 é baixado/gerenciado pelo `uv`.

## Uso

```bash
# sessões salvas em data/
uv run obdml --source data --data ../data --out reports

# com benchmark de falhas sintéticas
uv run obdml --source data --data ../data --out reports --synthetic

# valores injetados (CSV longo ou wide)
uv run obdml --source inject --file injected.csv --train-data ../data --out reports

# serial ao vivo (60 s)
uv run obdml --source serial --port /dev/cu.usbserial-XXXX --seconds 60

# serial contínuo até Ctrl-C
uv run obdml --source serial --port /dev/cu.usbserial-XXXX --follow
```

## Saída

```text
reports/
  <sessão>/
    summary.md
    anomalies.csv
    matrix_profile.csv
    plots/score.png
    plots/signal_maf.png
  index.md
  synthetic_eval.md
```

## Pipeline

1. Lê `samples_long.csv`, `metadata.json` e `events.jsonl`.
2. Monta grade de 1 Hz com tolerância por PID (rápidos 10 s, lentos 30 s).
3. Detecta regimes: `engine_off`, `idle`, `moving_cold`, `moving_warm`.
4. Adiciona features derivadas: aceleração, jerk, razões, derivadas e
   rolling std (sensor travado).
5. Treina **PCA + Mahalanobis** no regime normal.
6. Calcula **matrix profile** (motivos/discords) por sinal.
7. Escreve relatório Markdown + PNG.

## Testes

```bash
uv run pytest
```

## Limitações

- 4 sessões, um carro; os scores são exploratórios.
- `2F` é ruidoso; não usar como ground truth.
- Cold start é reportado separado para não ser confundido com falha.
- Sem labels: validação por falhas sintéticas, estabilidade e inspeção.
