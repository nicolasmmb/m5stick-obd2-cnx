# Otimização do firmware — plano e o que foi feito

> Branch: `feat/obd-replay` (mergeada na `main` em `d0da8ad`)
> Escopo: M5StickC Plus2 + ELM327 BLE + replay serial.
> Este documento registra o plano de otimização, o que foi implementado, as
> medições antes/depois e o que ficou pendente ou fora de escopo.

## 1. Objetivo

Manter a UI estável a ~60 fps, sem OOM/freeze, com replay serial, e:

- evitar alocações no hot path;
- usar buffers de tamanho fixo;
- redesenhar só os tiles sujos;
- ter heartbeat visual no rodapé;
- desligar o BLE quando o modo for serial/replay;
- validar cada mudança com ops/allocs/timing.

## 2. Diagnóstico

O travamento não era a tela congelando: o firmware morria com
`fatal error: out of memory`.

Evidências coletadas no serial:

```text
E,2068,DBG,heap=37600,ble=0
E,4068,DBG,heap=52576,ble=0
E,6068,DBG,heap=63696,ble=0
```

- heap crescia ~5,5 KB/s durante o replay, mesmo com BLE desligado;
- o parser usava `string()`, `strings.SplitN`, `ToUpper`, `strconv` e
  `decodeHex` por registro;
- o cache era `map[string]Reading` e `Reading` retinha `Raw []byte`;
- quando o `C,REPLAY` se perdia no reset do USB, o firmware iniciava o BLE,
  consumia a memória restante e estourava o heap.

## 3. O que foi implementado

### 3.1 Parser serial sem alocação

- novo pacote `internal/obdline`;
- interpreta `A`, `C,REPLAY`, `C,LIVE`, `C,CLEAR`, `R,P`;
- PID em `[2]byte`, payload em `[8]byte`, sem strings;
- `I`, `E` e `X` são ignorados, como antes;
- `SerialLink` usa buffer fixo de 2048 bytes, sem `append` por linha.

### 3.2 Registry e cache

- `obdpid.Registry` indexa decoders por chave `uint16` (dois bytes ASCII);
- `DecodeBytes` evita criar string temporária;
- `Reading` não tem mais `Raw []byte`;
- `obdstore.Cache` virou tabela fixa de 64 slots com chave `[2]byte`;
- `Clear` é reset de array, sem `make`;
- caminho de replay usa `PutBytes`/`PutQualityBytes`.

### 3.3 Render sem alocação

- `format.go`: inteiros e decimais formatados em buffer fixo;
- removido `strconv` e concatenação do caminho de render;
- `ui.go` desenha texto direto da tabela de glifos, sem a slice de cores e o
  wrapper de rotação do `tinyfont`;
- loop da UI em `12 ms` (medido ~70 ticks/s);
- compose/flush dos tiles sujos a cada tick, sem a espera de 100 ms;
- heartbeat 5×4 px a 2 Hz.

### 3.4 BLE no modo replay

- firmware único (live + replay); o build separado `replayonly` foi descartado;
- `Adapter.Enable()` só é chamado quando o modo live começa;
- janela de 3 s no boot para o host entrar em replay antes de ligar o BLE;
- `StopScan` não é chamado antes do `Enable` (evita `HCI invalid packet`);
- scan com retry e timeout de 10 s;
- conexão encerrada com `defer dev.Disconnect()`;
- respostas `K,REPLAY` e `K,LIVE` para handshake.

**Limitação:** `espradio` não expõe `Deinit`. Se o BLE foi ligado uma vez, a
região reservada no linker não volta ao heap; nesse caso o firmware só para o
scan e desconecta.

### 3.5 Host de replay

- `--reset`: pulso de RTS reinicia a placa (DTR alto para não entrar em modo
  download);
- `C,REPLAY` repetido a cada 300 ms até receber `K,REPLAY`;
- a serial é sempre drenada, mesmo sem `--echo`;
- `--echo` mostra a saída do firmware.

## 4. Medições

