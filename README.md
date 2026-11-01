# M5StickC Plus2 + TinyGo — POC Wi-Fi / BLE / Tela (OBD2)

POC para validar TinyGo, Wi-Fi, BLE e o LCD do M5StickC Plus2 antes de integrar com OBD2.

## Status

| Fase | Descrição | Resultado |
|---|---|---|
| A | TinyGo 0.44.0 + flash + serial | ✅ |
| B1 | Wi-Fi scan (espradio v0.5.0) | ✅ 25–36 APs |
| B2 | BLE scan (bluetooth v0.17.0) | ✅ 10–14 devices, nomes recebidos |
| B3 | Tela ST7789 + Wi-Fi + BLE juntos | ✅ 25–36 APs / 12–16 BLE |
| B4 | Menu interativo de sensores na tela | ✅ IMU, RTC, bateria, botões, buzzer, LED/IR |
| B5 | Lib SPM1423 PDM (I2S0 + DMA) | ✅ RMS/dBFS; tom de 2,7 kHz detectado |
| B6 | Espectrograma FFT na tela | ✅ FFT 512 + waterfall 120x50 |
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
poc/display-scan/ # tela: Wi-Fi + BLE, abas e navegação por botões
poc/sensors/      # menu interativo de sensores na tela
poc/mic-pdm/      # teste de RMS/dBFS do microfone na serial
lib/spm1423/      # driver PDM do SPM1423 usando I2S0 + DMA
```

## Comandos

```bash
make build       # compila os seis POCs e mostra tamanho
make hello       # flash + monitor
make wifi        # flash + monitor
make ble         # flash + monitor
make display     # flash + monitor (POC da tela)
make sensors     # flash + monitor (POC de sensores)
make mic         # flash + monitor (POC do microfone)
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
| sensors | 142 KB | 31 KB + 65 KB do framebuffer |
| mic-pdm | 23 KB | 4,8 KB + 1 KB de DMA |

O BLE no ESP32 clássico reserva ainda 64 KB fixos de RAM para o controlador.

## POC da tela (`poc/display-scan`)

- LCD ST7789V2 em landscape 240x135, `Rotation90`, `RowOffset=40`,
  `ColumnOffset=52`, cor BGR.
- Abas: **WIFI** (ciano) e **BLE** (verde), com contador de cada uma.
- Lista com 9 linhas ordenadas por RSSI; a selecionada fica com fundo branco.
- Navegação:
  - **A** (G37, frontal/central): alterna a aba.
  - **B** (G39, lateral superior): sobe na lista.
  - **C** (G35, lateral/power): desce na lista.
- A lista rola sozinha quando a seleção passa da 9ª linha.
- Redesenho parcial: ao navegar só as linhas antiga/nova são atualizadas,
  sem `FillScreen` — evita o piscar da tela inteira.
- Ciclo de scan: Wi-Fi (~4 s) → BLE (8 s) → pausa de 5 s → repete.

## POC de sensores (`poc/sensors`)

Menu interativo de 7 páginas, renderizado em framebuffer (sem piscar):

| Página | O que mostra |
|---|---|
| IMU MPU6886 | aceleração X/Y/Z (g), giroscópio X/Y/Z (°/s), temperatura |
| RTC BM8563 | data, hora, dia da semana e status |
| BATERIA | tensão, porcentagem e barra (ADC GPIO38) |
| BOTOES | estado e contador de A/B/C |
| BUZZER | tom de 2700 Hz em G2 |
| LED / IR | estado do LED vermelho/IR em G19 |
| MICROFONE | SPM1423 PDM ao vivo: RMS, dBFS e barra de nível |
| ESPECTRO | FFT 512 pontos, waterfall 0–7,8 kHz e escala de cor dBFS |

Navegação:

- **A** (G37): próxima página.
- **B** (G39): página anterior.
- **C** (G35): ação da página (ajustar RTC, zerar contadores, bipar, alternar LED).

Notas técnicas:

- O ADC do ESP32 exige `machine.InitADC()` antes do `Configure`.
- O framebuffer RGB565 de 240x135 usa ~65 KB de RAM.
- O microfone usa a lib `lib/spm1423` (I2S0 + DMA); na página MICROFONE,
  **C** toca um bip de 2,7 kHz para validar o nível.
- A página ESPECTRO usa FFT de 512 pontos com janela Hann:
  - 120 colunas (tempo) × 50 linhas (frequência), atualização ~10 Hz.
  - Eixo de 0 a ~7,8 kHz; a cor vai de azul (fraco) a vermelho (forte).
  - **C** toca o bip e desenha uma linha vertical em ~2,7 kHz.
- O buzzer usa PWM (LEDC) em G2, porque o driver bit-banged não é confiável
  com o scheduler do ESP32.

## Lib do microfone (`lib/spm1423`)

Driver PDM para o SPM1423 do M5StickC Plus2. O TinyGo não implementa
`machine.I2S` no ESP32 clássico, então a lib configura o periférico **I2S0**
diretamente e usa o conversor **PDM→PCM em hardware**.

Uso:

```go
mic := spm1423.New(machine.GPIO0, machine.GPIO34)
err := mic.Configure(spm1423.Config{BufferSize: 512})
mic.Start()
defer mic.Stop()

buf := make([]int16, 512)
n, err := mic.Read(buf)
rms := spm1423.RMS(buf[:n])
dbfs := spm1423.DBFS(rms)
```

Detalhes:

- Clock PDM em **G0** (`I2S0I_WS_OUT`), dados em **G34** (`I2S0I_DATA_IN15`).
- MCLK 8 MHz (160 MHz / 20), BCLK 1 MHz, amostragem ~**15,625 kHz**.
- `RXEOF_NUM` configurado para o tamanho do buffer; DMA com descritor de 12 bytes.
- `RMS` remove o offset DC; `DBFS` é relativo ao fundo de escala de 16 bits.
- `Read` bloqueia até o buffer DMA encher (512 amostras ≈ 33 ms).

Validação real com um tom de **2,7 kHz** no buzzer:

```
silêncio:  rms ~16   dbfs -66
bip:       rms ~6400 dbfs -14
```

Cuidados:

- O valor em dBFS **não é SPL calibrado**; para SPL de verdade é preciso
  calibrar com um medidor de referência.
- O microfone PDM tem offset DC; use sempre `RMS`/`DBFS` em vez do valor bruto.
- A taxa é nominal de 15,625 kHz (divisor inteiro). Para 16 kHz exatos seria
  preciso usar divisor fracionário no `CLKM_CONF`.

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
