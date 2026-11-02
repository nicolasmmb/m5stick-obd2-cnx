# Logger OBD2 — documentação dos arquivos

O firmware `poc/obd2/obd-logger` envia linhas compactas pela serial. O host
`cmd/obd-logger` decodifica e grava os arquivos abaixo em `data/<sessão>/`.

## Arquivos

| Arquivo | Documento | Quem grava |
|---|---|---|
| `raw.log` | [raw-log.md](raw-log.md) | host, cópia exata da serial |
| `decoded.ndjson` | [decoded-ndjson.md](decoded-ndjson.md) | host, JSON canônico |
| `samples_long.csv` | [samples-long.md](samples-long.md) | host |
| `samples_wide_1hz.csv` | [samples-wide.md](samples-wide.md) | host |
| `events.jsonl` | [events.md](events.md) | host |
| `metadata.json` | [metadata.md](metadata.md) | host |
| `invalid.log` | [invalid-lines.md](invalid-lines.md) | host |

## Uso rápido

```bash
# 1. gravar o firmware
make obd-logger

# 2. fechar o monitor serial (Ctrl-C)

# 3. ler a serial e gravar
make log
```

Teste sem carro:

```bash
make replay
```

## Estrutura de saída

```text
data/
  20261102-183000/
    raw.log
    decoded.ndjson
    samples_long.csv
    samples_wide_1hz.csv
    events.jsonl
    metadata.json
    invalid.log
```

`data/` está no `.gitignore`.

## Protocolo

Ver [protocol.md](protocol.md) e
[../OBD2-LOGGER-FORMAT.md](../OBD2-LOGGER-FORMAT.md).

## Tela

Ver [../ui-layout.md](../ui-layout.md).

## Privacidade

O VIN pode aparecer em `raw.log`, `decoded.ndjson` e `metadata.json`. Remova ou
hasheie antes de compartilhar os logs.
