# `invalid.log`

Linhas que o parser não conseguiu interpretar.

## Exemplo

```text
P,abc,0C,1AF8,OK
{"t":"P"}
```

## Causas comuns

- linha truncada pela serial;
- baud errado;
- firmware antigo com outro formato;
- JSON inválido;
- campos obrigatórios ausentes.

## Uso

- diagnosticar cabo/baud;
- comparar com `raw.log`;
- verificar se uma versão de firmware quebrou o protocolo.

## Observações

- Linhas inválidas não entram em `samples_long.csv`.
- O contador aparece em `metadata.json` como `invalid_lines`.
- Se o arquivo crescer muito, pare a coleta e verifique o firmware/porta.
