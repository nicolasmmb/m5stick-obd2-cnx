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
| B10 | Fingerprint2 (A-K323CP) no Grove | ✅ abas, cadastro AutoEnroll, excluir/limpar, imagem, LED ring |
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
poc/encoder/      # tela de teste do Unit Encoder U135 (I2C Grove)
poc/gps/          # tela de teste do Unit GPS v1.1 (UART Grove)
poc/tof/          # tela de teste do Unit Mini ToF-90 (I2C Grove)
poc/joystick2/    # tela de teste do Unit Joystick2 Hall (I2C Grove)
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
make encoder     # flash + monitor (tela do Unit Encoder U135)
make gps         # flash + monitor (tela do Unit GPS v1.1)
make tof         # flash + monitor (tela do Unit Mini ToF-90)
make joystick2   # flash + monitor (tela do Unit Joystick2)
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
| finger2 | 115 KB | 15,1 KB |

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

O Unit Fingerprint2 (**A-K323CP**, SKU U203) é controlado por uma lib 100% Go
no módulo separado [M5Unit-Fingerprint2-Go](https://github.com/nicolasmmb/M5Unit-Fingerprint2-Go), sem C/C++/CGo, Arduino ou ESP-IDF.

### Hardware e ligação

- UART **115200 8N1**, `machine.UART1`.
- TX = **G32**, RX = **G33** (Grove).
- IMU/tela/botões/buzzer/LED do stick não são afetados.
- O módulo dorme; `Init()` faz `Handshake`, `Activate`, lê firmware/parâmetros e
  grava `SetWorkMode(1)` para ficar acordado enquanto a tela está ligada.
- **ENV IV e Fingerprint2 compartilham o Grove (G32/G33).** Use um de cada vez.

### Protocolo

Pacotes `EF 01 | addr(4) | PID | len(2) | payload | checksum(2)`:

| PID | Tipo | Payload |
|---|---|---|
| `0x01` | comando | `cmd` + parâmetros |
| `0x07` | ACK | **confirm** + dados |
| `0x02` | dados | **somente dados** |
| `0x08` | fim dos dados | **somente dados** |

> **Detalhe importante (já causou bug real):** só o ACK começa com o byte de
> confirmação. Nos pacotes de dados o payload começa direto. Ler pacotes de
> dados com o mesmo helper do ACK descarta **1 byte por pacote** — com os ~65
> pacotes da imagem isso perdia 65 bytes (8320 → 8255) e produzia uma **linha
> diagonal fixa** na imagem por deslocamento acumulado. `readData()` lê
> `pkt[9:len-2]`; `ackData()` (só ACKs) lê `confirm=pkt[9]` e dados de `pkt[10:]`.

### Cadastro (AutoEnroll)

O cadastro usa o state machine do próprio módulo, comando `0x31`:

```go
dev.StartAutoEnroll(page, 2, finger2.AutoEnrollDefault)
for {
    ev, err := dev.NextEnrollEvent(200 * time.Millisecond)
    // ev.Confirm, ev.Stage, ev.Param
    if ev.Stage == finger2.EnrollStageStore { break }
}
```

Etapas reportadas em `Stage` (Param1):

| Valor | Etapa | Tela |
|---|---|---|
| `0x00` | legal check | VALIDANDO |
| `0x01` | get image | ENCOSTE O DEDO / CAPTURANDO |
| `0x02` | gen char | GERANDO |
| `0x03` | lift finger | LEVANTE O DEDO |
| `0x04` | merge | MESCLANDO |
| `0x05` | verify | VERIFICANDO |
| `0x06` | store | CADASTRADO |

`confirm == 0x02` (sem dedo) **não** é erro: é só espera. Ao final, o módulo
grava o template e a página livre vem de `FirstFreePage()` (índice de templates).

O cadastro manual antigo (2× `Capture` + `GenChar` + `RegModel`) foi abandonado:
a segunda captura falhava com frequência e o `RegModel` às vezes gravava um
template ruim. O AutoEnroll valida a consistência internamente.

### Leitura (busca)

`PS_Search (0x04)` compara o **char buffer**, não a imagem. A sequência correta é:

```
Capture (0x01)  →  GenChar(1) (0x02)  →  Search(buffer 1) (0x04)
```

Sem o `GenChar(1)`, o `Search` reutiliza o buffer da última leitura/cadastro e
**qualquer dedo casa** com score ~446 (era o falso positivo que aparecia). Com o
fluxo correto, dedo cadastrado dá score ~85 e dedo diferente retorna
`confirm 0x09` (não encontrado).

O critério de match é o **código de confirmação**, não o score. O score é só
confiança. `SetMatchLevel(5)` grava o registrador 5 no `Init()`.

### Imagem do sensor

- Resolução **80×208**, 4 bits/pixel (2 pixels por byte), 508 dpi.
- `UpImage (0x0A)` envia ~8320 bytes em pacotes de dados + pacote de fim.
- Nibble **baixo** = pixel par, **nibble alto** = pixel ímpar; `0` = preto,
  `15` = branco (convertido para `v*17`).
- A POC deixa o dedo assentar **350 ms** e captura de novo antes do upload —
  a primeira frame do toque costuma sair borrada.
- Renderização: rotaciona 90° (fica 208×80) e centraliza na tela; usa o número
  real de linhas recebidas.

### Tela (`poc/finger2`)

Duas linhas de abas com nomes completos:

```
LEITURA | IMAGEM | CADASTRO
EXCLUIR | LIMPAR | INFO
```

- **A/B** percorre as abas; **C** executa a ação (e confirma/cancela).
- **LEITURA**: `Capture` → `GenChar(1)` → `Search`; mostra ID e confiança.
- **IMAGEM**: captura e desenha a digital na tela; **C** faz nova captura.
- **CADASTRO**: AutoEnroll com 2 leituras.
- **EXCLUIR**: identifica o dedo e pede confirmação antes de apagar.
- **LIMPAR**: apaga todos os templates, com confirmação.
- **INFO**: firmware/SN, memória/MatchLevel, work mode/sleep, último resultado.
- LED ring e buzzer acompanham o estado (verde ok, vermelho falha, amarelo lido).

### POC de testes (`poc/finger2-test`)

Firmware separado para validar os comandos que a tela principal não usa. Menu na
tela (**A/B** escolhe, **C** roda), resultado `PASS`/`FAIL` na tela e na serial:

```
TEST|07|WorkMode|PASS|orig=1 lido=1
```

| # | Teste | O que valida |
|---|---|---|
| 01 | GetSN | leitura do serial number (32 bytes) |
| 02 | GetRandom | duas leituras e valores diferentes |
| 03 | CheckSensor | auto-teste do sensor |
| 04 | ReadINF | página INF (512 bytes) |
| 05 | Notepad | escreve 32 bytes, lê e compara |
| 06 | LED modes | blink vermelho + fade verde/ciano (visual) |
| 07 | WorkMode | `Get` → `Set(1)` → `Get` |
| 08 | SleepTime | `Get` → `Set(30)` → `Get` → restaura |
| 09 | SaveConfig | grava configuração no flash |
| 10 | LoadChar | carrega template da página 0 no buffer 1 |
| 11 | Template R/W | `LoadChar` → `UploadTemplateAuto` → `DownloadTemplateAuto` → `Store` → `Delete` |
| 12 | SearchNow | busca com o dedo (precisa de template) |
| 13 | AutoIdentify | identificação contínua até a etapa VERIFY |
| 14 | FW version | leitura do firmware por `0xD7` |
| 15 | Wakeup | dorme (work mode 0, 10 s), espera o toque e captura o pacote de wakeup |

Comando:

```bash
make finger-test
```

Resultado da campanha neste módulo (A-K323CP):

| Teste | Resultado |
|---|---|
| GetSN | PASS — 32 bytes, mas **alinhado à direita**: os 24 primeiros são zero |
| GetRandom | PASS — dois valores distintos |
| CheckSensor | PASS |
| ReadINF | PASS — 512 bytes |
| Notepad | PASS — escrita/leitura conferem |
| LED modes | PASS — blink e fade (modos 1–6; modo 7 não existe) |
| WorkMode | PASS — ida e volta |
| SleepTime | PASS — set 30, lê 30, restaura |
| SaveConfig | PASS |
| LoadChar | PASS — página 0 |
| Template R/W | PASS — 7262 bytes, store + delete |
| SearchNow | PASS — ID 0, score ~317 |
| AutoIdentify | PASS — etapas 0→1→5, ID 0, score 166–352 |
| FW version | PASS — `0xD7` devolve 1 |
| Wakeup | PASS — pacote de wakeup recebido após dormir e tocar |

O `Template R/W` exercita também `UploadTemplate` e `DownloadTemplate` (as
versões `Auto` chamam as simples em loop).

### Paridade com a lib original

Todas as funções `PS_*` da lib original têm equivalente Go e foram validadas
neste hardware. As diferenças são de **API**, não de protocolo:

| Original | Go |
|---|---|
| `PS_GetImage` | `Capture` |
| `PS_DeletChar` | `Delete` |
| `PS_ValidTemplateNum` | `ValidTemplates` |
| `PS_GetChipSN` | `GetSN` |
| `PS_ReadINFpage` | `ReadINF` |
| `PS_ControlBLN` | `ControlLED` |
| `PS_GetImageInfo` | `ImageInfo` |
| `PS_UploadTemplateAuto` (callback de progresso) | `UploadTemplateAuto` (retorna os bytes) |
| `PS_AutoEnroll`/`PS_AutoIdentify` (bloqueantes) | eventos `NextEnrollEvent`/`NextIdentifyEvent` (não bloqueiam a tela) |
| `setWakeupCallback` | `SetWakeupHandler` + `WaitWakeup` |

Diferenças de comportamento conscientes:

- `Init()` grava **MatchLevel 5** e **WorkMode 1**; a lib original não força isso.
- A lib original **declara** `0xD0`–`0xD7` (sleep/work mode/activate/status/
  save/firmware) mas **não implementa**; aqui estão implementadas e testadas.
- O callback de wakeup original depende de um parser em background. Aqui o
  pacote `EF 01 FF FF FF FF 07 00 03 FF 01 09` é reconhecido e descartado
  dentro das leituras (e `WaitWakeup` espera por ele), sem thread extra.
- Sem `ControlLEDRainbow` e sem `IAPReset` (não existem no protocolo).

Notas:

- Os testes 10–13 exigem pelo menos um template cadastrado (use a tela principal).
- O teste 11 cria uma página temporária e apaga no final.
- **Não existe `IAPReset`**: `0xFD` é o código de status `PARAM_ERROR` na lib
  original, não um comando. Foi removido da lib.
- **Não existe modo rainbow**: os modos de LED válidos são 1–6 (breathing,
  flashing, on, off, fade-in, fade-out). O modo 7 retorna `confirm 0x35`
  (parâmetro inválido) e foi removido da lib.
- `SetSleepTime`/`GetSleepTime`/`SetWorkMode`/`GetWorkMode`/`SaveConfig`
  (`0xD0`–`0xD6`) existem no protocolo, mas a lib original não os expõe; por isso
  não há referência de comportamento — o teste apenas verifica ida e volta.

### Performance (medido no hardware)

Benchmark em `poc/finger2-bench` com `runtime.ReadMemStats` e `time.Since`:

| Operação | Antes | Depois |
|---|---|---|
| `UpImageTo` (8320 B) | 525 mallocs, heap 131 KB | **0 mallocs**, heap 91 KB |
| `UpImage` (8320 B) | 526 mallocs, heap 147 KB | 1 malloc, heap 108 KB |
| Upload de template (7262 B) | 172 mallocs | **1 malloc** |
| Download de template (7262 B) | 146 mallocs | **0–1 malloc** |
| Leitura `Capture`+`GenChar`+`Search` (20×) | 20 mallocs | **0** |
| `ReadSysParams` (200×) | 1 cópia por chamada | **0** |
| Render 80×208 → 240×135 | 5,69 ms | 3,09 ms (paleta) |

- `UpImageTo`/`ReadINFTo` escrevem em buffer do chamador e `readPacket` usa um
  scratch persistente: caiu de 525 para 1 malloc por captura.
- `transceive` copia o payload do ACK para os chamadores não reterem uma visão
  do scratch; upload/download de template usam a variante interna sem cópia.
- O pacote de comando também é montado num buffer de TX reutilizado: comandos
  não alocam mais (era 1 malloc por comando).
- `transceive` não copia mais o ACK: quem devolve dados ao chamador copia
  explicitamente e os demais consomem o scratch antes do próximo comando.
- O **upload não acelera**: ~1,81 s por imagem, limitado pelo módulo/UART.
  Pacotes de 256 B (registrador 6) dão ~1,709 s (−5%), mas mudam a config do
  módulo; não adotado.
- A renderização usa paleta de 16 níveis em RGB565 em vez de converter cada
  pixel.
- A tela só é redesenhada quando o estado muda (`renderState`): sem transferir
  65 KB por SPI a cada 100 ms e sem redesenhar a imagem parada.
- O pacote `internal/wire` do módulo
  [M5Unit-Fingerprint2-Go](https://github.com/nicolasmmb/M5Unit-Fingerprint2-Go)
  isola framing/checksum/ACK/wakeup e tem testes de host (`make test` no repo da
  lib, sem hardware).

Redesenho (`poc/finger2-render-bench`):

| Cenário | Antes (10 Hz) | Depois |
|---|---|---|
| Tela ociosa | 210 ms/s (flush SPI 16,3 ms/frame) | ~0 |
| Aba IMAGEM | 270 ms/s | ~28 ms/s (1 redraw/captura) |
| Checagem de estado | — | 93 ns |
| Loop `min/max/avg` removido | 2,5 ms/captura | 0 |

### Armadilhas conhecidas

- `modeRead = iota` declarado no mesmo `const` de `screenW`/`screenH` vale **2**,
  não 0 — as abas ficam deslocadas e `modeCount` estoura o array. Mantenha os
  dois blocos `const` separados.
- `Search` sempre precisa de um `GenChar` antes.
- O `GetSN` deste módulo devolve 32 bytes com o serial no **fim** (zeros à
  esquerda); a tela INFO mostra os últimos 8 bytes não nulos.
- Apagar a base (`Empty`, `0x0D`) é obrigatório ao trocar de dedo de teste,
  senão o template antigo continua respondendo.
- O cache de build do TinyGo pode ficar velho após muitas edições; se o binário
  não mudar de tamanho, rode `tinygo clean` antes de gravar.

### API resumida (M5Unit-Fingerprint2-Go)

| Grupo | Funções |
|---|---|
| Sistema | `Init`, `Handshake`, `Activate`, `ModuleStatus`, `GetFirmwareVersion`, `ReadSysParams`, `SetSleepTime`, `GetSleepTime`, `SetWorkMode`, `GetWorkMode`, `SaveConfig`, `GetSN`, `CheckSensor`, `GetRandom`, `WaitWakeup`, `SetWakeupHandler` |
| Biometria | `Capture`, `GetEnrollImage`, `GenChar`, `Match`, `Search`, `SearchNow`, `RegModel`, `Store`, `LoadChar`, `Delete`, `Empty` |
| Cadastro auto | `StartAutoEnroll`, `NextEnrollEvent`, `AutoEnroll`, `AutoIdentify`, `Cancel` |
| Templates | `ValidTemplates`, `ReadIndexTable`, `FirstFreePage`, `UploadTemplate`, `UploadTemplateAuto`, `DownloadTemplate`, `DownloadTemplateAuto` |
| Imagem | `UpImage`, `ImageInfo` |
| Extras | `ReadINF`, `ReadNotepad`, `WriteNotepad`, `ControlLED`, `SetLED`, `WriteReg`, `SetMatchLevel` |

## Unit Encoder U135

Tela de teste do encoder rotativo M5Stack (**SKU U135**, STM32F030F4P6):

- I2C **0x40**, Grove I2C1 (**SDA=G32, SCL=G33**, 100 kHz).
- Driver puro Go separado:
  [M5Unit-Encoder-Go](https://github.com/nicolasmmb/M5Unit-Encoder-Go) v0.1.0.
- Registradores: `0x00` modo, `0x10` contagem `int16` LE, `0x20` botão,
  `0x30` LED RGB `[index, R, G, B]` (2× SK6812).
- Tela: contagem, direção/delta, botão e LEDs; **A** alterna o LED,
  **B** a cor, **C** zera a contagem.
- **Grove compartilhado** com ENV IV e Fingerprint2: use apenas um por vez.

## Unit GPS v1.1

Tela de teste do GPS M5Stack (**v1.1**, AT6668 + MAX2659):

- UART do Grove: **TX=G32, RX=G33**; **115200 8N1** (o U032 antigo usa 9600;
  a tela testa 115200 e cai para 9600 se não houver NMEA).
- Driver puro Go separado:
  [M5Unit-GPS-Go](https://github.com/nicolasmmb/M5Unit-GPS-Go) v0.3.1 —
  parser NMEA com **0 alocações por sentença**.
- NMEA: `GGA` (satélites, qualidade, altitude), `RMC` (UTC, lat/lon, velocidade,
  rumo), `GSV` (satélites em vista e melhor SNR, somados por constelação) e
  `TXT` (antena).
- Páginas (**A/B**, **C** avança): POSIÇÃO (lat/lon/alt, satélites em uso,
  satélites em vista e melhor SNR), TEMPO (UTC, velocidade, rumo), STATUS
  (antena, qualidade, baud, sentenças/checksums e taxa de perda).
- Otimização medida (60 s no hardware): o estado incluía UTC e velocidade, que
  mudam a cada segundo, forçando **58 redraws** e perda de sentenças por
  overflow do ring UART de 128 B. Agora só entra no estado o que a página
  mostra, o SNR é quantizado em 5 dB e o SPI subiu para 80 MHz: **13 redraws**
  e **0% de checksum errors em regime** (5 no boot, durante a troca de telas).
- `ANTENNA OPEN` é reportado pelo módulo mesmo com a cerâmica embutida em uso;
  com vista para o céu o fix é obtido normalmente.
- Grove compartilhado com Fingerprint2/Encoder/ENV IV: um por vez.

## Unit Mini ToF-90°

Tela de teste do ToF M5Stack **Mini ToF-90°** (SKU U196, **VL53L0CXV0DH**,
mesmos registradores do VL53L0X):

- Grove **I2C1: SDA=G32, SCL=G33**, endereço **0x29**, 400 kHz. O Grove é
  compartilhado com Encoder/GPS/Fingerprint2: use apenas um por vez.
- Driver puro Go separado:
  [M5Unit-ToF90-Go](https://github.com/nicolasmmb/M5Unit-ToF90-Go) v0.3.0 —
  **0 alocações por operação**, 3 transações I2C por amostra em modo contínuo.
- Alcance 3–200 cm (200 cm só no modo **LONG**, com ambiente escuro), 1 mm de
  resolução, FoV 25° e emissor rotacionado 90°.
- Páginas (**B** avança): MEDIDA (valor grande, taxa, mín/máx, status),
  MODO (budget e signal rate) e INFO (modelo, revisão, amostras/erros).
- **A** alterna mm ↔ cm; **C** cicla 20/33/50/100/200 ms e **LONG**
  (200 ms + 0.1 MCPS).
- Os contadores de amostras são atualizados no log serial a 1 Hz; a tela
  redesenha no máximo a 10 Hz.
- Validado no hardware: **33 ms ≈ 35 Hz**; **LONG (200 ms) ≈ 5 Hz**, 0% de
  leituras inválidas a ~1,24 m; o status **11 (MERGED OK)** é uma leitura
  válida. Trocar o budget em modo contínuo exige **parar e reiniciar** o
  stream (o sensor trava a configuração no start) — o botão C já faz isso.

## Unit Joystick2 (Hall)

Tela de teste do **Unit Joystick2** (SKU U024-V2, joystick Hall + STM32G030):

- Grove **I2C1: SDA=G32, SCL=G33**, endereço **0x63**, 400 kHz. Um unit por
  vez no Grove (compartilhado com Encoder/GPS/Fingerprint2/ToF).
- Driver puro Go separado:
  [M5Unit-Joystick2-Go](https://github.com/nicolasmmb/M5Unit-Joystick2-Go)
  v0.1.1 — **2 transações I2C por amostra**, 0 alocações.
- Eixos: ADC 16 bits (0–65535), posição ±4095, ADC/posição 8 bits e botão Z.
- Todas as páginas (**C** avança): VETOR (mira + X/Y + botão), BRUTO (ADC 16/8
  e posição 8 bits) e INFO (fw, bootloader, endereço, LED, calibração).
- **A** cicla a cor do LED (8 presets), **B** o brilho (25/50/100%).
- **I2C a 100 kHz**: a 400 kHz o slave do STM32 dá timeout em polling contínuo;
  a 100 kHz rodou a 35–43 Hz sem erros.
- Validado no hardware: X/Y chegam a **±4095**, botão Z registra, ADC de 16 bits
  cobre a faixa e os presets de cor/brilho do LED funcionam.
- **Y do módulo é positivo para baixo** (stick para cima → `-4095`, igual à lib
  oficial). O POC aplica `Orientation{InvertY: true}` para a mira subir junto
  com o stick; a lib não altera o sinal por conta própria.
- A calibração/deadzone é **somente leitura** no POC (escritas vão para a
  flash do STM32; a lib expõe `SetCalibration`/`SetDeadzone*`).

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
make check     # roda gopls check em todos os POCs e libs
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
