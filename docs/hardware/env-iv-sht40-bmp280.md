> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# ENV IV — SHT40 + BMP280

O ENV IV usa **SHT40 (0x44)** e **BMP280 (0x76)** no I2C externo:

- Grove: SDA = G32, SCL = G33, `machine.I2C1`.
- Página **ENV IV** no `../../../m5stick-hardware-pocs/board/sensors` e POC serial `../../../m5stick-hardware-pocs/units/env-iv`.
- Mostra temperatura, umidade, pressão e altitude.
