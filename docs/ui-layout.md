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
│ SERIAL │  BLE   │  ELM   │
├────────┼────────┼────────┤
│ PID/s  │  TX    │  ERR   │
└────────┴────────┴────────┘
```

| Tile | Ícone | Valor |
|---|---|---|
| SERIAL | seta | WAIT / OK / DISC |
| BLE | barras de sinal | OFF / SCAN / LINK / ON / ERR |
| ELM | elo/conector | WAIT / INIT / OK / ERR |
| PID/s | barras | taxa |
| TX | seta | linhas enviadas |
| ERR | exclamação | erros |

> `SERIAL` usa ACK do host: o leitor envia `A\n` a cada 1 s.
> `WAIT` (laranja) = ainda sem ACK; `OK` (verde) = host respondendo;
> `DISC` (vermelho) = ACK parou por mais de 3 s.

- `BLE=SCAN` (laranja) enquanto procura o adaptador;
- `BLE=LINK` (ciano) durante a conexão;
- `BLE=ON` (verde) conectado;
- `ELM=WAIT` (cinza) quando o BLE ainda não conectou — não é erro;
- `ELM=INIT` (laranja) durante o handshake;
- `ELM=OK` (verde) handshake concluído;
- `ELM=ERR` (vermelho) só quando o BLE conectou e o ELM falhou.

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

## Página 3 — TEMPERATURAS

Layout assimétrico, igual ao da página 2:

```text
┌──────────────────┬──────────────────┐
│      AGUA        │      AR ADM      │
│      75 °C       │      43 °C       │
│   ███████░░░░    │   █████░░░░░░    │
├──────────────────┴──────────────────┤
│           CATALISADOR               │
│              506 °C                 │
│   ████████████████░░░░░░░░░░░░░     │
└──────────────────────────────────────┘
```

| Dado | PID | Faixas |
|---|---|---|
| Água | `0105` | verde <100 °C, amarelo 100–110 °C, vermelho >110 °C |
| Ar admitido | `010F` | verde <50 °C, amarelo 50–70 °C, vermelho >70 °C |
| Catalisador | `013C` | verde <700 °C, amarelo 700–900 °C, vermelho >900 °C |

Cada tile tem:

- ícone;
- label;
- valor em °C;
- barra de faixa na base.

## Página 4 — MOTOR

Layout assimétrico:

```text
┌──────────────────┬──────────────────┐
│      CARGA       │    BORBOLETA     │
│       31%        │       22%        │
├──────────────────┴──────────────────┤
│              LAMBDA                 │
│               0.85                  │
└──────────────────────────────────────┘
```

| Dado | PID | Faixas |
|---|---|---|
| Carga do motor | `0104` | verde <60%, amarelo 60–85%, vermelho >85% |
| Borboleta | `0111` | verde <50%, amarelo 50–80%, vermelho >80% |
| Lambda | `0144` | verde 0,95–1,05; amarelo 0,90–1,10; vermelho fora |

O lambda tem uma marca branca no centro (λ=1,00).

## Página 5 — DIAGNÓSTICO

```text
┌──────────────────┬──────────────────┐
│      TENSAO      │       MIL        │
│      12.5 V      │       OFF        │
├──────────────────┴──────────────────┤
│              DTCS                   │
│                0                    │
└──────────────────────────────────────┘
```

| Dado | Fonte |
|---|---|
| Tensão do módulo | `0142` |
| MIL/check engine | `0101` bit 7 |
| DTCs armazenados | Mode `03` |

Cores:

- tensão <11,8 V = vermelho; 11,8–12,2 V = amarelo; acima = ciano;
- MIL ON = vermelho; OFF = verde;
- DTCs >0 = vermelho; 0 = verde.

## Página 6 — VIAGEM

```text
┌──────────────────┬──────────────────┐
│    DISTANCIA     │   COMBUSTIVEL    │
│      12.40 km    │      1.25 L      │
├──────────────────┴──────────────────┤
│              CONSUMO                │
│               9.9 km/L              │
└──────────────────────────────────────┘
```

Dados derivados:

- distância integrada da velocidade (`010D`);
- combustível usado pela diferença do nível (`012F`, tanque de 44 L);
- consumo = distância / combustível usado.

A distância e o consumo zeram quando o firmware reinicia.

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
