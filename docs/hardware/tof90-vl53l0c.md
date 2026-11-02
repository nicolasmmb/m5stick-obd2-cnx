> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Unit Mini ToF-90° (VL53L0C)

Tela de teste do ToF M5Stack **Mini ToF-90°** (SKU U196, **VL53L0CXV0DH**,
mesmos registradores do VL53L0X):

- Grove **I2C1: SDA=G32, SCL=G33**, endereço **0x29**, 400 kHz. O Grove é
  compartilhado com Encoder/GPS/Fingerprint2: use apenas um por vez.
- Driver puro Go separado:
  [M5Unit-ToF90-Go](https://github.com/nicolasmmb/M5Unit-ToF90-Go) v0.3.0 —
  **0 alocações por operação**, 3 transações I2C por amostra em modo contínuo.
- Alcance 3–200 cm (200 cm só no modo **LONG**, com ambiente escuro), 1 mm de
  resolução, FoV 25° e emissor rotacionado 90°.
- Páginas (**B** avança): MEDIDA (valor grande, taxa, mín/máx, status),
  MODO (budget e signal rate) e INFO (modelo, revisão, amostras/erros).
- **A** alterna mm ↔ cm; **C** cicla 20/33/50/100/200 ms e **LONG**
  (200 ms + 0.1 MCPS).
- Os contadores de amostras são atualizados no log serial a 1 Hz; a tela
  redesenha no máximo a 10 Hz.
- Validado no hardware: **33 ms ≈ 35 Hz**; **LONG (200 ms) ≈ 5 Hz**, 0% de
  leituras inválidas a ~1,24 m; o status **11 (MERGED OK)** é uma leitura
  válida. Trocar o budget em modo contínuo exige **parar e reiniciar** o
  stream (o sensor trava a configuração no start) — o botão C já faz isso.
