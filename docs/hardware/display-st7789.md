> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Display ST7789V2

> POC de referência: `examples/display-scan/`.

## Hardware

- LCD ST7789V2 135×240.
- MOSI = **G15**, CLK = **G13**, DC = **G14**, RST = **G12**, CS = **G5**,
  BL = **G27**.
- Em landscape: 240×135, `Rotation90`, `RowOffset=40`, `ColumnOffset=52`.
- O painel usa ordem de cor **BGR**: `display.IsBGR(true)`.

## Notas

- LCD ST7789V2 em landscape 240x135, `Rotation90`, `RowOffset=40`,
  `ColumnOffset=52`, cor BGR.
- Abas: **WIFI** (ciano) e **BLE** (verde), com contador de cada uma.
- Lista com 9 linhas ordenadas por RSSI; a selecionada fica com fundo branco.
- Navegação:
  - **A** (G37, frontal/central): alterna a aba.
  - **B** (G39, lateral superior): sobe na lista.
  - **C** (G35, lateral/power): desce na lista.
- A lista rola sozinha quando a seleção passa da 9ª linha.
- Redesenho parcial: ao navegar só as linhas antiga/nova são atualizadas,
  sem `FillScreen` — evita o piscar da tela inteira.
- Ciclo de scan: Wi-Fi (~4 s) → BLE (8 s) → pausa de 5 s → repete.
