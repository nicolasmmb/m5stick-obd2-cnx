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
- botões A/B (interrupt + polling fallback);
- ACK serial do host para o estado `SERIAL`.

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

- reconexão;
- Ctrl-C gracioso;
- ACK `A\n` a cada 1 s;
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
| Firmware compacto no carro | ⏳ pendente |
| Botões A/B | ⏳ pendente de confirmação |
| SERIAL WAIT/OK/DISC | ⏳ pendente de confirmação |
| Cores após `IsBGR(false)` | ⏳ pendente de confirmação |
| Página 3 (temperaturas) | ⏳ pendente de confirmação visual |
| Log real de 5–10 min | ⏳ pendente |

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
| Botões A/B | Ainda não confirmados; se não funcionarem, criar POC de GPIO |
| Cores | `IsBGR(false)` foi aplicado; confirmar laranja/vermelho/verde |
| ACK serial | Confirmar `WAIT → OK → DISC` no hardware |
| Teste real | Rodar 5–10 min no carro e validar `raw.log`/CSV |
| BLE cai durante polling | Hoje continua tentando PIDs; falta detectar e reconectar |
| VIN | Parser multi-frame precisa ser validado no log real |
| DTC/readiness | Ainda não entram no logger |
| Sequência | `P` compacto não tem `seq`; host usa tempo/ordem |
| Cal ID/Mode 06 | Ainda parciais |

## 7. Próximos passos

### P0 — validar o que está na mão

1. Confirmar troca de página com A/B.
2. Confirmar cores: WAIT laranja, OK verde, DISC vermelho.
3. Rodar `make log` e ver SERIAL=OK.
4. Fechar o loader e ver SERIAL=DISC após 3 s.
5. Rodar uma sessão real de 5–10 min no carro.
6. Conferir `raw.log`, `decoded.ndjson` e CSVs.

### P1 — robustez

7. Detectar perda de BLE e reconectar.
8. Adicionar `seq` ou `session_id` no protocolo.
9. Emitir `X` em shutdown/reconexão.
10. Adicionar DTCs/readiness ao logger.
11. Validar VIN no carro.
12. Criar teste de hardware para A/B.

### P2 — dados e ML

13. Converter CSV para Parquet.
14. Criar notebook/pipeline de análise.
15. Gerar features de viagem.
16. Primeiro modelo: consumo/autonomia.
17. Segundo: anomalia/manutenção.
18. Terceiro: score de direção.

## 8. Comandos úteis

```bash
make obd-logger   # grava firmware compacto + tela
# Ctrl-C para fechar o monitor
make log          # lê a serial e grava em data/
make replay       # testa com sample compacto
make test
make build
make check
```

## 9. Documentos relacionados

- [README](../README.md)
- [Formato do logger](OBD2-LOGGER-FORMAT.md)
- [Documentação por arquivo](logger/README.md)
- [Layout da tela](ui-layout.md)
- [Catálogo OBD2](OBD2-DATA-CATALOG.md)
- [Dados decodificados](OBD2-DECODED-DATA.md)
- [Ideias de ML](ml/PROJECT-IDEAS.md)
- [Notas de hardware](hardware/README.md)

## 10. Estado do git

A última implementação commitada foi:

```text
a53b791 fix: ui
```

As mudanças de tela/status/ACK e este documento estão no working tree e ainda
não foram commitadas.
