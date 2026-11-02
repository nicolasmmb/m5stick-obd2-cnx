# CLI do leitor serial

## Build

```bash
make logger
# ou
go build -o /tmp/obd-logger ./cmd/obd-logger
```

## Uso com o carro

```bash
/tmp/obd-logger \
  --port /dev/cu.usbserial-XXXX \
  --baud 115200 \
  --out data
```

Ou:

```bash
make log PORT=/dev/cu.usbserial-XXXX
```

Antes de rodar:

1. grave o firmware `make obd-logger`;
2. feche o monitor serial com `Ctrl-C`;
3. rode o leitor.

## Replay

```bash
/tmp/obd-logger \
  --replay testdata/obd-logger-sample-compact.txt \
  --out data
```

Também aceita NDJSON antigo.

## Flags

| Flag | Padrão | Descrição |
|---|---|---|
| `--port` | — | porta serial |
| `--baud` | `115200` | baud rate |
| `--out` | `data` | diretório de saída |
| `--session` | timestamp | ID da sessão |
| `--replay` | — | arquivo de replay em vez da serial |

## Comportamento

- reconecta se a serial cair;
- Ctrl-C fecha a sessão e grava `metadata.json`;
- linhas inválidas vão para `invalid.log`;
- `data/<sessão>/` é criado automaticamente;
- se o diretório existir, recebe sufixo `-1`, `-2`, etc.

## Solução de problemas

| Sintoma | Ação |
|---|---|
| `no serial port` | feche monitor/`screen`; reconecte USB |
| permissão negada | verifique a porta `/dev/cu.*` |
| linha inválida | veja `invalid.log` |
| sem amostras | confirme o firmware e o baud 115200 |
| VIN vazio | o adaptador pode não responder `0902` |
