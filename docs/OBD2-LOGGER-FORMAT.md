# Formato do logger OBD2 (NDJSON → CSV)

> O M5Stick não armazena dados. Ele transmite linhas NDJSON pela serial USB e o
> host (`cmd/obd-logger`) grava os arquivos. O firmware que emite esse formato
> é `poc/obd2/obd-logger`.

## Fluxo

```text
M5StickC Plus2
  └─ BLE → ELM327 → carro
       └─ USB serial (115200) → cmd/obd-logger
            ├─ data/<sessão>/raw.ndjson
            ├─ data/<sessão>/samples_long.csv
            ├─ data/<sessão>/samples_wide_1hz.csv
            ├─ data/<sessão>/events.jsonl
            └─ data/<sessão>/metadata.json
```

## NDJSON

Cada linha é um objeto JSON independente.

### S — início da sessão

```json
{"t":"S","v":1,"sid":"2026-11-02T18-30-00","fw":"0.1","vin":"9BG...","tank_dl":440}
```

| Campo | Obrigatório | Descrição |
|---|---|---|
| `t` | sim | `S` |
| `v` | sim | versão do formato |
| `sid` | não | ID da sessão; o host gera se faltar |
| `fw` | não | versão do firmware |
| `vin` | não | VIN do veículo |
| `tank_dl` | não | capacidade do tanque em decilitros (440 = 44,0 L) |

### P — leitura de PID

```json
{"t":"P","seq":1,"ms":1234,"pid":"0C","raw":"1AF8","v":1726,"u":"rpm","q":"OK","lat":138}
```

| Campo | Obrigatório | Descrição |
|---|---|---|
| `t` | sim | `P` |
| `seq` | não | sequência crescente |
| `ms` | sim | milissegundos desde o início da sessão |
| `pid` | sim | PID sem o modo, ex.: `0C` |
| `raw` | não | bytes de dados em hex |
| `v` | sim | valor já convertido |
| `u` | sim | unidade |
| `q` | sim | qualidade |
| `lat` | não | latência da consulta em ms |

### E — evento

```json
{"t":"E","ms":5000,"type":"dtc","code":"P0133","msg":"DTC armazenado"}
```

Campos `type`, `code` e `msg` são livres; o host grava a linha em
`events.jsonl`.

### X — fim da sessão

```json
{"t":"X","ms":600000,"reason":"user"}
```

## Qualidade

| Valor | Significado |
|---|---|
| `OK` | leitura válida |
| `TIMEOUT` | o adaptador não respondeu |
| `STALE` | valor reaproveitado; a idade está no wide |
| `ENGINE_OFF` | motor desligado |
| `OUT_OF_RANGE` | valor fora da faixa esperada |
| `NO_DATA` | PID sem dados |

## CSV long

`samples_long.csv`:

```text
ts_host,session_id,seq,ms,pid,raw,value,unit,quality,latency_ms
```

Uma linha por leitura de PID. É a fonte de verdade.

## CSV wide 1 Hz

`samples_wide_1hz.csv` é gerado pelo host. Para cada segundo, usa o último valor
de cada PID e grava a idade do dado:

```text
ms,ts_host,rpm,rpm_age_ms,speed,speed_age_ms,coolant,coolant_age_ms,...
```

PIDs incluídos no wide:

```text
rpm, speed, load, coolant, intake, throttle, map, maf,
stft, ltft, lambda, fuel_pct, catalyst, voltage, ethanol,
ambient, relative_throttle, pedal_d, pedal_e,
commanded_throttle, baro
```

Se um PID ainda não foi lido, a célula fica vazia. `age_ms` mostra há quanto
tempo o valor foi atualizado.

## Metadados

`metadata.json`:

```json
{
  "session_id": "2026-11-02T18-30-00",
  "started_at": "...",
  "ended_at": "...",
  "port": "/dev/cu.usbserial-...",
  "baud": 115200,
  "replay": false,
  "firmware": "0.1",
  "vin": "9BG...",
  "tank_dl": 440,
  "samples": 1234,
  "events": 2,
  "invalid_lines": 0
}
```

## Taxa e volume

- Uma leitura de PID leva ~130–170 ms.
- O logger deve alternar PIDs rápidos e lentos.
- O host reamostra para 1 Hz sem fingir que todos os PIDs têm 1 Hz.
- 1 h de log longo fica na ordem de poucos MB.

## Replay

O loader aceita um arquivo NDJSON já existente:

```bash
go run ./cmd/obd-logger --replay logs/exemplo.ndjson --out data/
```

Isso permite testar o parser, CSV e wide sem carro nem Stick.

## Privacidade

O VIN pode ser incluído no `metadata.json` e no `S`. Se o log for compartilhado,
remova ou hasheie o VIN antes.
