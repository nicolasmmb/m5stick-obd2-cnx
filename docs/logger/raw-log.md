# `raw.log`

Cópia exata das linhas recebidas da serial, antes de qualquer decodificação.

## Exemplo

```text
I,0,0.1,9BGEA48A0LG185881,440
P,1234,0C,1AF8,138,OK
P,1250,0D,00,132,OK
E,5000,NO_DATA,0C
X,600000,user
```

## Para que serve

- auditar o que o Stick realmente enviou;
- reproduzir a sessão com `--replay`;
- depurar problemas de protocolo;
- comparar versões de firmware.

## Observações

- Pode conter NDJSON antigo se o firmware antigo for usado.
- Pode conter VIN.
- Linhas inválidas também ficam aqui; `invalid.log` é uma cópia só delas.
- Não edite durante a coleta.
