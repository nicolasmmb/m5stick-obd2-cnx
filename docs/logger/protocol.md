# Protocolo compacto

## I — início

```text
I,<ms>,<fw>,<vin>,<tank_dl>
```

Exemplo:

```text
I,0,0.1,9BGEA48A0LG185881,440
```

- `ms` = milissegundos desde o boot, monotônico (não reinicia na reconexão).
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
E,6100,MIL,ON
E,6101,DTC_COUNT,2
E,6102,READINESS,07E100
E,6103,DTC,P0133
E,6104,DTC_PENDING,P0300
E,6105,DTC_PERMANENT,P0420
E,7000,RECONNECT,ble_lost
```

## X — fim

```text
X,<ms>,<motivo>
```

Exemplos:

```text
X,600000,host
X,600100,ble_lost
X,600200,replay
```

## Comandos host → Stick

```text
A                 ACK de presença do host
C,REPLAY          entra em replay
C,LIVE            volta ao modo live
C,CLEAR           limpa o cache
C,STOP            encerra o live, emite X,host e pausa o BLE
R,<registro>      injeta um registro no replay
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
