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
| B6 | Espectrograma FFT na tela | ✅ FFT 256 + waterfall, fundo preto e pico |
| B7 | Bola com física do IMU | ✅ 60 fps, dirty rects e 5 presets |
| B8 | Simulador de areia | ✅ 1.000 / 10.000 grãos, grid 240×84 |
| B9 | ENV IV no Grove (SHT40 + BMP280) | ✅ temp/umidade/pressão/altitude |
| B10 | Fingerprint2 (A-K323CP) no Grove | ✅ tela, captura/busca, LED ring |
| C | OBD2 | ⏸️ bloqueado: adaptador ELM327 Bluetooth Classic |

## Ideias de sensores e apps

Catálogo completo em [docs/ideias-sensores-apps.md](docs/ideias-sensores-apps.md):
quick wins, apps para o carro, sensores M5Stack, projetos combinados, notas de
hardware e checklist de sensores.

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
poc/env-iv/       # teste serial do ENV IV (SHT40 + BMP280)
poc/finger2/      # tela do Unit Fingerprint2 (UART Grove)
lib/finger2/      # protocolo UART do A-K323CP
lib/spm1423/      # driver PDM do SPM1423 usando I2S0 + DMA
docs/             # ideias de sensores, apps e projetos
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
make env         # flash + monitor (POC do ENV IV)
make finger      # flash + monitor (tela do Fingerprint2)
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
| sensors | 149 KB | 39 KB + 65 KB do framebuffer |
| mic-pdm | 23 KB | 4,8 KB + 1 KB de DMA |
| env-iv | 34 KB | 4,7 KB |
| finger2 | 98 KB | 12,3 KB |

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
| ENV IV | SHT40 (temp/umidade) + BMP280 (pressão/altitude) no Grove |
| FINGER2 | tela do Unit Fingerprint2: captura, busca, ID/score e LED ring |
| ESPECTRO | FFT 256, waterfall 0–7,8 kHz em fundo preto, pico em kHz/dB |
| BOLA IMU | bola que cai com a gravidade do IMU e quica nas bordas |
| AREIA | simulador de areia com 1.000 ou 10.000 grãos |

Navegação:

- **A** (G37): próxima página.
- **B** (G39): página anterior.
- **C** (G35): ação da página (ajustar RTC, zerar contadores, bipar, alternar LED).

Notas técnicas:

- O ADC do ESP32 exige `machine.InitADC()` antes do `Configure`.
- O framebuffer RGB565 de 240x135 usa ~65 KB de RAM.
- O microfone usa a lib `lib/spm1423` (I2S0 + DMA); na página MICROFONE,
  **C** toca um bip de 2,7 kHz para validar o nível.
- A página ESPECTRO usa FFT de 256 pontos com janela Hann:
  - Waterfall de 120 colunas × 50 linhas, atualização ~10 Hz.
  - dBFS normalizado por 32768 e offset DC removido: silêncio fica **preto**.
  - Cores: verde → amarelo → vermelho conforme o nível.
  - Mostra o pico em kHz e dBFS no topo.
  - **C** toca o bip; aparece uma linha brilhante em ~2,7 kHz.
- A página BOLA IMU usa a aceleração do MPU6886 como gravidade:
  - a bola quica nas bordas do campo com restituição configurável.
  - **C** alterna 5 presets de gravidade, quique e raio.
  - O mapeamento dos eixos é fixo (sem opções invertidas).
  - Roda a ~60 fps: só a região da bola (dirty rect) é enviada ao display
    a cada frame; o fundo/campo/rodapé são desenhados uma vez.
- A página AREIA usa um autômato em grid 240×84:
  - **C** alterna entre 1.000 e 10.000 grãos; cada troca reinicia a areia.
  - Os grãos seguem a gravidade do IMU em 8 direções (incluindo diagonais),
    com deslizamento quando a diagonal está bloqueada.
  - O spawn vem de uma faixa central no lado oposto à gravidade (tipo moega).
  - 3 tons de areia, deslizamento com atrito (ângulo de repouso) e fluxo mais
    lento em inclinações suaves.
  - Só o campo de areia é enviado ao display por frame (40 KB), com SPI a 40 MHz.
  - O grid ocupa ~20 KB de RAM.
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

## ENV IV no Grove

O ENV IV usa **SHT40 (0x44)** e **BMP280 (0x76)** no I2C externo:

- Grove: SDA = G32, SCL = G33, `machine.I2C1`.
- Página **ENV IV** no `poc/sensors` e POC serial `poc/env-iv`.
- Mostra temperatura, umidade, pressão e altitude.

## Fingerprint2 no Grove

O Unit Fingerprint2 (A-K323CP) usa UART 115200 8N1 no Grove:

- TX = G32, RX = G33 (`machine.UART1`).
- O módulo dorme e acorda com **toque no sensor**; a tela mostra “TOQUE NO SENSOR”.
- `poc/finger2` mostra firmware, capacidade, digitais cadastradas, ID/score e
  controla o LED ring e o buzzer.
- Ainda não há cadastro de digitais pela POC; sem template cadastrado a busca
  retorna “NAO CADASTRADO”.

**Atenção:** ENV IV e Fingerprint2 compartilham o Grove (G32/G33). Use um de
cada vez; não dá para rodar as duas páginas/POCs juntas.

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
