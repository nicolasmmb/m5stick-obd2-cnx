# IMU MPU6886

> Nota técnica preservada do README antigo. O POC que usa este sensor está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Como funciona

O MPU6886 é uma IMU de 6 eixos com acelerômetro e giroscópio de 3 eixos, além
de sensor de temperatura. No M5StickC Plus2 ele fica no barramento **I2C0**:

- SDA = **G21**
- SCL = **G22**
- Endereço I2C = **0x68**

A aplicação lê registradores de dados do sensor, converte os valores brutos
para:

- aceleração em **g**;
- velocidade angular em **°/s**;
- temperatura em **°C**.

## O que o POC faz

A página **IMU MPU6886** do POC de sensores mostra:

- aceleração X/Y/Z;
- giroscópio X/Y/Z;
- temperatura.

A página **BOLA IMU** usa a aceleração como gravidade: a bola cai e quica nas
bordas do campo. O mapeamento dos eixos é fixo, sem inversão configurável.

## Cuidados

- O sensor é I2C; um timeout de barramento pode travar a leitura.
- A orientação física do Stick muda o sinal dos eixos.
- Para usar como inclinômetro, calibre o offset de repouso.
