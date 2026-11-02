# RTC BM8563

> Nota técnica preservada do README antigo. O POC que usa este sensor está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Como funciona

O BM8563 é um relógio de tempo real (RTC) com bateria de backup. No
M5StickC Plus2 ele fica no **I2C0**:

- SDA = **G21**
- SCL = **G22**
- Endereço I2C = **0x51**

Ele mantém data, hora e dia da semana mesmo com o Stick desligado, desde que a
bateria de backup esteja presente.

## O que o POC faz

A página **RTC BM8563** mostra:

- data;
- hora;
- dia da semana;
- status do RTC.

A ação do botão **C** permite ajustar o RTC.

## Cuidados

- A leitura é feita por registradores BCD; é preciso converter para decimal.
- Sem bateria de backup, o RTC perde a hora ao remover a alimentação.
- O RTC é independente do `time.Now()` do TinyGo; os dois precisam ser
  sincronizados se a aplicação usar horário.
