> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Unit Encoder U135

Tela de teste do encoder rotativo M5Stack (**SKU U135**, STM32F030F4P6):

- I2C **0x40**, Grove I2C1 (**SDA=G32, SCL=G33**, 100 kHz).
- Driver puro Go separado:
  [M5Unit-Encoder-Go](https://github.com/nicolasmmb/M5Unit-Encoder-Go) v0.1.0.
- Registradores: `0x00` modo, `0x10` contagem `int16` LE, `0x20` botão,
  `0x30` LED RGB `[index, R, G, B]` (2× SK6812).
- Tela: contagem, direção/delta, botão e LEDs; **A** alterna o LED,
  **B** a cor, **C** zera a contagem.
- **Grove compartilhado** com ENV IV e Fingerprint2: use apenas um por vez.
