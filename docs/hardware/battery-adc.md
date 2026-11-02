# Bateria — ADC GPIO38

> Nota técnica preservada do README antigo. O POC que usa este sensor está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Como funciona

A tensão da bateria Li-ion é medida pelo ADC do ESP32 no pino **GPIO38**. O
hardware usa um divisor de tensão de fator **2,0**, então a tensão real é
aproximadamente o dobro da leitura do ADC.

O POC mostra:

- tensão da bateria;
- porcentagem estimada;
- barra de nível.

## Cuidados

- O ADC do ESP32 exige `machine.InitADC()` antes do `Configure`.
- A leitura é ruidosa; o POC faz média/filtro.
- A porcentagem é uma estimativa por tensão, não um combustível medido.
- A curva da Li-ion não é linear; 3,7 V nominais não significam 50% exatos.
- Com o Stick alimentado por USB, a tensão pode ficar acima da faixa da bateria.
