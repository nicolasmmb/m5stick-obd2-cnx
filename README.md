# M5StickC Plus2 + TinyGo — OBD2 BLE

Repo focado em ler um adaptador **ELM327 BLE** com o M5StickC Plus2 e exibir
dados do carro no LCD.

> Guia de hardware e pinout: **[M5STICKC-PLUS2-GUIDE.md](M5STICKC-PLUS2-GUIDE.md)**.
> Catálogo de PIDs, siglas e processos: **[docs/OBD2-DATA-CATALOG.md](docs/OBD2-DATA-CATALOG.md)**.
> Tabela por dado, confiabilidade e frequência: **[docs/OBD2-DECODED-DATA.md](docs/OBD2-DECODED-DATA.md)**.
> Ideias de ML e modelagem: **[docs/ml/PROJECT-IDEAS.md](docs/ml/PROJECT-IDEAS.md)**.

## Status

| Item | Resultado |
|---|---|
| TinyGo + flash + serial | ✅ |
| BLE GATT no ELM327 | ✅ `FFF0/FFF1/FFF2` |
| Handshake ELM327 | ✅ `ELM327 v2.1` |
| PIDs Mode 01 | ✅ 43 suportados |
| UI OBD2 | ✅ 3 páginas, sem flicker/ghosting |
| Data loader | ✅ serial compacta → CSV long/wide; replay testado |
| taxa | ~5–7 Hz por PID; varredura completa ~7,2 s |

## Hardware

- M5StickC Plus2, ESP32-PICO-V3-02, 8 MB flash.
- LCD ST7789V2 135×240.
- Adaptador OBD2 **BLE** `OBDII` (`ELM327 v2.1`).
- Serviço `FFF0`, notify `FFF1`, write `FFF2`.
- Porta serial CH9102 (`/dev/cu.usbserial-*`).

## Estrutura

```text
poc/obd2/
  obd-probe/       # descobre e enumera o GATT do ELM327
  obd-client/      # handshake ELM327 + PIDs no serial
  obd-bench/       # lista os 43 PIDs e mede latência/Hz
  obd-tune/        # compara ATAT/ATST
  obd-display/     # UI de 3 páginas no LCD
  obd-logger/      # firmware compacto + tela de status/dados

cmd/obd-logger/    # loader host: serial compacta/NDJSON legado → CSV
internal/obdlog/   # parser compacto/NDJSON, CSV long/wide e metadados
testdata/          # amostras compacta e NDJSON para replay

examples/
  display-scan/    # referência de LCD + Wi-Fi/BLE
  ble-scan/        # referência de scan BLE
  wifi-scan/       # referência de scan Wi-Fi
  hello/           # smoke test da placa

docs/
  OBD2-DATA-CATALOG.md
  OBD2-DECODED-DATA.md
  ml/PROJECT-IDEAS.md
  PROJECT-STATUS.md  # status atual e próximos passos
  logger/            # documentação de cada arquivo do data loader
  ui-layout.md       # grade e páginas da tela
  hardware/          # notas técnicas de display, sensores e Units

logs/
  obd-bench-2026-11-01.log
  obd-display-2026-11-02.log
```

POCs de Units Grove, microfone e outros sensores foram movidos para
`../m5stick-hardware-pocs/` para manter este repo focado em OBD2.

## Comandos

```bash
make build       # compila OBD2 + examples
make check       # gopls check nos arquivos ativos

make obd         # flash + monitor da UI de 3 páginas
make obd-probe   # enumera GATT do adaptador
make obd-client  # handshake + PIDs no serial
make obd-bench   # benchmark dos 43 PIDs
make obd-tune    # teste ATAT/ATST
make obd-logger  # firmware compacto + tela de status/dados

make display     # exemplo de LCD
make ble         # exemplo de scan BLE
make wifi        # exemplo de scan Wi-Fi
make hello       # smoke test

make logger      # compila o loader Go em /tmp/obd-logger
make log         # lê a serial e grava em data/
make replay      # testa o loader com testdata/obd-logger-sample-compact.txt
make test        # testes do loader/parser

make monitor     # monitor serial apenas
```

Direto com TinyGo:

```bash
tinygo flash -target=m5stick-c -port=/dev/cu.usbserial-XXXX \
  -size short -monitor ./poc/obd2/obd-display
```

## Data loader

O M5Stick não armazena dados. Ele transmite **linhas compactas** pela serial e o host decodifica/grava
em `data/<sessão>/`:

```text
raw.log
decoded.ndjson
samples_long.csv
samples_wide_1hz.csv
events.jsonl
metadata.json
invalid.log
```

