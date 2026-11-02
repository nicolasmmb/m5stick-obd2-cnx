# Layout da tela — `obd-logger`

Tela 240×135, landscape. A UI usa framebuffer 4bpp e flush de linhas alteradas.

## Área útil

Para evitar corte de texto nas bordas:

```text
inset = 1 px em cada lado
conteúdo = 238 × 130
gap = 2 px
footer = 2 px para os indicadores de página
```

## Página 1 — STATUS

Ocupa a tela inteira em **3 colunas × 2 linhas**, com 6 tiles de status.

```text
┌────────┬────────┬────────┐
│  BLE   │  ELM   │ PID/s  │
├────────┼────────┼────────┤
│  TX    │  ERR   │ UPTIME │
└────────┴────────┴────────┘
```

| Tile | Ícone | Valor |
|---|---|---|
| BLE | barras de sinal | ON/OFF |
| ELM | elo/conector | OK/ERR |
| PID/s | barras | taxa |
| TX | seta | linhas enviadas |
| ERR | exclamação | erros |
| UPTIME | relógio | hh:mm |

Cores:

- verde = ok;
- vermelho = erro;
- ciano/azul = informação;
- âmbar = transmissão;
- cinza = inativo.

## Página 2 — DADOS

Layout assimétrico para usar a tela inteira:

```text
┌──────────────────┬──────────────────┐
│    VELOCIDADE    │       RPM        │
│        0         │      1120        │
│       km/h       │       rpm        │
├──────────────────┴──────────────────┤
│ COMBUSTIVEL        25.0 L      58%  │
│ ███████████████░░░░░░░░░░░░░░░░░░░  │
└─────────────────────────────────────┘
```

- velocidade e RPM em dois tiles grandes;
- combustível em um tile largo embaixo;
- barra de combustível usa a largura disponível;
- ícones de velocímetro e bomba.

## Tipografia

- valores grandes: `Bold18`;
- se não couber: `Bold12`;
- labels/unidades: `Proggy Tiny`.
- Todo texto é centralizado com `tinyfont.LineWidth`.

## Botões

- **A** = próxima página.
- **B** = página anterior.
- **C** não é usado para evitar desligar o Stick.

## Atualização

- página redesenhada a cada ~500 ms;
- troca de página imediata;
- framebuffer evita flicker/ghosting.
