# `events.jsonl`

Eventos e erros recebidos do firmware.

## Formato

```json
{"t":"E","ms":5000,"type":"NO_DATA","msg":"0C"}
```

## Uso

- saber quando um PID falhou;
- separar gaps de dados;
- registrar DTCs/abastecimento no futuro;
- correlacionar com amostras.

## Tipos atuais

| Tipo | Significado |
|---|---|
| `NO_DATA` | PID sem dados |
| `PARSE_ERROR` | resposta inválida |
| `note` | evento manual/replay |

## Observações

- O evento não substitui a amostra; ele explica ausências.
- `ms` é o tempo do dispositivo.