| Caminho | Tempo | Alocações |
| --- | --- | --- |
| `obdwire.ParseCommand` (host, antes) | 95 ns/op | 1 alloc/op |
| `obdline.Parse` (Stick) | 31 ns/op | 0 allocs/op |
| `Registry.DecodeBytes` | 48 ns/op | 0 allocs/op |
| `Cache.Put` + `Get` | 39 ns/op | 0 allocs/op |
| `Cache.Put` + `Get` (map, antes) | 45 ns/op | 0 allocs/op |

Binário:

| Versão | Flash | RAM estática |
| --- | --- | --- |
| Antes (debug removido) | 348.412 | 54.676 |
| Depois | 302.356 | 59.028 |

A RAM estática subiu por causa da tabela fixa do cache; o heap por amostra foi
reduzido e o crescimento linear sumiu.

## 5. Validação executada

- `go test ./internal/... ./cmd/...` ✅
- `gopls check` no firmware ✅
- `git diff --check` ✅
- replay `--speed 5 --loop`: **90 s / 2.436 registros** sem OOM;
- validação final: **45 s / 1.213 registros**, zero `fatal`, zero `panic`,
  zero `scan is already in progress`;
- handshake `K,REPLAY` confirmado;
- `ble=0` confirmado durante replay;
- após o replay, `C,LIVE` iniciou o scan limpo e terminou em
  `no OBDII BLE device found`.

Firmware gravado durante a validação:

```text
MD5 8585138a1b7afa7c47b2ff1ded051bc5
```

## 6. Pendente de validação

- ~~confirmar visualmente os ~60 fps, botões, cores e páginas 3–6~~ ✅;
- ~~testar `--speed 20` e `--speed 0` (rajada máxima)~~ ✅;
- ~~replay de 5–10 min sem debug para confirmar estabilidade do heap~~ ✅;
- rodar uma sessão real de 5–10 min no carro. ⏳

O P1 (reconexão BLE, `X`/`C,STOP`, DTCs/readiness) está em
[P1-ROBUSTNESS.md](P1-ROBUSTNESS.md).

## 7. Fora do escopo desta otimização

- alocações do caminho live (`emitPID`, `pidData`, `readDiagnostics`,
  `readVIN`, `hexBytes`);
- reconexão automática quando o BLE cai durante o polling;
- `seq`/`session_id` no protocolo e `X` em shutdown/reconexão;
- DTCs/readiness no logger e validação do VIN multi-frame;
- pipeline Parquet/ML.

## 8. Commits

```text
254b5a8 refactor(obdpid): stop retaining raw payload in readings
872e07a perf(obdpid): index decoders by 16-bit PID key
c19e52e test(obdpid): benchmark allocation-free DecodeBytes
09c99f2 feat(obdline): add zero-allocation serial protocol parser
31830c3 test(obdline): cover compact protocol parsing
c4bf55d perf(obdstore): replace map cache with a fixed slot table
b2a55d9 test(obdstore): benchmark fixed-slot cache
4e6f3bb test(obdwire): record host parser allocation baseline
5da3698 perf(firmware): decode replay PIDs without strings
c28f91e refactor(firmware): apply decoded replay records directly
26fa3e0 perf(firmware): parse serial input from a fixed buffer
8866bcf perf(firmware): format UI numbers with fixed buffers
212d110 perf(ui): compose dirty tiles every tick without allocations
3c97bbb refactor(firmware): split live BLE path from mode switching
aad7969 feat(replay): reset the board and handshake before replay
41740f3 docs(replay): document handshake and reset flags
df6b54d docs(ui): describe the 12 ms dirty-tile render loop
7a9432c docs: record performance results and BLE behavior
```

## 9. Como reproduzir

```bash
# build live
tinygo build -target=m5stick-c -size short ./poc/obd2/obd-logger

# gravar
tinygo flash -target=m5stick-c -port=/dev/cu.usbserial-XXXX ./poc/obd2/obd-logger

# replay com reset + handshake
go run ./cmd/obd-replay \
  --port /dev/cu.usbserial-XXXX \
  --file data/20261102-162543/raw.log \
  --speed 5

# benchmarks
go test ./internal/obdline ./internal/obdpid ./internal/obdstore \
  -run=XXX -bench=. -benchmem
```
