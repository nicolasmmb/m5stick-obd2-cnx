# `metadata.json`

Resumo da sessão.

## Exemplo

```json
{
  "session_id": "20261102-183000",
  "started_at": "2026-11-02T18:30:00-03:00",
  "ended_at": "2026-11-02T18:40:00-03:00",
  "port": "/dev/cu.usbserial-XXXX",
  "baud": 115200,
  "replay": false,
  "firmware": "0.1",
  "vin": "9BG...",
  "tank_dl": 440,
  "samples": 1234,
  "events": 2,
  "invalid_lines": 0,
  "duration_ms": 600000,
  "end_reason": "host",
  "mil": false,
  "dtc_count": 0,
  "readiness": "07E100",
  "dtcs": ["P0133"],
  "pending_dtcs": ["P0300"],
  "permanent_dtcs": ["P0420"]
}
```

## Campos

| Campo | Significado |
|---|---|
| `session_id` | ID/diretório da sessão |
| `source_session_id` | `sid` enviado pelo firmware, se houver |
| `started_at` / `ended_at` | horário do host |
| `port` / `baud` | configuração serial |
| `replay` | `true` se veio de arquivo |
| `firmware` | versão do firmware |
| `vin` | VIN, se recebido |
| `tank_dl` | capacidade do tanque em dL |
| `samples` | PIDs decodificados |
| `events` | eventos |
| `invalid_lines` | linhas rejeitadas |
| `duration_ms` | duração pelo tempo do dispositivo |
| `end_reason` | último `X`: `host`, `ble_lost`, `replay` ou `error` |
| `mil` | MIL ligada/desligada (`0101`) |
| `dtc_count` | DTCs confirmados (`0101`) |
| `readiness` | bytes B/C/D de `0101` em hex |
| `dtcs` | códigos armazenados (Mode `03`), sem repetição |
| `pending_dtcs` | códigos pendentes (Mode `07`) |
| `permanent_dtcs` | códigos permanentes (Mode `0A`) |

## Uso

- selecionar sessões;
- comparar logs;
- saber se o VIN está presente;
- validar se a coleta terminou.
