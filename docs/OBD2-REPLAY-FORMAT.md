# Replay — protocolo e arquitetura

## Objetivo

Enviar uma gravação (`raw.log`) para o M5Stick e renderizar as 6 páginas como
se o carro estivesse conectado, sem BLE/OBD2.

## Fluxo

```text
raw.log → cmd/obd-replay → serial 115200 → M5Stick REPLAY → cache → UI
```

## Comandos host → Stick

| Linha | Efeito |
|---|---|
| `A` | ACK do host |
| `C,REPLAY` | entra em replay e pausa BLE |
| `C,LIVE` | sai do replay e retoma BLE |
| `C,CLEAR` | limpa o cache |
| `C,STOP` | emite `X,<ms>,host` e pausa o BLE |
| `R,<registro>` | injeta um registro compacto |

## Handshake

Ao abrir a serial o host pulsa o reset da placa e repete `C,REPLAY` até receber
a confirmação:

| Linha Stick → host | Efeito |
|---|---|
| `K,REPLAY` | replay ativo |
| `K,LIVE` | live ativo |

Isso garante que o comando não se perca durante o boot e que o BLE não seja
inicializado no modo replay.

### Exemplos

```text
C,REPLAY
R,I,0,0.1,9BGEA48A0LG185881,440
R,P,241,0C,0000AAAAAA,241,OK
R,P,600,0D,1E,140,OK
R,P,1200,05,73,135,OK
R,E,2000,NOTE,replay
R,X,2500,end
C,LIVE
```

O `R,` carrega o mesmo formato compacto que o firmware já emite:

```text
I,<ms>,<fw>,<vin>,<tank_dl>
P,<ms>,<pid>,<raw>,<lat>,<q>
E,<ms>,<type>,<detail>
X,<ms>,<reason>
```

## Modos

```text
LIVE  → BLE ativo, scheduler de PIDs, UI + logger
REPLAY → BLE pausado, comandos R,... atualizam o cache, UI ativa
```

## UI

- modo replay indicado por `R`/`REPLAY` no rodapé;
- mesma página e mesmo cache do modo live;
- `SERIAL=OK` enquanto o host envia ACK;
- `BLE=OFF` durante replay;
- `ELM=WAIT` durante replay.

## CLI

```bash
go run ./cmd/obd-replay \
  --port /dev/cu.usbserial-XXXX \
  --file data/<sessão>/raw.log \
  --speed 1
```

Flags:

| Flag | Descrição |
|---|---|
| `--port` | serial do Stick |
| `--file` | `raw.log` ou `decoded.ndjson` |
| `--speed` | 1 = tempo real; 2 = dobro; 0 = sem espera |
| `--loop` | repete a gravação |
| `--start` / `--end` | intervalo em ms |
| `--dry-run` | não abre serial |
| `--verbose` | mostra cada linha |
| `--reset` | pulsa o reset antes do replay (padrão: ligado) |
| `--echo` | mostra a saída da serial do Stick |

## Arquitetura

```text
cmd/obd-replay
  ├─ Source (raw.log / NDJSON)
  ├─ ReplayEngine (timing, ACK, loop)
  └─ SerialSink (C/R/A)

Firmware
  ├─ SerialLink (linhas A/C/R)
  ├─ ReplayController (I/P/E/X → Cache)
  ├─ Cache/Snapshot
  └─ UI
```

## Critérios de aceite

- uma gravação atualiza as 6 páginas;
- `C,LIVE` retorna ao scan BLE;
- linha inválida não trava o firmware;
- `SERIAL` fica OK com ACK;
- sem OOM no Stick.
