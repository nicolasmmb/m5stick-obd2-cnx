> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Fingerprint2 A-K323CP

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

### Tela (`../../../m5stick-hardware-pocs/units/finger2`)

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

### POC de testes (`../../../m5stick-hardware-pocs/units/finger2-test`)

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

POC de teste (no repositório externo):

```text
../../../m5stick-hardware-pocs/units/finger2-test
```

O target antigo `make finger-test` não existe mais neste repo focado em OBD2.

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

Benchmark em `../../../m5stick-hardware-pocs/units/finger2-bench` com `runtime.ReadMemStats` e `time.Since`:

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

Redesenho (`../../../m5stick-hardware-pocs/units/finger2-render-bench`):

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
