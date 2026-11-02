# Buzzer — GPIO2

> Nota técnica preservada do README antigo. O POC que usa este sensor está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Como funciona

O buzzer passivo do M5StickC Plus2 fica em **GPIO2**. O POC gera um tom de
**2700 Hz** usando **PWM (LEDC)** do ESP32.

O buzzer também é usado como referência acústica para validar o microfone:
quando o tom toca, o espectro deve mostrar um pico em ~2,7 kHz.

## Cuidados

- O driver bit-banged não é confiável com o scheduler do ESP32; use PWM/LEDC.
- O volume é fixo; não há controle analógico.
- Não use o buzzer em frequências muito baixas/altas fora da faixa ressonante.
