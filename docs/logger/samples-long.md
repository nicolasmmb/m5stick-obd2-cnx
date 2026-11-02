# `samples_long.csv`

Uma linha por leitura de PID, já decodificada.

## Cabeçalho

```text
ts_host,session_id,seq,ms,pid,raw,value,unit,quality,latency_ms
```

## Exemplo

```text
2026-11-02T18:30:01.234-03:00,20261102-183000,1,1234,0C,1AF8,1726,rpm,OK,138
```

## Colunas

| Coluna | Significado |
|---|---|
| `ts_host` | horário em que o host recebeu a linha |
| `session_id` | ID da sessão |
| `seq` | sequência do firmware, quando existir |
| `ms` | tempo do dispositivo |
| `pid` | PID sem o modo |
| `raw` | bytes de dados |
| `value` | valor decodificado |
| `unit` | unidade |
| `quality` | `OK`, `NO_DATA`, `NO_DECODER` etc. |
| `latency_ms` | latência da consulta |

## Uso

- fonte de verdade para análises;
- entrada para Pandas/Go/Julia;
- base para gerar o wide;
- auditoria de qualidade.

## Observações

- PIDs diferentes têm frequências diferentes; não assuma 1 Hz.
- `ms` é relativo ao início da sessão.
- Para séries temporais, use `ms` ou `ts_host`, não a ordem das linhas.
