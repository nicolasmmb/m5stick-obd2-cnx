# Notas técnicas de hardware

Notas preservadas do README antigo, separadas por sensor/unit. Os POCs
correspondentes foram movidos para `../../../m5stick-hardware-pocs/` para manter este
repositório focado em OBD2.

## Índice

| Arquivo | Hardware | Barramento / endereço |
|---|---|---|
| [display-st7789.md](display-st7789.md) | LCD ST7789V2 | SPI2, G13/G15/G14/G12/G5/G27 |
| [imu-mpu6886.md](imu-mpu6886.md) | IMU MPU6886 | I2C0 G21/G22, `0x68` |
| [rtc-bm8563.md](rtc-bm8563.md) | RTC BM8563 | I2C0 G21/G22, `0x51` |
| [battery-adc.md](battery-adc.md) | Bateria Li-ion | ADC G38, divisor 2,0 |
| [buttons.md](buttons.md) | Botões A/B/C | G37/G39/G35 |
| [buzzer.md](buzzer.md) | Buzzer passivo | PWM G2 |
| [led-ir.md](led-ir.md) | LED vermelho/IR | G19 |
| [microphone-spm1423.md](microphone-spm1423.md) | Microfone SPM1423 | PDM I2S0 G0/G34 |
| [env-iv-sht40-bmp280.md](env-iv-sht40-bmp280.md) | ENV IV | I2C1 G32/G33, `0x44`/`0x76` |
| [fingerprint2-a-k323cp.md](fingerprint2-a-k323cp.md) | Fingerprint2 | UART1 G32/G33, 115200 |
| [encoder-u135.md](encoder-u135.md) | Encoder U135 | I2C1 G32/G33, `0x40` |
| [gps-at6668.md](gps-at6668.md) | GPS v1.1 | UART1 G32/G33, 115200 |
| [tof90-vl53l0c.md](tof90-vl53l0c.md) | ToF-90 | I2C1 G32/G33, `0x29` |
| [joystick2-u024.md](joystick2-u024.md) | Joystick2 | I2C1 G32/G33, `0x63` |
| [nfc-st25r3916.md](nfc-st25r3916.md) | NFC U216 | I2C1 G32/G33, `0x50` |
| [demos-sensores.md](demos-sensores.md) | Demos do POC de sensores | ESPECTRO, BOLA IMU, AREIA |

## Regra do Grove

O Grove do StickC Plus2 é compartilhado: **um unit por vez**.
I2C usa `machine.I2C1` (SDA=G32/SCL=G33); UART usa `machine.UART1`
(TX=G32/RX=G33).

## POCs

Os POCs de hardware estão em:

```text
../../../m5stick-hardware-pocs/board/
../../../m5stick-hardware-pocs/units/
../../../m5stick-hardware-pocs/lib/spm1423/
```
