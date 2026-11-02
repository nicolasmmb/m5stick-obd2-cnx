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
  "duration_ms": 600000
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

## Uso

- selecionar sessões;
- comparar logs;
- saber se o VIN está presente;
- validar se a coleta terminou.
