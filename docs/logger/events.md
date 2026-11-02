# `events.jsonl`

Eventos e erros recebidos do firmware.

## Formato

```json
{"t":"E","ms":5000,"type":"NO_DATA","msg":"0C"}
```

## Uso

- saber quando um PID falhou;
- separar gaps de dados;
- registrar DTCs e readiness;
- correlacionar com amostras.

## Tipos atuais

| Tipo | Detalhe | Significado |
|---|---|---|
| `NO_DATA` | PID | PID sem dados |
| `PARSE_ERROR` | PID | resposta inválida |
| `RECONNECT` | motivo | link BLE caiu e o firmware vai reconectar |
| `MIL` | `ON`/`OFF` | luz de anomalia (`0101` bit 7) |
| `DTC_COUNT` | número | DTCs confirmados (`0101` bits 6–0) |
| `READINESS` | hex | bytes B/C/D de `0101` (ex.: `07E100`) |
| `DTC` | código | DTC armazenado (Mode `03`) |
| `DTC_PENDING` | código | DTC pendente (Mode `07`) |
| `DTC_PERMANENT` | código | DTC permanente (Mode `0A`) |
| `note` | texto | evento manual/replay |

O `cmd/obd-logger` também resume `MIL`, `DTC_COUNT`, `READINESS` e os códigos
em `metadata.json` (campos `mil`, `dtc_count`, `readiness`, `dtcs`,
`pending_dtcs`, `permanent_dtcs`).

## Observações

- O evento não substitui a amostra; ele explica ausências.
- `ms` é o tempo do dispositivo.
