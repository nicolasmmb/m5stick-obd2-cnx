> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Microfone SPM1423 (`../../../m5stick-hardware-pocs/lib/spm1423`)

Driver PDM para o SPM1423 do M5StickC Plus2. O TinyGo não implementa
`machine.I2S` no ESP32 clássico, então a lib configura o periférico **I2S0**
diretamente e usa o conversor **PDM→PCM em hardware**.

Uso:

```go
mic := spm1423.New(machine.GPIO0, machine.GPIO34)
err := mic.Configure(spm1423.Config{BufferSize: 512})
mic.Start()
defer mic.Stop()

buf := make([]int16, 512)
n, err := mic.Read(buf)
rms := spm1423.RMS(buf[:n])
dbfs := spm1423.DBFS(rms)
```

Detalhes:

- Clock PDM em **G0** (`I2S0I_WS_OUT`), dados em **G34** (`I2S0I_DATA_IN15`).
- MCLK 8 MHz (160 MHz / 20), BCLK 1 MHz, amostragem ~**15,625 kHz**.
- `RXEOF_NUM` configurado para o tamanho do buffer; DMA com descritor de 12 bytes.
- `RMS` remove o offset DC; `DBFS` é relativo ao fundo de escala de 16 bits.
- `Read` bloqueia até o buffer DMA encher (512 amostras ≈ 33 ms).

Validação real com um tom de **2,7 kHz** no buzzer:

```
silêncio:  rms ~16   dbfs -66
bip:       rms ~6400 dbfs -14
```

Cuidados:

- O valor em dBFS **não é SPL calibrado**; para SPL de verdade é preciso
  calibrar com um medidor de referência.
- O microfone PDM tem offset DC; use sempre `RMS`/`DBFS` em vez do valor bruto.
- A taxa é nominal de 15,625 kHz (divisor inteiro). Para 16 kHz exatos seria
  preciso usar divisor fracionário no `CLKM_CONF`.
