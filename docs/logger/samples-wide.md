# `samples_wide_1hz.csv`

Reamostragem para 1 Hz: uma linha por segundo, com o último valor de cada PID e
a idade desse valor.

## Cabeçalho

```text
ms,ts_host,rpm,rpm_age_ms,speed,speed_age_ms,coolant,coolant_age_ms,...
```

## Exemplo

```text
1000,2026-11-02T18:30:01-03:00,1726,0,0,880,75,750,...
```

## PIDs incluídos

```text
rpm, speed, load, coolant, intake, throttle, map, maf,
stft, ltft, lambda, fuel_pct, catalyst, voltage, ethanol,
ambient, relative_throttle, pedal_d, pedal_e,
commanded_throttle, baro
```

## `age_ms`

- `0` = valor lido no segundo atual;
- `500` = valor tem 500 ms;
- célula vazia = PID ainda não lido.

Isso evita fingir que todos os PIDs têm 1 Hz.

## Uso

- features para ML;
- gráficos;
- detecção de gaps;
- comparação entre viagens.

## Observações

- O wide é derivado; não é fonte de verdade.
- Para auditoria, use `samples_long.csv` ou `raw.log`.
- Se um PID mudar muito rápido, o wide perde resolução; consulte o long.
