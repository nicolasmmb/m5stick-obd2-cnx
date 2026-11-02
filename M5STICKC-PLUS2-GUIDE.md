# M5StickC Plus2 + TinyGo — guia do projeto

> Complemento do [README](README.md) e do
> [guia de criação de libs](LIB-CREATION-GUIDE.md).
> Projeto: <https://github.com/nicolasmmb/m5stick-obd2-cnx>

## 1. Visão geral

O projeto usa o **M5StickC Plus2** com **TinyGo** para validar tela, sensores,
Grove, Wi-Fi e BLE antes de integrar com **OBD2**. Cada unit M5Stack tem uma
lib Go própria, publicada em repositório separado.

## 2. Conexão física e suportes

### 2.1 USB e alimentação

- Conecte o Stick por **USB-C com cabo de dados** (não use cabo só de carga).
- O chip serial é **CH9102** → porta `/dev/cu.usbserial-*` no macOS.
- A bateria é mantida ligada pelo **HOLD em GPIO4**; todo POC faz:
  ```go
  hold := machine.GPIO4
  hold.Configure(machine.PinConfig{Mode: machine.PinOutput})
  hold.High()
  ```
- Botão de power no topo; reset no lado (ou pelo próprio `tinygo flash`).
- Evite prender o Stick em caixa metálica: a antena Wi-Fi/BLE é interna.

### 2.2 Suportes e montagem

- Furos compatíveis com LEGO na traseira (montagem em cases/braços).
- A tela fica na frente; mantenha o Grove acessível.
- Para testes no carro, fixe com fita/velcro longe de metal e do volante.
- Não cubra a área da antena (topo do Stick).

### 2.3 Grove (barramento compartilhado)

O Grove do StickC Plus2 é **I2C1/UART1 em G32/G33** e é **compartilhado**:

- **um unit por vez**;
- não conecte ENV IV + Fingerprint2 + Encoder juntos;
- para UART (GPS/Fingerprint2), o mesmo par G32/G33 vira TX/RX.

### 2.4 Botões

| Botão | GPIO | Uso |
|---|---|---|
| A | G37 | interrupção (falling) |
| B | G39 | interrupção (falling) |
| C | G35 | interrupção; **evite toque longo** (pode desligar o Stick) |
| Power | — | liga/desliga |
| Reset | — | reinicia |

> Em alguns POCs, `GPIO37.Get()`/`GPIO39.Get()` não refletem o botão; use
> interrupção. `GPIO35.Get()` (C) funciona para polling. Toque longo em C pode
> desligar o aparelho — use ações de 2 toques (ex.: B 2×).

## 3. Pinout do M5StickC Plus2

| Função | Pino(s) |
|---|---|
| LCD ST7789V2 135×240 | MOSI=G15, CLK=G13, DC=G14, RST=G12, CS=G5, BL=G27 |
| Grove I2C/UART | SDA/TX=G32, SCL/RX=G33 (`machine.I2C1`/`UART1`) |
| Botões | A=G37, B=G39, C=G35 |
| HOLD (bateria) | G4 |
| Buzzer | G2 (PWM/LEDC) |
| LED vermelho / IR | G19 |
| Bateria (ADC) | G38 (divisor 2.0) |
| IMU MPU6886 | I2C0 G21/G22, **0x68** |
| RTC BM8563 | I2C0 G21/G22, **0x51** |
| Microfone SPM1423 PDM | CLK=G0, DATA=G34 |
| USB serial | CH9102 |

> PSRAM de 2 MB **não é usada** pelo TinyGo neste target.

## 4. Ambiente e versões

| Item | Versão usada |
|---|---|
| macOS | 27.0.1 arm64 |
| Go | **1.26.3** |
| TinyGo | **0.44.0** (LLVM 22.1.4) |
| TinyGo path | `~/.local/tinygo` |
| Target | **`m5stick-c`** (ESP32; Plus2 = ESP32-PICO-V3-02) |
| `tinygo.org/x/drivers` | v0.37.0 |
| `tinygo.org/x/tinyfont` | v0.6.0 |
| `tinygo.org/x/bluetooth` | **v0.17.0** |
| `tinygo.org/x/espradio` | **v0.5.0** |

Libs do projeto (tags usadas no `go.mod`):

| Unit | Lib | Tag |
|---|---|---|
| Fingerprint2 | `M5Unit-Fingerprint2-Go` | v0.1.0 |
| Encoder U135 | `M5Unit-Encoder-Go` | v0.1.2 |
| GPS v1.1 | `M5Unit-GPS-Go` | v0.3.1 |
| ToF-90 | `M5Unit-ToF90-Go` | v0.3.0 |
| Joystick2 | `M5Unit-Joystick2-Go` | v0.2.1 |
| NFC U216 | `M5Unit-NFC-Go` | v0.8.1 |

