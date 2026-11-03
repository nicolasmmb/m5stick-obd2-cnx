# Status do projeto e próximos passos

> Documento de consolidado do que foi discutido e implementado até agora.
> Foco: M5StickC Plus2 + ELM327 BLE + data loader no host.

## 1. Resumo

O projeto saiu de “ler alguns PIDs na tela” para um fluxo completo:

```text
M5StickC Plus2
  ├─ conecta no ELM327 BLE
  ├─ consulta PIDs
  ├─ envia linhas compactas pela serial
  └─ mostra status/dados no LCD

Host (cmd/obd-logger)
  ├─ lê a serial
  ├─ decodifica raw → valor/unidade
  ├─ grava NDJSON/CSV
  └─ gera metadados e resample 1 Hz
```

O Stick **não formata JSON nem armazena dados**. A lógica de schema, unidades,
CSV, wide e metadados fica no host.

## 2. Estrutura atual

```text
poc/obd2/
  obd-probe/       # descoberta GATT
  obd-client/      # handshake + PIDs no serial
  obd-bench/       # benchmark dos 43 PIDs
  obd-tune/        # ATAT/ATST
  obd-display/     # UI de dados OBD2
  obd-logger/      # firmware compacto + tela + ACK serial

cmd/obd-logger/    # loader host
internal/obdlog/   # parser compacto/NDJSON + CSV/metadados
examples/          # display, BLE, Wi-Fi e hello
docs/              # catálogo, dados, logger, ML, hardware e UI
testdata/          # amostras para replay
logs/              # logs reais antigos
```

POCs de Units Grove, microfone e sensores foram movidos para
`../m5stick-hardware-pocs/`.

## 3. O que está pronto

### Firmware `obd-logger`

- conexão BLE com o adaptador `OBDII`;
- handshake ELM327;
- leitura de VIN;
- scheduler de PIDs rápidos e lentos;
- protocolo compacto:

```text
I,<ms>,<fw>,<vin>,<tank_dl>
P,<ms>,<pid>,<raw>,<lat>,<q>
E,<ms>,<tipo>,<detalhe>
X,<ms>,<motivo>
```

- tela com framebuffer 4bpp e sem flicker;
- página 1: SERIAL, BLE, ELM, PID/s, TX, ERR;
- página 2: velocidade, RPM e combustível;
- página 3: água, ar admitido e catalisador;
- página 4: carga, borboleta e lambda;
- página 5: tensão, MIL e DTCs;
- página 6: distância, combustível usado e consumo;
- botões A/B (interrupt + polling fallback);
- ACK serial do host para o estado `SERIAL`;
- reconexão automática do BLE com backoff (1/2/5/10 s) e beeps no buzzer;
- `X` em perda de link, replay e shutdown do host (`C,STOP`);
- leitura de DTCs armazenados/pendentes/permanentes e readiness;
- modo `C,SIM` para rodar o loop live pela serial, sem BLE/carro.

### Host `cmd/obd-logger`

- leitura serial com `go.bug.st/serial`;
- modo `--replay`;
- parser compacto e compatibilidade com NDJSON antigo;
- decodificação de PIDs no host;
- saída:

```text
data/<sessão>/
  raw.log
  decoded.ndjson
  samples_long.csv
  samples_wide_1hz.csv
  events.jsonl
  metadata.json
  invalid.log
```

- reconexão da porta serial;
- Ctrl-C gracioso com `C,STOP` para o firmware;
- ACK `A\n` a cada 1 s;
- resumo de MIL/DTCs/readiness em `metadata.json`;
- `data/` ignorado pelo git.

### Documentação

- catálogo de PIDs e siglas;
- tabela por dado com confiabilidade/frequência;
- formato do logger;
- documentação por arquivo de saída;
- layout da tela;
- notas técnicas de hardware;
- ideias de ML e modelagem.

### Testes

- `go test` do parser e loader;
- replay compacto e NDJSON;
- `make build`;
- `make check`.

## 4. O que já foi validado

| Item | Status |
|---|---|
| Adaptador BLE `OBDII` | ✅ validado em hardware |
| Handshake ELM327 | ✅ |
| 43 PIDs Mode 01 | ✅ |
| Taxa ~5–7 Hz por PID | ✅ medido |
| Varredura completa ~7,2 s | ✅ medido |
| Loader replay compacto | ✅ |
| Loader replay NDJSON | ✅ |
| Botões A/B | ✅ confirmado |
| SERIAL WAIT/OK/DISC | ✅ confirmado |
| Cores após `IsBGR(false)` | ✅ confirmado |
| Páginas 1–6 | ✅ confirmadas visualmente |
| Replay 5–10 min sem OOM | ✅ |
| UI durante conexão BLE | ✅ confirmado (sem trava) |
| Dados salvos em `data/` | ✅ confirmado |
| Reconexão BLE + beeps | ⏳ pendente de hardware |
| `X` em `ble_lost`/`host` | ✅ testes de host; ⏳ hardware |
| DTC/readiness | ✅ testes de host; ⏳ carro |
| Simulação serial (`C,SIM`) | ✅ host + firmware compilando; ⏳ hardware |
| Log real de 5–10 min no carro | ⏳ pendente |

## 5. Decisões tomadas

- Stick envia **raw**; host decodifica.
- Protocolo principal: **compacto**, não JSON no Stick.
- Saída canônica: `decoded.ndjson`.
- CSV long é a fonte de verdade; wide é derivado.
- Wide em 1 Hz com `age_ms`.
- VIN pode ser mantido nos logs.
- `data/` não vai para o git.
- Loader em **Go**.
- Sticks sem OBD2 continuam atualizando a UI e tentando reconectar.
- `SERIAL` usa ACK do host, não DTR.
- BLE/ELM têm estados próprios; ELM não mostra ERR antes de conectar.

