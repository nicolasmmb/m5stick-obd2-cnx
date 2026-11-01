# M5StickC Plus2 + TinyGo — POC Wi-Fi / BLE / Tela (OBD2)

POC para validar TinyGo, Wi-Fi, BLE e o LCD do M5StickC Plus2 antes de integrar com OBD2.

## Status

| Fase | Descrição | Resultado |
|---|---|---|
| A | TinyGo 0.44.0 + flash + serial | ✅ |
| B1 | Wi-Fi scan (espradio v0.5.0) | ✅ 25–36 APs |
| B2 | BLE scan (bluetooth v0.17.0) | ✅ 10–14 devices, nomes recebidos |
| B3 | Tela ST7789 + Wi-Fi + BLE juntos | ✅ 25–36 APs / 12–16 BLE |
| C | OBD2 | ⏸️ bloqueado: adaptador ELM327 Bluetooth Classic |

## Ambiente

- macOS 27.0.1 arm64
- Go 1.26.3
- TinyGo 0.44.0 (LLVM 22.1.4) em `~/.local/tinygo`
- Target: `m5stick-c` (ESP32; o Plus2 usa ESP32-PICO-V3-02)
- Porta serial: CH9102 (`/dev/cu.usbserial-*`)

## Hardware

- ESP32-PICO-V3-02, 520 KB SRAM, 8 MB flash, 2 MB PSRAM (PSRAM não usada pelo TinyGo)
- LCD ST7789V2 135x240: MOSI=G15, CLK=G13, DC=G14, RST=G12, CS=G5, BL=G27
- HOLD (bateria): GPIO4
- USB-serial CH9102: passar `-port=` explicitamente

## Estrutura

```
poc/hello/        # print + tick + LED
poc/wifi-scan/    # scan de redes Wi-Fi (serial)
poc/ble-scan/     # scan BLE com nomes (serial)
poc/display-scan/ # tela: Wi-Fi + BLE, páginas alternadas
```

## Comandos

```bash
make build       # compila os quatro POCs e mostra tamanho
make hello       # flash + monitor
make wifi        # flash + monitor
make ble         # flash + monitor
make display     # flash + monitor (POC da tela)
make monitor     # monitor serial apenas
```

Ou diretamente:

```bash
tinygo flash -target=m5stick-c -port=/dev/cu.usbserial-XXXX \
  -size short -monitor ./poc/ble-scan
```

## Resultados medidos (`-size short`)

| POC | flash | RAM estática |
|---|---|---|
| hello | 9,1 KB | 4,7 KB |
| ble-scan | 228 KB | 17,8 KB |
| wifi-scan | 472 KB | 41,6 KB |
| display-scan | 581 KB | 52,0 KB |

O BLE no ESP32 clássico reserva ainda 64 KB fixos de RAM para o controlador.

## POC da tela (`poc/display-scan`)

- LCD ST7789V2 em landscape 240x135, `Rotation90`, `RowOffset=40`,
  `ColumnOffset=52`, cor BGR.
- Dois painéis simultâneos:
  - Topo (ciano): `WIFI nn` + até 4 redes (`RSSI SSID`).
  - Base (verde): `BLE nn` + até 4 dispositivos (`RSSI nome` ou final do MAC).
- Ciclo: scan Wi-Fi (~4 s) → atualiza topo → scan BLE (8 s) → atualiza base
  → espera 15 s → repete.

## Zed (autocomplete e diagnósticos)

O projeto inclui `.zed/settings.json`, que aponta o `gopls` para
`scripts/gopls-tinygo.sh`. O wrapper lê `tinygo info m5stick-c` e exporta o
`GOROOT` cacheado, `GOOS`, `GOARCH` e as build tags do TinyGo antes de iniciar
o `gopls`. Assim o Zed entende `machine`, `espradio` e `bluetooth` sem afetar
outros projetos Go.

Requisitos:

```bash
which gopls    # ~/go/bin/gopls
gopls version  # v0.21.1 ou mais novo
```

Validação pela linha de comando:

```bash
make check     # roda gopls check nos quatro POCs
```

No Zed, abra a pasta `m5stick-obd2-cnx` como raiz do projeto e reinicie o
language server (`zed: restart language server`) se ele já estava rodando.
Se mover o repositório, ajuste o caminho absoluto em `.zed/settings.json`.

## Observação sobre OBD2

`tinygo.org/x/bluetooth` + `espradio` implementam **somente BLE (GATT)**. ELM327
Bluetooth Classic (SPP) não é suportado. Para a Fase C é necessário:

1. adaptador OBD2 **BLE** (recomendado), ou
2. ponte HC-05 mestre ligada à UART do M5, ou
3. adaptador OBD2 **Wi-Fi** via TCP.