Atualizar uma lib:

```bash
GOPROXY=direct go get github.com/nicolasmmb/M5Unit-NFC-Go@v0.8.1
```

## 5. Build, flash e monitor

```bash
make build            # compila todos os POCs e mostra tamanho
make <poc>            # flash + monitor de um POC
make check            # gopls check em todos os arquivos
```

Exemplos:

```bash
make hello
make wifi
make ble
make display
make sensors
make mic
make env
make finger
make encoder
make gps
make tof
make joystick2
make nfc
```

Direto com TinyGo:

```bash
tinygo build -target=m5stick-c -size short -o /tmp/x.bin ./poc/hello
tinygo flash -target=m5stick-c -port=/dev/cu.usbserial-XXXX -size short -monitor ./poc/hello
tinygo monitor -port=/dev/cu.usbserial-XXXX
```

- Descubra a porta com `ls /dev/cu.*` (CH9102).
- `-size short` mostra `code/data/bss/flash/ram`.
- `-monitor` abre o serial depois do flash.
- Se a porta estiver ocupada, feche `screen`/monitor antes de gravar.

## 6. Wi-Fi (`espradio` v0.5.0)

O POC `poc/wifi-scan` valida o rádio e lista APs.

```go
import "tinygo.org/x/espradio"

if err := espradio.Enable(espradio.Config{}); err != nil { ... }
if err := espradio.Start(); err != nil { ... }
aps, err := espradio.Scan()
var stats espradio.Stats
espradio.ReadStats(&stats)
```

Notas:

- `Enable` inicializa o rádio; `Start` sobe a stack.
- `Scan` retorna SSID/RSSI (2,4 GHz).
- Medido no projeto: **25–36 APs**.
- Wi-Fi + BLE juntos: validado no `poc/display-scan` (25–36 APs / 12–16 BLE).
- Para OBD2 via Wi-Fi será necessário abrir **TCP** para o adaptador; o POC
  atual só faz scan.

## 7. BLE (`tinygo.org/x/bluetooth` v0.17.0)

O POC `poc/ble-scan` usa **BLE GATT**:

```go
var adapter = bluetooth.DefaultAdapter
_ = adapter.Enable()
adapter.SetScanType(bluetooth.ScanTypeActive)
err := adapter.Scan(func(_ *bluetooth.Adapter, r bluetooth.ScanResult) {
    println(r.Address.String(), r.RSSI, r.LocalName())
})
```

Notas:

- **Somente BLE** (GATT). **Bluetooth Classic/SPP não é suportado.**
- Active scan pede SCAN_RSP para obter nomes.
- Medido: **10–14 dispositivos**, nomes recebidos.
- O `poc/ble-scan` destaca nomes com `OBD`, `ELM`, `VLINK`, `ICAR`.

## 8. Projeto OBD2

### 8.1 Status

**Fase C pausada**: o adaptador ELM327 disponível é **Bluetooth Classic**,
que o TinyGo não implementa. O restante do projeto (tela, sensores, Wi-Fi,
BLE, units) já foi validado.

### 8.2 Opções de conexão

1. **Adaptador OBD2 BLE** (recomendado) — GATT, funciona com
   `tinygo.org/x/bluetooth`.
2. **Ponte HC-05 mestre** ligada à UART do Stick (G32/G33) — converte Classic
   em serial.
3. **Adaptador OBD2 Wi-Fi** — conectar via TCP usando `espradio`.

### 8.3 Arquitetura planejada

```
[M5StickC Plus2] --BLE GATT / Wi-Fi TCP / UART--> [ELM327] --CAN--> [ECU]
        |                                              |
        +-- parser AT/PID --> tela / log / DTC <--------+
```

Camadas:

1. **Transporte**: BLE GATT (FFF0/FFF1/FFF2 típicos), TCP ou UART.
2. **Cliente ELM327**: `AT Z`, `ATE0`, `AT SP0`, `AT DP`, `0100`, `010C`...
3. **Parser de PID**: RPM, velocidade, temperatura, throttle, intake, fuel
   trims, DTCs (`03`, `07`).
4. **UI**: tela com páginas (métricas, DTCs, status do adaptador).
5. **Log**: serial e, se necessário, buffer em RAM.

### 8.4 PIDs iniciais sugeridos

| PID | Métrica |
|---|---|
| `010C` | RPM |
| `010D` | velocidade |
| `0105` | temperatura do líquido |
| `0111` | throttle |
| `010F` | temperatura do ar |
| `010E` | avanço |
| `03` | DTCs confirmados |

Ainda **não há código OBD2** no repositório; a Fase C depende do adaptador
adequado.

## 9. Suportes (units) e configuração

