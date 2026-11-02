# P1 — robustez do modo live

> Implementado sobre a `main` (P0 validado, exceto a sessão longa no carro).
> Escopo: reconexão BLE, encerramento explícito (`X`) e diagnóstico
> (DTCs/readiness) no logger.

## 1. Reconexão automática

### Detecção

O polling distingue três resultados:

| Resultado | Quando | Ação |
|---|---|---|
| `pidOK` | resposta válida | zera o contador de falhas |
| `pidRetry` | `NO DATA`, `PARSE_ERROR` ou timeout | incrementa o contador |
| `pidLinkLost` | `WriteWithoutResponse` falhou | encerra a sessão imediatamente |

Três falhas consecutivas (`pidRetry`) também encerram a sessão: um adaptador
fora de alcance normalmente para de responder antes de a escrita falhar.

### Backoff

| Tentativa | Espera |
|---|---|
| 1 | 1 s |
| 2 | 2 s |
| 3 | 5 s |
| 4+ | 10 s |

O contador é zerado quando o handshake ELM termina com sucesso. Durante a
espera o buzzer emite **um beep de 120 ms por segundo** (1, 2, 5 ou 10 beeps),
a 2,7 kHz, com duty máximo (`Top()/2`, volume fixo do hardware).

### Reconexão

Depois do backoff o firmware refaz o ciclo completo:

```text
scan → connect → discover → EnableNotifications
     → ATZ/ATE0/ATL0/ATS0/ATSP0
     → VIN → I → diagnóstico → polling
```

O relógio `ms` é monotônico desde o boot (`liveMs()`), então o `X`/`I` de uma
reconexão não reiniciam a linha do tempo do host. A sessão do host continua e o
CSV wide não perde o passo.

## 2. `X` e `C,STOP`

O firmware emite `X` sempre que uma sessão live que já emitiu `I` termina:

| Motivo | Quando |
|---|---|
| `ble_lost` | escrita falhou ou 3 timeouts consecutivos |
| `replay` | host pediu `C,REPLAY` |
| `host` | host pediu `C,STOP` |
| `error` | outra falha depois do início da sessão |

`C,STOP` foi adicionado ao protocolo:

| Linha | Efeito |
|---|---|
| `C,STOP` | emite `X,<ms>,host`, para o scan e pausa o BLE |
| `C,LIVE` | retoma o modo live a partir da pausa |

No Ctrl-C o `cmd/obd-logger` envia `C,STOP`, drena a serial por até 700 ms e
grava o `X` em `raw.log`/`decoded.ndjson`/`metadata.json` antes de fechar.
Firmware antigo ignora a linha.

## 3. DTCs e readiness

O firmware lê, a cada conexão/reconexão:

- `0101`: MIL, contagem confirmada e bytes de readiness;
- `03`: DTCs armazenados;
- `07`: DTCs pendentes;
- `0A`: DTCs permanentes.

Eventos emitidos:

```text
E,<ms>,MIL,<ON|OFF>
E,<ms>,DTC_COUNT,<n>
E,<ms>,READINESS,<BC D em hex>      # ex.: 07E100
E,<ms>,DTC,<P0133>
E,<ms>,DTC_PENDING,<P0300>
E,<ms>,DTC_PERMANENT,<P0420>
```

O host já grava todo `E` em `events.jsonl` e agora resume os campos em
`metadata.json`:

```json
{
  "mil": true,
  "dtc_count": 2,
  "readiness": "07E100",
  "dtcs": ["P0133", "U0123"],
  "pending_dtcs": ["P0300"],
  "permanent_dtcs": ["P0420"]
}
```

A resposta `43 00 AA AA AA AA AA` (nenhum DTC) é tratada corretamente: o
primeiro byte é a contagem, não um código. Respostas multi-frame
(`0:43...|1:...`) têm os prefixos removidos antes da decodificação.

## 4. Validação

- `go test ./internal/... ./cmd/...` ✅
- `go vet ./internal/... ./cmd/...` ✅
- `tinygo build -target=m5stick-c -size short` ✅
  - antes: flash 302.356 / RAM 59.028
  - depois: flash 307.844 / RAM 59.084
- `gopls check` no firmware ✅

Pendente de hardware:

- derrubar o adaptador e confirmar `X,ble_lost` + reconexão + beeps;
- Ctrl-C e confirmar `X,host` e `BLE=OFF`;
- carro com DTC/readiness para validar os códigos reais.

## 5. Arquivos

```text
internal/obdline/line.go          # C,STOP
internal/obddtc/                  # DTC + readiness (novo pacote)
internal/obdlog/loader.go         # resumo em metadata.json
cmd/obd-logger/main.go            # C,STOP no shutdown
poc/obd2/obd-logger/live.go       # detecção, reconexão, diagnóstico
poc/obd2/obd-logger/main.go       # backoff e relógio monotônico
poc/obd2/obd-logger/buzzer.go     # beeps do backoff (novo)
poc/obd2/obd-logger/serial_link.go# ModePaused
```
