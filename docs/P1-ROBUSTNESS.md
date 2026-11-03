# P1 — robustez do modo live

> Status: **implementado; validação de hardware em backlog**.
> P0 concluído com o carro.
> Escopo: reconexão BLE, encerramento explícito (`X`) e diagnóstico
> (DTCs/readiness) no logger.

## 1. Reconexão automática

### Detecção

O polling distingue três resultados:

| Resultado | Quando | Ação |
|---|---|---|
| `pidOK` | resposta válida | zera o contador de falhas |
| `pidRetry` | `NO DATA`/`PARSE_ERROR` | zera o contador: o adaptador respondeu |
| `pidTimeout` | sem resposta dentro de 2 s | incrementa o contador |
| `pidLinkLost` | `WriteWithoutResponse` falhou | encerra a sessão imediatamente |

Três **timeouts** consecutivos também encerram a sessão: um adaptador fora de
alcance normalmente para de responder antes de a escrita falhar. `NO DATA` é
uma resposta válida (PID não suportado), então **não** derruba o link.

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

- `go test ./internal/... ./cmd/... ./poc/obd2/obd-logger` ✅
- `go vet` nos mesmos pacotes ✅
- `tinygo build -target=m5stick-c -size short` ✅
  - antes: flash 302.356 / RAM 59.028
  - depois (P1): flash 307.844 / RAM 59.084
  - com o simulador serial: flash 321.488 / RAM 62.924
- `gopls check` no firmware ✅

A lógica de sessão agora roda no host contra um `Transport` fake
(`session_test.go`), com respostas reais dos logs:

- VIN multi-frame;
- `0101`/`03`/`07`/`0A` → MIL, contagem, readiness e códigos;
- 3 timeouts → `errLinkLost` + `X,ble_lost`;
- erro de escrita → link perdido imediato;
- `C,STOP`/modo → `X,host`.

Validação no Stick sem BLE: ver [SERIAL-SIM.md](SERIAL-SIM.md).

Validado em hardware:

- a UI continua atualizando durante o `Connect` do BLE (sem pausa);
- os dados continuam sendo gravados em `data/`.

Backlog de hardware (P1):

- derrubar o adaptador e confirmar reconexão + beeps;
- Ctrl-C e confirmar `BLE=OFF`;
- carro com DTC/readiness para validar os códigos reais;
- `seq`/`session_id` no protocolo.

## 5. Arquivos

```text
internal/obdline/line.go            # C,STOP / C,SIM
internal/obddtc/                    # DTC + readiness
internal/obdlog/loader.go           # resumo em metadata.json
cmd/obd-logger/main.go              # C,STOP no shutdown
cmd/obd-replay/sim.go               # simulador Q/S
poc/obd2/obd-logger/session.go      # loop live sem BLE
poc/obd2/obd-logger/transport.go    # interface Transport
poc/obd2/obd-logger/transport_ble.go# BLE
poc/obd2/obd-logger/transport_sim.go# simulador serial
poc/obd2/obd-logger/sink_tinygo.go  # saída I/P/E/X + UI
poc/obd2/obd-logger/main.go         # backoff e seleção de transporte
poc/obd2/obd-logger/buzzer.go       # beeps do backoff
poc/obd2/obd-logger/serial_link.go  # C,SIM / S,<resposta>
poc/obd2/obd-logger/session_test.go # testes no host
```
