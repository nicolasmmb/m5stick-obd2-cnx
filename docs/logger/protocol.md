# Protocolo compacto

## I — início

```text
I,<ms>,<fw>,<vin>,<tank_dl>
```

Exemplo:

```text
I,0,0.1,9BGEA48A0LG185881,440
```

- `ms` = milissegundos desde o início.
- `fw` = versão do firmware.
- `vin` = VIN de 17 caracteres (pode ser vazio).
- `tank_dl` = capacidade do tanque em decilitros.

## P — PID

```text
P,<ms>,<pid>,<raw_hex>,<lat_ms>,<qualidade>
```

Exemplo:

```text
P,1234,0C,1AF8,138,OK
```

O host aplica a fórmula do PID. `raw_hex` contém os bytes de dados, sem o
cabeçalho `41 <pid>`.

## E — evento

```text
E,<ms>,<tipo>,<detalhe>
```

Exemplos:

```text
E,5000,NO_DATA,0C
E,6000,PARSE_ERROR,0D
```

## X — fim

```text
X,<ms>,<motivo>
```

Exemplo:

```text
X,600000,user
```

## Regras

- uma linha por registro;
- separador `,`;
- sem JSON no Stick;
- `P` não leva unidade nem valor decodificado;
- o host é a fonte de verdade de fórmulas e unidades.

## Compatibilidade

O host também aceita o NDJSON antigo:

```json
{"t":"P","ms":1234,"pid":"0C","raw":"1AF8","v":1726,"u":"rpm","q":"OK","lat":138}
```
