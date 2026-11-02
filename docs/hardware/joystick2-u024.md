> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Unit Joystick2 U024-V2

Tela de teste do **Unit Joystick2** (SKU U024-V2, joystick Hall + STM32G030):

- Grove **I2C1: SDA=G32, SCL=G33**, endereço **0x63**, 400 kHz. Um unit por
  vez no Grove (compartilhado com Encoder/GPS/Fingerprint2/ToF).
- Driver puro Go separado:
  [M5Unit-Joystick2-Go](https://github.com/nicolasmmb/M5Unit-Joystick2-Go)
  v0.2.0 — **2 transações I2C por amostra**, 0 alocações.
- Eixos: ADC 16 bits (0–65535), posição ±4095, ADC/posição 8 bits e botão Z.
- Páginas (**C** avança): VETOR (mira + X/Y + botão + caixa da deadzone),
  BRUTO (ADC 16/8 e posição 8 bits), **DEADZONE** (ajuste) e INFO (fw,
  bootloader, endereço, LED, calibração).
- Fora da página DEADZONE: **A** cicla a cor do LED (8 presets) e **B** o
  brilho (25/50/100%).
- Na página **DEADZONE**: **A** diminui e **B** aumenta 128 unidades (3,1%)
  por toque, aplicados aos dois eixos; **C** grava na flash, volta para VETOR
  e mostra `DEADZONE SALVA`; **segurar o botão do joystick por 2 s** restaura
  a calibração lida no boot. A caixa cinza na mira mostra a área morta
  aplicada.
- **I2C a 100 kHz**: a 400 kHz o slave do STM32 dá timeout em polling contínuo;
  a 100 kHz rodou a 35–43 Hz sem erros.
- Validado no hardware: X/Y chegam a **±4095**, botão Z registra, ADC de 16 bits
  cobre a faixa e os presets de cor/brilho do LED funcionam.
- **Y do módulo é positivo para baixo** (stick para cima → `-4095`, igual à lib
  oficial). O POC aplica `Orientation{InvertY: true}` para a mira subir junto
  com o stick; a lib não altera o sinal por conta própria.
- A gravação da deadzone é **persistente na flash do STM32**. O POC guarda a
  calibração de fábrica em RAM no boot e a restaura segurando o botão do
  joystick por 2 s; após um power-cycle esse backup se perde (anote os valores
  na página INFO se for experimentar bastante). A lib também expõe
  `SetCalibration` para escrita direta.
