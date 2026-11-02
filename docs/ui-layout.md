# Layout da tela — `obd-logger`

Tela 240×135, landscape. A UI usa framebuffer 4bpp e flush de linhas alteradas.

## Grade calculada

```text
availW = screenW - 2*margin - (cols-1)*gap
availH = screenH - 2*margin - (rows-1)*gap
side   = min(availW/cols, availH/rows)
gridW  = cols*side + (cols-1)*gap
gridH  = rows*side + (rows-1)*gap
x0     = (screenW - gridW)/2
y0     = (screenH - gridH)/2
```

Parâmetros atuais:

```text
margin = 2
gap    = 2
```

## Página 1 — STATUS

Grade **3×2 = 6 quadrados iguais**.

```text
┌────────┬────────┬────────┐
│  BLE   │  ELM   │ PID/s  │
├────────┼────────┼────────┤
│  TX    │  ERR   │ UPTIME │
└────────┴────────┴────────┘
```

| Tile | Valor |
|---|---|
| BLE | ON/OFF |
| ELM | OK/ERR |
| PID/s | taxa de amostras |
| TX | linhas enviadas |
| ERR | erros/timeouts |
| UPTIME | hh:mm |

Cores: verde = OK, vermelho = erro, cinza = inativo.

## Página 2 — DADOS

Grade **3×1**. Os quadrados ficam maiores e centralizados.

```text
┌──────────┬──────────┬──────────┐
│   VEL    │   RPM    │  COMB    │
│    0     │  1120    │   25.0   │
│   km/h   │   rpm    │    L     │
└──────────┴──────────┴──────────┘
```

- velocidade em km/h;
- RPM;
- combustível em litros (`% * 44 L`).

O valor usa `Bold18` ou `Bold12` se não couber. O texto é centralizado com
`tinyfont.LineWidth`.

## Botões

- **A** = próxima página.
- **B** = página anterior.
- **C** não é usado (evita desligar o Stick).

## Page indicator

Dois pontinhos no rodapé; o ativo fica branco.

## Atualização

- página redesenhada a cada ~500 ms;
- troca de página imediata;
- framebuffer evita o “apaga e desenha” visível.
