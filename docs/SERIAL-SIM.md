# Simulador serial — validar o modo live sem BLE

> Objetivo: rodar o **mesmo loop live** (`Session`) sem carro e sem adaptador
> BLE, usando a serial e um `raw.log` como resposta.

## Por que existe

O replay normal (`C,REPLAY` + `R,P,...`) injeta valores finais no cache. Ele
não passa por `Session`, então não valida:

- handshake ELM;
- polling de PIDs;
- detecção de link perdido (3 timeouts);
- reconexão, backoff e beeps;
- `X`/`C,STOP`;
- leitura de DTCs/readiness.

O modo `C,SIM` resolve isso: o firmware pergunta (`Q`), o host responde (`S`),
e o loop live roda de verdade.

## Arquitetura

```text
Firmware
  Session.Run()
    └─ Transport
         ├─ BLE    (carro)
         └─ sim    (Q/S na serial)

Host
  cmd/obd-replay --sim
    ├─ loadSnapshot(raw.log)  -> último raw de cada PID + VIN
    ├─ responde Q,<cmd> com S,<resposta ELM>
    └─ --drop-after N          -> para de responder por --drop-for
```

O mesmo binário do firmware atende carro e simulador. O BLE só é inicializado
no modo live normal.

## Protocolo

| Direção | Linha | Efeito |
|---|---|---|
| host → Stick | `C,SIM` | entra no modo simulado |
| host → Stick | `C,STOP` | encerra e emite `X,host` |
| host → Stick | `A` | ACK de presença |
| Stick → host | `Q,<cmd>` | comando ELM (`ATZ`, `010C`, `0902`...) |
| host → Stick | `S,<resposta>` | resposta ELM (`410C1AF8AA||>`) |
| Stick → host | `K,SIM` | confirmação do modo |
| Stick → host | `I/P/E/X` | mesmo protocolo do logger |

Exemplo:

```text
host: C,SIM
Stick: K,SIM
Stick: Q,ATZ
host:  S,ELM327 v2.1
Stick: Q,0902
host:  S,490201394247...||>
Stick: I,120,0.1,9BG...,440
Stick: Q,0101
host:  S,41010007E100AA||>
Stick: E,125,MIL,OFF
Stick: E,126,DTC_COUNT,0
Stick: E,127,READINESS,07E100
Stick: Q,010C
host:  S,410C1AF8AA||>
Stick: P,130,0C,1AF8,0,OK
```

## Uso

```bash
go run ./cmd/obd-replay --sim \
  --port /dev/cu.usbserial-XXXX \
  --file data/20261102-162543/raw.log
```

Pelo Makefile:

```bash
make sim FILE=data/20261102-162543/raw.log
make sim FILE=data/... DROP_AFTER=40
```

## Flags

| Flag | Descrição |
|---|---|
| `--sim` | ativa o modo simulado |
| `--port` | serial do Stick |
| `--file` | `raw.log`/NDJSON de onde saem VIN e PIDs |
| `--drop-after N` | para de responder depois de N comandos |
| `--drop-for` | tempo sem responder (padrão 10 s) |
| `--mil` | MIL ligada na resposta de `0101` |
| `--dtcs` | DTCs armazenados, ex.: `P0133,U0123` |
| `--readiness` | bytes B/C/D de `0101` (padrão `07E100`) |
| `--echo` | mostra a serial crua |
| `--verbose` | mostra cada `Q -> S` |
| `--reset` | pulso de reset antes de `C,SIM` (padrão ligado) |

`--speed`, `--loop`, `--start` e `--end` não se aplicam ao `--sim`: o
simulador usa um retrato estático do arquivo (último valor de cada PID).

## Testar reconexão

```bash
go run ./cmd/obd-replay --sim \
  --port /dev/cu.usbserial-XXXX \
  --file data/.../raw.log \
  --drop-after 40 \
  --drop-for 10s
```

Sequência esperada:

```text
Q,010C
Q,010C
Q,010C
X,123456,ble_lost     <- 3 timeouts
beep ...              <- backoff 1 s (1 beep)
Q,ATZ
host volta a responder
I,124000,0.1,...      <- nova sessão, ms monotônico
```

## Testar DTC/readiness

```bash
go run ./cmd/obd-replay --sim \
  --port /dev/cu.usbserial-XXXX \
  --file data/.../raw.log \
  --mil \
  --dtcs P0133,U0123 \
  --readiness 07E100
```

Esperado em `events.jsonl`/serial:

```text
E,125,MIL,ON
E,126,DTC_COUNT,2
E,127,READINESS,07E100
E,128,DTC,P0133
E,129,DTC,U0123
```

## Testar shutdown

`Ctrl-C` no simulador envia `C,STOP`. O firmware emite `X,<ms>,host` e pausa o
BLE.

## Testes automáticos

```bash
go test ./poc/obd2/obd-logger ./cmd/obd-replay ./internal/...
```

- `session_test.go` usa um `Transport` fake com respostas reais dos logs;
- valida VIN multi-frame, DTC/readiness, 3 timeouts → `X,ble_lost`, escrita
  com erro → link perdido e `C,STOP` → `X,host`;
- `sim_test.go` valida a montagem das respostas ELM do simulador.

## Limitações

- o snapshot é estático: as 6 páginas mostram o último valor do arquivo;
- o simulador não reproduz latência real do ELM (responde na hora);
- o transporte BLE em si continua precisando de hardware para validação final;
- `--drop-for` deve ser maior que 3 timeouts (6–9 s) para garantir a queda.