## 6. Pendências e riscos

| Item | Risco/ação |
|---|---|
| Teste real | Rodar 5–10 min no carro e validar `raw.log`/CSV |
| Reconexão BLE | Validar no hardware: derrubar o adaptador, ouvir os beeps e ver `X,ble_lost` |
| `C,STOP` | Confirmar `X,host` e `BLE=OFF` após Ctrl-C |
| VIN | Parser multi-frame precisa ser validado no log real |
| DTC/readiness | Validar códigos reais no carro; resposta sem DTC é `43 00 AA...` |
| Sequência | `P` compacto não tem `seq`; host usa tempo/ordem |
| Cal ID/Mode 06 | Ainda parciais |
| BLE após `C,STOP` | `espradio` não tem `Deinit`; a pausa não devolve a arena do controlador |

## 7. Próximos passos

### P0 — validar o que está na mão

1. ~~Confirmar troca de página com A/B.~~ ✅
2. ~~Confirmar cores: WAIT laranja, OK verde, DISC vermelho.~~ ✅
3. ~~Rodar `make log` e ver SERIAL=OK.~~ ✅
4. ~~Fechar o loader e ver SERIAL=DISC após 3 s.~~ ✅
5. Rodar uma sessão real de 5–10 min no carro. ⏳
6. Conferir `raw.log`, `decoded.ndjson` e CSVs. ⏳ (dados de serial reais já conferidos)

### P1 — robustez (implementado, falta hardware)

7. ~~Detectar perda de BLE e reconectar.~~ ✅
8. ~~Emitir `X` em shutdown/reconexão (`C,STOP`).~~ ✅
9. ~~Adicionar DTCs/readiness ao logger.~~ ✅
10. Adicionar `seq` ou `session_id` no protocolo. ⏳
11. Validar VIN no carro. ⏳
12. Validar reconexão, `X` e DTCs no hardware. ⏳

Detalhes em [P1-ROBUSTNESS.md](P1-ROBUSTNESS.md).

### P2 — replay e dados

- ~~implementar `cmd/obd-replay` e modo REPLAY no firmware~~ ✅;
- ~~testar gravações sem o carro~~ ✅;
- migrar a UI para snapshots com qualidade/idade.

### P3 — dados e ML

13. Converter CSV para Parquet.
14. Criar notebook/pipeline de análise.
15. Gerar features de viagem.
16. Primeiro modelo: consumo/autonomia.
17. Segundo: anomalia/manutenção.
18. Terceiro: score de direção.

## 8. Comandos úteis

```bash
make obd-logger         # grava o firmware live (BLE + tela + replay)
make log                # lê a serial e grava em data/; Ctrl-C envia C,STOP
make replay             # testa o loader com sample compacto
make replay-device      # envia um raw.log ao Stick
make sim                # loop live via serial (sem BLE)
make test
make build
make check
```

Firmware: um único binário (live + replay).

### Performance do firmware

| Caminho | Tempo | Alocs |
| --- | --- | --- |
| `obdwire.ParseCommand` (host) | 95 ns/op | 1 |
| `obdline.Parse` (Stick) | 31 ns/op | 0 |
| `Registry.DecodeBytes` | 48 ns/op | 0 |
| `Cache.Put` + `Get` | 39 ns/op | 0 |

- cache fixo de 64 slots, sem `map` no caminho de leitura/escrita;
- `Reading` não retém mais `Raw []byte`;
- texto desenhado direto pelo glifo, sem a slice de cores do `tinyfont`;
- replay validado por 90 s / 2.436 registros sem OOM, a ~70 ticks/s;
- binário P1: flash 307.844 / RAM 59.084 (antes 302.356 / 59.028);
- com simulador serial: flash 318.832 / RAM 59.804.

### BLE no modo replay

O firmware é um único binário completo (live + replay). O BLE só é
inicializado quando o modo live começa; ao entrar em `C,REPLAY`:

- o scan é parado;
- o adaptador não é habilitado de novo;
- se já estava conectado, a sessão é encerrada no próximo ciclo.

Se o host enviar `C,REPLAY` nos primeiros 2 s após o boot, o
`Adapter.Enable()` nunca é chamado e a arena do controlador BLE não é alocada
em runtime.

Limitação: `espradio` não expõe `Deinit`; depois que o BLE foi habilitado uma
vez, a região reservada no linker não pode ser devolvida ao heap. Nesse caso o
firmware para o scan, encerra a conexão e não usa mais o adaptador até um novo
`C,LIVE`.

## 9. Documentos relacionados

- [README](../README.md)
- [P1 — robustez](P1-ROBUSTNESS.md)
- [Simulador serial](SERIAL-SIM.md)
- [Plano de otimização](OPTIMIZATION-PLAN.md)
- [Formato do logger](OBD2-LOGGER-FORMAT.md)
- [Formato do replay](OBD2-REPLAY-FORMAT.md)
- [Documentação por arquivo](logger/README.md)
- [Layout da tela](ui-layout.md)
- [Catálogo OBD2](OBD2-DATA-CATALOG.md)
- [Dados decodificados](OBD2-DECODED-DATA.md)
- [Ideias de ML](ml/PROJECT-IDEAS.md)
- [Notas de hardware](hardware/README.md)

## 10. Estado do git

A otimização e o replay estão mergeados na `main` (`d0da8ad`). O P1
(reconexão, `X`/`C,STOP`, DTCs/readiness) está commitado na `main`. O
simulador serial (`C,SIM`) está na branch `feat/serial-sim`. O P0-5 (sessão
real de 5–10 min no carro) segue pendente.
