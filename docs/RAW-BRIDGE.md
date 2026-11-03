# Raw bridge — firmware minimalista + logger v2

> Branch: `feat/raw-bridge`.
> Objetivo: testar BLE/ELM e gravação de dados de forma independente, com o
> M5 apenas repassando bytes crus entre o ELM327 e o PC.

## 1. Fluxo sequencial

```text
PC                         M5
│ HELLO ─────────────────► │
│ ◄──────────────── ACK    │
│ HB ────────────────────► │
│ ◄──────────────── HB_ACK│
│                          ├─ BLE scan
│ ◄──────────────── STATE BLE=SCAN
│                          ├─ conecta no ELM
│ ◄──────────────── STATE BLE=ON
│ C,ATZ ─────────────────► │
│ ◄──────────────── Q/R    │
│ C,START ───────────────► │
│ ◄──────────────── START_ACK
│ ◄──────────────── R,<hex>│
```

Ordem rígida:

1. serial;
2. BLE;
3. ELM;
4. `START`;
5. gravação append-only.

O M5 não interpreta `OK`, `NO DATA`, `>` nem PIDs.

## 2. Protocolo

### PC → M5

| Linha | Efeito |
|---|---|
| `HELLO,1,<session>,<seq>` | inicia conexão |
| `HB,<seq>` | heartbeat |
| `C,<cmd>` | envia comando ao ELM |
| `START,<seq>` | começa a gravar |
| `STOP,<seq>` | para de gravar |
| `BYE,<seq>` | encerra |
| `C,MEM` | responde `M,<ms>,alloc=...,free=...,sys=...` |

### M5 → PC

| Linha | Efeito |
|---|---|
| `HELLO_ACK,1,<seq>` | confirma serial |
| `HB_ACK,<seq>,<ms>` | confirma heartbeat |
| `S,<ms>,SER,<0|1>` | serial conectado |
| `S,<ms>,BLE,<OFF|SCAN|ON>` | BLE |
| `S,<ms>,ELM,<WAIT|RX|TIMEOUT>` | atividade do ELM |
| `Q,<ms>,<cmd>` | comando enviado |
| `R,<ms>,<hex>` | bytes crus |
| `START_ACK,<seq>` | gravação iniciada |
| `STOP_ACK,<seq>` | gravação parada |
| `ERR,<ms>,<code>,<detail>` | erro |
| `X,<ms>,<reason>` | fim/reconexão |

`R` é hex porque a resposta do ELM contém `\r`, `>` e lixo.

O PC acumula `R` até ver `3E` (`>`).

## 3. Firmware

```bash
tinygo flash -target=m5stick-c -size short \
  -port=/dev/cu.usbserial-XXXX \
  ./poc/obd2/obd-raw-bridge
```

UI:

```text
┌──────────────────┬──────────────────┐
│ SERIAL           │ BLE              │
│ WAIT/OK/DISC     │ OFF/SCAN/ON      │
├──────────────────┼──────────────────┤
│ ELM              │ TX               │
│ WAIT/RX/TIMEOUT  │ contador         │
└──────────────────┴──────────────────┘
```

- luz fictícia no topo/esquerda;
- cards com relevo de 1 px;
- alerta em cor saturada;
- heartbeat verde quando `recording`, azul caso contrário;
- sem páginas, botões ou valores de PID.

## 4. Logger v2

```bash
go run ./cmd/obd-logger-v2 \
  --port /dev/cu.usbserial-XXXX \
  --out data \
  --pids 010C,010D,0111
```

### Flags

| Flag | Função |
|---|---|
| `--pids` | lista (`010C,010D`) ou `all` |
| `--multi` | PIDs por comando (1 = individual) |
| `--interval` | espera entre comandos |
| `--timeout` | timeout por comando |
| `--session` | reaproveita/append no diretório |
| `--reset` | pulso de reset antes do handshake (padrão ligado) |
| `--ble-timeout` | tempo máximo esperando `BLE=ON` (padrão 20 s) |
| `--mem` | pede `C,MEM` no início/fim |
| `--mem-every` | pede `C,MEM` periodicamente |
| `--echo` | mostra serial cru |
| `--verbose` | loga cada linha |

### `--pids all`

O logger pede:

```text
0100 → bitmask 01..20
0120 → bitmask 21..40
0140 → bitmask 41..60
...
```

e monta a lista dos PIDs suportados. Isso não inclui:

- PIDs proprietários;
- CAN bruto;
- DTC/VIN (precisam ser pedidos à parte).

### Saída

```text
data/<sessão>/
  raw_elm.log       # append-only, Q/R/S/E/X com ts_host
  commands.csv      # append-only
  metadata.json
  summary.txt
```

- `raw_elm.log` abre com `O_APPEND`;
- flush a cada linha;
- `fsync` a cada 1 s;
- `--session` existente continua o arquivo;
- sessão nova ganha diretório próprio.

## 5. Multi-PID

Teste:

```bash
go run ./cmd/obd-logger-v2 --port ... --pids 010C,010D --multi 2
```

O host:

1. envia `010C 010D`;
2. verifica se os dois `41<pid>` vieram;
3. se faltar, repete individualmente.

O suporte depende do ELM e da ECU. O log antigo mostra que `010C 010D`
funcionou, mas isso precisa ser confirmado no hardware atual.

## 6. Teste de bancada

```bash
# 1. firmware
tinygo flash -target=m5stick-c -port=... ./poc/obd2/obd-raw-bridge

# 2. logger
go run ./cmd/obd-logger-v2 --port=... --out data --pids 010C --verbose
```

Verificar:

- `HELLO_ACK`;
- `SER=OK`;
- `BLE=SCAN` → `BLE=ON`;
- handshake ELM;
- `START_ACK`;
- `Q/R`;
- `raw_elm.log` append-only;
- `SERIAL=DISC` ao fechar o logger.

## 7. Limitações

- se o adaptador estiver desligado, o logger termina com `BLE: timeout`;
- o `--verbose` mostra cada `HELLO`/`HB`/`S`;
- sem PC, não há polling;
- sem parser de ELM no M5: toda interpretação é no PC;
- `ELM=RX` significa “chegou resposta”, não “OK”;
- o BLE pode reconectar, mas o `Deinit` do `espradio` não existe;
- multi-PID é opcional e dependente do adaptador;
- a UI não mostra valores de PID;
- `C,MEM` reporta heap; use `--mem-every 10s` para acompanhar.
