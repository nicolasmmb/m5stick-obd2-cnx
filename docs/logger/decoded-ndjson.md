# `decoded.ndjson`

Versão JSON canônica gerada pelo host a partir de `raw.log`.

## Exemplo

```json
{"t":"P","seq":1,"ms":1234,"pid":"0C","raw":"1AF8","v":1726,"u":"rpm","q":"OK","lat":138}
```

## Campos

| Campo | Significado |
|---|---|
| `t` | `S`, `P`, `E` ou `X` |
| `ms` | tempo do dispositivo |
| `pid` | PID |
| `raw` | bytes originais |
| `v` | valor decodificado |
| `u` | unidade |
| `q` | qualidade |
| `lat` | latência em ms |

## Para que serve

- consumir em Python/Go/Julia sem implementar o protocolo compacto;
- comparar com `raw.log`;
- alimentar análise e ML;
- manter um formato estável mesmo se o protocolo serial mudar.

## Observações

- `P` sem decodificador conhecido recebe `q = NO_DECODER`.
- O host não inventa valor para PID desconhecido.
- O VIN aparece no registro `S`.
