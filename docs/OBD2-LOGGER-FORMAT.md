# Formato do logger OBD2

> O M5Stick **não formata JSON nem armazena dados**. Ele envia linhas compactas
> pela serial. Toda a decodificação, unidades, JSON, CSV e resample ficam no
> host (`cmd/obd-logger`).

## Fluxo

```text
M5Stick (poc/obd2/obd-logger)
  └─ BLE → ELM327 → carro
       └─ USB serial 115200
            └─ cmd/obd-logger
                 ├─ raw.log
                 ├─ decoded.ndjson
                 ├─ samples_long.csv
                 ├─ samples_wide_1hz.csv
                 ├─ events.jsonl
                 ├─ metadata.json
                 └─ invalid.log
```

## Protocolo compacto

### I — início

```text
I,<ms>,<fw>,<vin>,<tank_dl>
```

```text
I,0,0.1,9BGEA48A0LG185881,440
```

### P — leitura de PID

```text
P,<ms>,<pid>,<raw_hex>,<lat_ms>,<qualidade>
```

```text
P,1234,0C,1AF8,138,OK
```

O Stick envia **raw_hex**, não valor nem unidade. O host aplica a fórmula.

### E — evento/erro

```text
E,<ms>,<tipo>,<detalhe>
```

```text
E,5000,NO_DATA,0C
```

Eventos de diagnóstico emitidos a cada conexão/reconexão:

```text
E,120,MIL,OFF
E,121,DTC_COUNT,0
E,122,READINESS,07E100
E,123,DTC,P0133
E,124,DTC_PENDING,P0300
E,125,DTC_PERMANENT,P0420
```

- `READINESS` traz os bytes B/C/D de `0101` em hex (6 caracteres).
- `DTC`/`DTC_PENDING`/`DTC_PERMANENT` vêm dos modos `03`/`07`/`0A`; o
  primeiro byte da resposta é a contagem, não um código.
- `RECONNECT` aparece quando o link BLE cai e o firmware volta a escanear.

### X — fim

```text
X,<ms>,<motivo>
```

```text
X,600000,host
```

| Motivo | Quando |
|---|---|
| `ble_lost` | escrita BLE falhou ou 3 timeouts consecutivos |
| `replay` | host pediu `C,REPLAY` |
| `host` | host pediu `C,STOP` (Ctrl-C do loader) |
| `error` | outra falha depois do início da sessão |

Um `X` é emitido antes de cada reconexão e antes de trocar de modo. O `ms` é
monotônico desde o boot: reconectar **não** reinicia a linha do tempo.

### Comandos host → Stick

| Linha | Efeito |
|---|---|
| `A` | ACK de presença do host |
| `C,REPLAY` | entra em replay e pausa o BLE |
| `C,LIVE` | sai do replay/pausa e retoma o BLE |
| `C,CLEAR` | limpa o cache |
| `C,STOP` | emite `X,host` e pausa o BLE |
| `C,SIM` | entra no simulador serial (`Q`/`S`) |
| `C,MEM` | responde `M,mem,alloc=...,free=...` |
| `R,<registro>` | injeta um registro no modo replay |

## Qualidade

| Valor | Significado |
|---|---|
| `OK` | leitura válida |
| `NO_DATA` | PID sem dados |
| `PARSE_ERROR` | resposta inválida |
| `NO_DECODER` | host não tem fórmula para o PID |
| `TIMEOUT` | adaptador não respondeu |
| `STALE` | valor reaproveitado no wide |

## Arquivos gerados

| Arquivo | Conteúdo |
|---|---|
| `raw.log` | linhas compactas exatamente como chegaram |
| `decoded.ndjson` | JSON canônico gerado pelo host |
| `samples_long.csv` | uma linha por PID decodificado |
| `samples_wide_1hz.csv` | reamostragem 1 Hz com `age_ms` |
| `events.jsonl` | eventos `E` |
| `metadata.json` | sessão, VIN, firmware, contagens |
| `invalid.log` | linhas que o parser rejeitou |

Documentação por arquivo: **[docs/logger/](logger/)**.

## NDJSON legado

O loader ainda aceita linhas no formato antigo:

```json
{"t":"P","ms":1234,"pid":"0C","raw":"1AF8","v":1726,"u":"rpm","q":"OK","lat":138}
```

Isso mantém compatibilidade com `logs/` e replays antigos.

## Taxa e volume

- Uma consulta de PID leva ~130–170 ms.
- O firmware alterna PIDs rápidos e lentos.
- O host reamostra para 1 Hz; `age_ms` mostra a idade de cada valor.
- 1 h de log fica na ordem de poucos MB.

## Replay

```bash
go run ./cmd/obd-logger --replay=testdata/obd-logger-sample-compact.txt --out=data/
```

Também aceita o sample NDJSON legado.
