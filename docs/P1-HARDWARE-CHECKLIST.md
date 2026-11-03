# P1 — checklist de validação em hardware

> P1 está implementado e em backlog. Use este roteiro para fechar a validação
> sem precisar reler o código.

## 0. Preparação

```bash
# firmware
make obd-logger

# loader em outro terminal
make log
```

Confirme na página 1: `BLE=ON`, `ELM=OK`, `SERIAL=OK`.

## 1. Reconexão BLE + beeps

1. Com o carro/adaptador conectado, desligue o adaptador (ou afaste o M5).
2. O esperado no serial:
   ```text
   E,<ms>,NO_DATA,<pid>   (até 3 timeouts)
   X,<ms>,ble_lost
   ```
3. O buzzer deve emitir **1 beep** no backoff de 1 s; se falhar de novo,
   2, depois 5, depois 10.
4. Ao religar o adaptador:
   ```text
   E,<ms>,RECONNECT,ble_lost
   I,<ms>,0.1,<vin>,440
   E,<ms>,MIL,...
   ```
5. O `ms` deve continuar crescendo; **não** reinicia em 0.

Aceite: a UI continua atualizando o heartbeat durante tudo.

## 2. `C,STOP` no Ctrl-C

1. Deixe o loader rodando.
2. Pressione Ctrl-C no `make log`.
3. O esperado:
   ```text
   X,<ms>,host
   K,STOP
   ```
4. A página 1 deve mostrar `BLE=OFF`; o firmware fica pausado.
5. `metadata.json` deve ter `"end_reason": "host"`.

## 3. DTC / readiness

1. No carro com DTC conhecido (ou usando `--sim`):

   ```bash
   go run ./cmd/obd-replay --sim \
     --port /dev/cu.usbserial-XXXX \
     --file data/<sessão>/raw.log \
     --mil --dtcs P0133,U0123 --readiness 07E100
   ```

2. Confira no serial/`events.jsonl`:
   ```text
   E,<ms>,MIL,ON
   E,<ms>,DTC_COUNT,2
   E,<ms>,READINESS,07E100
   E,<ms>,DTC,P0133
   E,<ms>,DTC,U0123
   ```
3. `metadata.json` deve ter `mil`, `dtc_count`, `readiness`, `dtcs`.

## 4. VIN e CSV

1. Após uma sessão real, confira `raw.log`:
   ```text
   I,<ms>,0.1,<17 caracteres>,440
   ```
2. Confira `metadata.json` com `vin` preenchido.
3. `samples_long.csv` e `samples_wide_1hz.csv` devem ter linhas e `age_ms`
   coerente.

## 5. Memória com BLE conectado

O baseline medido sem conexão real (build P2 + `C,MEM`):

| Momento | Heap livre |
|---|---|
| boot | 41.899 B |
| `Adapter.Enable()` | 22.731 B |

Com o adaptador conectado, peça a medição pela serial:

```bash
stty -f /dev/cu.usbserial-XXXX 115200 raw -echo
printf 'C,MEM\n' > /dev/cu.usbserial-XXXX
```

O firmware responde:

```text
M,mem,alloc=<n>,free=<n>,sys=<n>,idle=<n>,total=<n>,gc=<n>
```

Aceite: `free` **≥ 20 KB** com BLE conectado.

## 6. Critérios de saída do backlog

- [ ] reconexão + beeps validados;
- [ ] `X,host` e `BLE=OFF` no Ctrl-C;
- [ ] DTC/readiness reais no carro;
- [ ] VIN multi-frame real;
- [ ] heap ≥ 20 KB com BLE conectado;
- [ ] sessão de 5–10 min sem OOM/trava.