| Unit | Barramento | Endereço/baud | Lib | Status |
|---|---|---|---|---|
| Fingerprint2 A-K323CP | UART Grove | 115200, TX G32/RX G33 | Fingerprint2-Go | ✅ |
| Encoder U135 | I2C Grove | 0x40, 100 kHz | Encoder-Go | ✅ |
| GPS v1.1 AT6668 | UART Grove | 115200 (fallback 9600) | GPS-Go | ✅ |
| Mini ToF-90 VL53L0C | I2C Grove | 0x29 | ToF90-Go | ✅ |
| Joystick2 U024-V2 | I2C Grove | 0x63, 100 kHz | Joystick2-Go | ✅ |
| NFC U216 ST25R3916 | I2C Grove | 0x50, 400 kHz | NFC-Go | ✅ |
| ENV IV SHT40/BMP280 | I2C Grove | 0x44/0x76 | — | ✅ local |

Regras:

- Grove é **um por vez**; o POC precisa ser recompilado/flashado para o unit
  em uso (ou usar um POC multi-unit, se existir).
- Unidades I2C usam `machine.I2C1` com SDA=G32/SCL=G33.
- Unidades UART usam `machine.UART1` com TX=G32/RX=G33.
- Ao adicionar um unit, siga o
  [guia de criação de libs](LIB-CREATION-GUIDE.md) e atualize:
  - `go.mod` (tag da lib);
  - `Makefile` (`build`, `check`, target de flash);
  - `README.md` (seção e lista de POCs);
  - `poc/<unit>` e, se houver escrita, o gate de confirmação.

## 10. Testes e debug

### 10.1 Testes de host (libs)

```bash
cd ../M5Unit-NFC-Go
make test          # go test -race
make alloc         # 0 allocs + footprint
make bench         # tx/op, ns/op, B/op
make bench-save NAME=before
make bench-compare FROM=before TO=after
make profile       # CPU/alloc profiles (pprof)
make escape        # heap escapes
```

### 10.2 Testes de hardware

- Grave o POC com `make <poc>`.
- Capture o serial:

  ```bash
  screen -dmS cap -L /dev/cu.usbserial-XXXX 115200
  # ... teste ...
  screen -S cap -X quit
  LC_ALL=C tr -cd '\11\12\15\40-\176' < screenlog.0 | tail
  ```

- **3 beeps** antes de pedir ação do usuário:

  ```bash
  for i in 1 2 3; do afplay /System/Library/Sounds/Glass.aiff; done
  ```

- Valide a matriz de hardware (cartão/tag, UID/IDm, tipo, leitura/escrita).
- Para escrita: confirmação explícita, whitelist de bloco/página e read-back.

### 10.3 Debug TinyGo

```bash
tinygo build -target=m5stick-c -size full ./poc/nfc     # mapa de tamanho
tinygo build -target=m5stick-c -print-allocs=. ./poc/nfc
tinygo build -target=m5stick-c -scheduler=async ./poc/nfc
tinygo build -target=m5stick-c -gc=leaking ./poc/nfc
tinygo gdb -target=m5stick-c ./poc/nfc                  # se disponível
```

- `-size full` lista pacotes que mais ocupam flash/RAM.
- `-print-allocs` mostra alocações por pacote.
- `-gc=leaking` remove o GC (útil para medir; não usar em produção).
- Logs via `println` aparecem no monitor serial.
- `make check` usa `scripts/gopls-tinygo.sh` para o `gopls` entender
  `machine`, build tags e GOROOT do TinyGo.

### 10.4 Problemas comuns

| Sintoma | Causa/ação |
|---|---|
| `no serial port` / porta ocupada | feche `screen`/monitor; reconecte o USB |
| flash falha | cabo de dados, porta correta, reset, modo bootloader |
| tela com cores trocadas | `display.IsBGR(true)` |
| tela deslocada | offsets 40/52 + rotação 90 |
| I2C timeout em 400 kHz | testar 100 kHz (alguns units) |
| unit não responde | Grove certo (G32/G33), um por vez, alimentação |
| botão A/B não lê por `Get()` | usar interrupção; C lê |
| C longo desliga o Stick | evitar; usar B 2× |
| PSRAM ausente | TinyGo não usa os 2 MB |
| Wi-Fi/BLE instável | antena interna, longe de metal, fonte estável |
| RAM estourando | `-size full`, reduzir buffers/prints |

## 11. Checklist rápido

- [ ] TinyGo 0.44.0 e Go 1.26.3.
- [ ] HOLD G4 em HIGH.
- [ ] Porta CH9102 correta.
- [ ] Grove G32/G33, um unit por vez.
- [ ] `make check` e `make build` verdes.
- [ ] Log serial limpo (sem timeouts).
- [ ] 3 beeps antes de ação do usuário.
- [ ] Escrita só com confirmação e read-back.
- [ ] Atualizar README/PLAN após validar.