Formato completo: **[docs/OBD2-LOGGER-FORMAT.md](docs/OBD2-LOGGER-FORMAT.md)**.
Documentação por arquivo: **[docs/logger/](docs/logger/)**.
Layout da tela: **[docs/ui-layout.md](docs/ui-layout.md)**.

Fluxo real:

```bash
make obd-logger   # grava o firmware compacto
# Ctrl-C para fechar o monitor serial
make log          # abre a serial e grava em data/
```

Teste sem carro:

```bash
make replay       # usa testdata/obd-logger-sample-compact.txt
make replay-device FILE=data/<sessão>/raw.log  # envia replay ao Stick
```

Ou direto:

```bash
go run ./cmd/obd-logger --port=/dev/cu.usbserial-XXXX --baud=115200 --out=data
go run ./cmd/obd-logger --replay=testdata/obd-logger-sample-compact.txt --out=data
```

- `samples_long.csv`: uma linha por PID lido.
- `samples_wide_1hz.csv`: reamostragem com último valor e `age_ms` por PID.
- `data/` é ignorado pelo git.

## UI OBD2

### Páginas

| Página | Dados (máx. 3) | Fonte |
|---|---|---|
| 1 | velocidade, RPM, combustível em litros | `010D`, `010C`, `012F` |
| 2 | ECT, IAT, catalisador | `0105`, `010F`, `013C` |
| 3 | % álcool, % gasolina, tipo | `0152`, derivado, `0151` |

- **A** = próxima página; **B** = anterior.
- Litros: `raw_2F / 255 * capacidade`. O padrão é **44,0 L**
  (`tankCapacityDeciL = 440`) em `poc/obd2/obd-display/main.go`.
- Layout calculado por margens/gaps e largura real dos textos
  (`tinyfont.LineWidth`), com valores centralizados.
- Framebuffer 4bpp + flush apenas das linhas alteradas: sem flicker/ghosting.

### Protocolo

```text
ATZ -> ELM327 v2.1
ATE0, ATL0, ATS0, ATSP0 -> OK
0100 -> 4100BE3FB813AA
010C -> 410C0000AAAAAA
0105 -> 410561AAAAAAAA
```

Respostas CAN são preenchidas com `AA`; o parser ignora o padding.

### Taxa medida

- 1 PID: ~5–7 Hz.
- 43 PIDs: ~7,2 s por varredura (~0,14 Hz).
- `AT*`: 35–50 ms.
- Consulta só dos PIDs da página atual; combustível/etanol a cada 8–10 ciclos.

Mais detalhes em **[docs/OBD2-DATA-CATALOG.md](docs/OBD2-DATA-CATALOG.md)**.

## Exemplos mantidos

| Exemplo | Para que serve |
|---|---|
| `display-scan` | LCD, abas, botões e framebuffer |
| `ble-scan` | scan BLE com nomes/service UUIDs |
| `wifi-scan` | scan Wi-Fi com `espradio` |
| `hello` | validar toolchain, serial e LED |

## Resultados de build (`-size short`)

| POC | flash | RAM estática |
|---|---|---|
| hello | 9,1 KB | 4,7 KB |
| ble-scan | 228 KB | 17,8 KB |
| wifi-scan | 472 KB | 41,6 KB |
| display-scan | 598 KB | 53,8 KB |
| obd-probe | 236 KB | 17,8 KB |
| obd-client | 240 KB | 17,9 KB |
| obd-bench | 300 KB | 19,3 KB |
| obd-tune | 295 KB | 19,3 KB |
| obd-display | 292 KB | 72,8 KB (16,2 KB de framebuffer) |
| obd-logger | 304 KB | 50,9 KB (16,2 KB de framebuffer) |

## Ambiente

- macOS 27.0.1 arm64
- Go 1.26.3
- TinyGo 0.44.0 (LLVM 22.1.4)
- Target: `m5stick-c`
- Drivers: `tinygo.org/x/drivers` v0.37.0
- Bluetooth: `tinygo.org/x/bluetooth` v0.17.0
- Wi-Fi: `tinygo.org/x/espradio` v0.5.0
- Fontes: `tinygo.org/x/tinyfont` v0.6.0

## Problemas comuns

| Sintoma | Ação |
|---|---|
| `no serial port` | feche `screen`/monitor e reconecte o USB |
| flash falha | cabo de dados, porta correta, reset/bootloader |
| tela deslocada | offsets 40/52 + rotação 90 |
| cores trocadas | `display.IsBGR(true)` |
| BLE não acha o adaptador | ignição ligada, adaptador pareado/energizado |
| OBD2 lento | normal: ~5–7 Hz por PID; consultar só o necessário |
| RAM estourando | `-size full`; reduzir buffers |
