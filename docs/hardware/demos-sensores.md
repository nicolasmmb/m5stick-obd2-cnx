# Demos do POC de sensores

> Notas preservadas do README antigo. O POC está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Menu

O POC tem menu interativo de 7 páginas, renderizado em framebuffer (sem piscar):

| Página | O que mostra |
|---|---|
| IMU MPU6886 | aceleração X/Y/Z, giroscópio X/Y/Z e temperatura |
| RTC BM8563 | data, hora, dia da semana e status |
| BATERIA | tensão, porcentagem e barra (ADC GPIO38) |
| BOTOES | estado e contador de A/B/C |
| BUZZER | tom de 2700 Hz em G2 |
| LED / IR | estado do LED vermelho/IR em G19 |
| MICROFONE | SPM1423 PDM ao vivo: RMS, dBFS e barra |
| ENV IV | SHT40 (temp/umidade) + BMP280 (pressão/altitude) |
| FINGER2 | tela do Fingerprint2 |
| ESPECTRO | FFT 256 + waterfall 0–7,8 kHz |
| BOLA IMU | bola com gravidade do IMU |
| AREIA | simulador de areia com 1.000/10.000 grãos |

Navegação: **A** próxima, **B** anterior, **C** ação da página.

## Notas gerais

- O ADC do ESP32 exige `machine.InitADC()` antes do `Configure`.
- O framebuffer RGB565 de 240x135 usa ~65 KB de RAM.
- O buzzer usa PWM (LEDC) em G2; o driver bit-banged não é confiável com o
  scheduler do ESP32.
- O microfone usa a lib `../../../m5stick-hardware-pocs/lib/spm1423/` (I2S0 + DMA).
  Na página MICROFONE, **C** toca um bip de 2,7 kHz para validar o nível.

## Espectro

- FFT de 256 pontos com janela Hann.
- Waterfall de 120 colunas × 50 linhas, atualização ~10 Hz.
- dBFS normalizado por 32768 e offset DC removido: silêncio fica **preto**.
- Cores verde → amarelo → vermelho conforme o nível.
- Mostra o pico em kHz e dBFS no topo.
- **C** toca o bip; aparece uma linha brilhante em ~2,7 kHz.

## Bola IMU

- Usa a aceleração do MPU6886 como gravidade.
- A bola quica nas bordas com restituição configurável.
- **C** alterna 5 presets de gravidade, quique e raio.
- Mapeamento dos eixos fixo, sem opções invertidas.
- Roda a ~60 fps: só a região da bola (dirty rect) é enviada ao display; fundo,
  campo e rodapé são desenhados uma vez.

## Areia

- Autômato em grid 240×84.
- **C** alterna 1.000/10.000 grãos; cada troca reinicia.
- Grãos seguem a gravidade do IMU em 8 direções (incluindo diagonais), com
  deslizamento quando a diagonal está bloqueada.
- Spawn em uma faixa central no lado oposto à gravidade (tipo moega).
- 3 tons de areia, deslizamento com atrito (ângulo de repouso) e fluxo mais
  lento em inclinações suaves.
- Só o campo de areia é enviado por frame (40 KB), com SPI a 40 MHz.
- O grid ocupa ~20 KB de RAM.
