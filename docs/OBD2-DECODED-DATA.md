# Dados OBD2 decodificados — tabela consolidada

> Baseado **somente** nas capturas já feitas com o ELM327 BLE `OBDII`
> (`ELM327 v2.1`, serviço `FFF0`).
> Não há novas consultas ao carro neste documento.

## Legenda

### Confiabilidade

| Nível | Significado |
|---|---|
| **Alta** | PID padrão SAE J1979, resposta real capturada e fórmula aplicada |
| **Média** | Valor derivado/assumido (ex.: litros dependem da capacidade do tanque) ou bruto ambíguo |
| **Parcial** | Log truncado ou decodificação incompleta |
| **N/A** | O carro não respondeu / não suporta |

### Frequência

| Rótulo | Significado |
|---|---|
| `~6 Hz` | limite medido para **um** PID isolado |
| `~2,5 Hz` | página 1 do app (velocidade + RPM) |
| `~1,8 Hz` | página 2 do app (3 temperaturas) |
| `~0,3 Hz` | combustível, consultado a cada 8 ciclos |
| `~0,5–1 Hz` | etanol, consultado a cada 10 ciclos |
| `0,1–0,2 Hz` | varredura completa dos 43 PIDs (7,2 s) |
| `1×/sessão` | dado estático |

### Unidades usadas nos exemplos

`km/h`, `rpm`, `°C`, `%`, `kPa`, `V`, `g/s`, `λ`, `km`, `s`, `L` e contagem.
As pressões do OBD2 vieram em **kPa**, não em psi. Não existe **km/l** neste
conjunto: consumo instantâneo não é um PID que capturamos.

## Mode 01 — dados ao vivo

| PID | Dado | Tipo / descrição | Exemplo exibido | Confiabilidade | Frequência | Observações |
|---|---|---|---|---|---|---|
| `01` | Monitor status since DTCs cleared | Bitmask: MIL, nº de DTCs e readiness | MIL OFF • 0 DTCs | Alta | 0,1–0,2 Hz | Raw `00 07 E1 00`: MIL apagada, 0 DTCs, ignição por faísca, testes disponíveis completos |
| `03` | Fuel system status | Bitmask por banco (2 bytes) | Fuel: N/D | Média | 0,1–0,2 Hz | Raw `00 00`: nenhum estado de loop reportado |
| `04` | Calculated engine load | Carga calculada do motor (%) | 0% | Alta | ~6 Hz máx.; sob demanda | Raw `00` = 0% com motor desligado |
| `05` | Engine coolant temperature (ECT) | Temperatura do líquido (°C) | 75 °C | Alta | ~1,8 Hz (página 2) | Raw `73` = 75 °C |
| `06` | Short term fuel trim bank 1 (STFT) | Correção rápida de combustível (%) | 0% | Alta | ~6 Hz máx.; sob demanda | Raw `80` = 0% |
| `07` | Long term fuel trim bank 1 (LTFT) | Correção aprendida de combustível (%) | -3,9% | Alta | ~6 Hz máx.; sob demanda | Raw `7B` = -3,9% |
| `0B` | Intake manifold absolute pressure (MAP) | Pressão absoluta do coletor (kPa) | 95 kPa | Alta | ~6 Hz máx.; sob demanda | Raw `5F` = 95 kPa |
| `0C` | Engine speed (RPM) | Rotação do motor (rpm) | 1.100 rpm | Alta | ~2,5 Hz (página 1) | 0 rpm desligado; ~1.100 rpm em marcha lenta |
| `0D` | Vehicle speed (VSS) | Velocidade do veículo (km/h) | 0 km/h | Alta | ~2,5 Hz (página 1) | Raw `00` = 0 km/h |
| `0E` | Timing advance | Avanço de ignição (° BTDC) | 42,5° BTDC | Alta | ~6 Hz máx.; sob demanda | Raw `D5` = 42,5° BTDC; pode não ter sentido com motor desligado |
| `0F` | Intake air temperature (IAT) | Temperatura do ar admitido (°C) | 43 °C | Média | ~1,8 Hz (página 2) | Bench: raw `53` = 43 °C; em outra leitura veio `00` (-40 °C): inconsistente |
| `10` | MAF air flow rate | Vazão de ar por massa (g/s) | 0,0 g/s | Alta | ~6 Hz máx.; sob demanda | Raw `00 00` = 0 g/s com motor desligado |
| `11` | Throttle position (TPS) | Posição da borboleta (%) | 31% | Alta | ~6 Hz máx.; sob demanda | Raw `50` = 31,4% |
| `13` | Oxygen sensors present (2 banks) | Bitmask de sensores O2 presentes | B1S1 + B1S2 | Alta | 0,1–0,2 Hz | Raw `03` = B1S1 e B1S2 presentes |
| `14` | O2 sensor 1 bank 1 | Tensão e STFT (V, %) | 1,275 V / 0% | Alta | ~6 Hz máx.; sob demanda | Raw `FF 80` = 1,275 V / 0% |
| `15` | O2 sensor 2 bank 1 | Tensão e STFT (V, %) | 1,275 V / n/d | Alta | ~6 Hz máx.; sob demanda | Raw `FF FF` = 1,275 V; B=`FF` indica sensor não usado no trim |
| `1C` | OBD standard | Enumeração da norma OBD | OBDBr-2 | Alta | 1×/sessão | Raw `1D` = 29 = **OBDBr-2** |
| `1F` | Run time since engine start | Tempo desde a partida (s) | 0 s | Alta | 0,1–0,2 Hz | Raw `00 00` = 0 s com motor desligado |
| `20` | PIDs supported 21–40 | Bitmask | bitmask | Alta | 1×/sessão | Usado para descobrir a página seguinte |
| `21` | Distance traveled with MIL on | Distância com MIL ligada (km) | 0 km | Alta | 0,1–0,2 Hz | Raw `00 00` = 0 km |
| `2E` | Commanded evaporative purge | Purga do EVAP comandada (%) | 0% | Alta | 0,1–0,2 Hz | Raw `00` = 0% |
| `2F` | Fuel tank level input | Nível do tanque (%) | 58% (25,5 L*) | Alta (para %) | ~0,3 Hz (página 1) | Raw `95` = 58,4%; litros são derivados |
| `30` | Warm-ups since codes cleared | Aquecimentos desde limpeza | 17 | Alta | 0,1–0,2 Hz | Raw `11` = 17 |
| `31` | Distance since codes cleared | Distância desde limpeza (km) | 77 km | Alta | 0,1–0,2 Hz | Raw `00 4D` = 77 km |
| `33` | Absolute barometric pressure | Pressão barométrica (kPa) | 95 kPa | Alta | 0,1–0,2 Hz | Raw `5F` = 95 kPa |
| `3C` | Catalyst temperature B1S1 | Temperatura do catalisador (°C) | 506 °C | Alta | ~1,8 Hz (página 2) | Raw `15 54` = 506 °C |
| `40` | PIDs supported 41–60 | Bitmask | bitmask | Alta | 1×/sessão | Usado para descobrir a página seguinte |
| `41` | Monitor status this drive cycle | Bitmask de readiness do ciclo | O2/cat. incompl. | Alta | 0,1–0,2 Hz | Raw `00 05 61 61`: O2 aquecedor/O2/catalisador ainda incompletos neste ciclo |
| `42` | Control module voltage | Tensão do módulo (V) | 12,54 V | Alta | ~6 Hz máx.; sob demanda | Raw `30 FB` = 12,539 V |
| `43` | Absolute load value | Carga absoluta (%) | 0% | Alta | ~6 Hz máx.; sob demanda | Raw `00 00` = 0% |
| `44` | Commanded equivalence ratio | Lambda comandado (λ) | 0,852 λ | Alta (fórmula); valor a confirmar | ~6 Hz máx.; sob demanda | Raw `6D 00` = 0,852 λ; com motor desligado pode não ser representativo |
| `45` | Relative throttle position | Posição relativa da borboleta (%) | 21% | Alta | ~6 Hz máx.; sob demanda | Raw `36` = 21,2% |
| `46` | Ambient air temperature | Temperatura ambiente (°C) | 26 °C | Alta | 0,1–0,2 Hz | Raw `42` = 26 °C |
| `47` | Absolute throttle position B | Posição absoluta da borboleta B (%) | 31% | Alta | ~6 Hz máx.; sob demanda | Raw `50` = 31,4% |
| `49` | Accelerator pedal position D | Posição do pedal D (%) | 19% | Alta | ~6 Hz máx.; sob demanda | Raw `31` = 19,2% |
| `4A` | Accelerator pedal position E | Posição do pedal E (%) | 10% | Alta | ~6 Hz máx.; sob demanda | Raw `1A` = 10,2% |
| `4C` | Commanded throttle actuator | Atuador de borboleta comandado (%) | 23% | Alta | ~6 Hz máx.; sob demanda | Raw `3A` = 22,7% |
| `4F` | Max λ/O2 voltage/O2 current/MAP | Limites máximos | — | Baixa / Parcial | 1×/sessão | Raw `00 00 00 00`: limites não reportados |
| `51` | Fuel type | Enumeração do combustível | Etanol | Alta (enum) | 1×/sessão | Raw `03` = Etanol; conflita com `0152` ≈ 40% |
| `52` | Ethanol fuel % | Percentual de etanol (%) | 38% (ou 40%) | Alta | ~0,5–1 Hz (página 3) | Bench: raw `62` = 38,4%; em outra leitura o app mostrou 40% (raw ≈ `66`) |
| `60` | PIDs supported 61–80 | Bitmask | bitmask | Alta | 1×/sessão | Usado para descobrir a página seguinte |
| `67` | Engine coolant temperature (2 sensores) | Flags + temperaturas (°C) | 75 °C (S1) | Alta | ~6 Hz máx.; sob demanda | Não usado no app (usa `0105`); raw `01 73 00`: sensor 1 = 75 °C, sensor 2 ausente |
| `68` | Intake air temperature (2 sensores) | Flags + temperaturas (°C) | 43 / 49 °C | Média | ~6 Hz máx.; sob demanda | Não usado no app (usa `010F`); raw `03 53 59` = 43/49 °C; 4 bytes extras `28 28 28 28` não identificados |

## Dados de identidade e diagnóstico

| Fonte | Dado | Tipo / descrição | Exemplo exibido | Confiabilidade | Frequência | Observações |
|---|---|---|---|---|---|---|
| `0902` | VIN | Identidade do veículo (17 ASCII) | VIN (17 chars) | Alta | 1×/sessão | Lido e decodificado; o valor não é reproduzido aqui, mas o log bruto em `logs/` contém o VIN em hexadecimal |
| `0904` | Calibration ID (CAL) | Identificadores ASCII da calibração | 6 IDs (parcial) | Parcial | 1×/sessão | Contador `06` = 6 IDs; log truncado, não foi possível montar todos |
| `0906` | CVN | Checksums de 4 bytes por calibração | 000032C8… | Alta (valores) / Parcial (mapa) | 1×/sessão | 6 CVNs: `000032C8`, `00001533`, `000099F5`, `0000687A`, `000095F2`, `00004652`; falta casar com os Cal IDs |
| `090A` | ECU name | Nome ASCII do módulo | ECM-EngineControl | Alta | 1×/sessão | `ECM-EngineControl` |
| `03` | DTCs armazenados | Códigos de falha confirmados | Nenhum | Alta | sob demanda | `43 00 00` = nenhum DTC |
| `07` | DTCs pendentes | Códigos pendentes | Nenhum | Alta | sob demanda | `47 00 00` = nenhum |
| `0A` | DTCs permanentes | Códigos permanentes | Nenhum | Alta | sob demanda | `4A 00 00` = nenhum |
| `0101`/`0141` | Readiness | Monitores disponíveis/completos | MIL OFF • O2/cat. pendente | Alta | 0,1–0,2 Hz | `0101 = 00 07 E1 00`; `0141 = 00 05 61 61` |
| `0200`/`0202`/`020C` | Freeze frame | Snapshot do DTC | — | N/A | — | `NO DATA`; o carro não forneceu freeze frame |
| `0600` | Mode 06 — suporte | Bitmask de MIDs | suporte bruto | Parcial | sob demanda | Raw `46 00 C0 00 00 01`; suporte existe, decodificação pendente |
| `0601` | Mode 06 — teste 01 | TID/MID + valores | — (bruto) | Parcial | sob demanda | Resposta multi-frame; log truncado; falta tabela J1979-DA |
| `ATI` | Identificação do adaptador | Texto | ELM327 v2.1 | Alta | 1×/sessão | `ELM327 v2.1` |
| `ATDP`/`ATRV`/`ATIGN` | Protocolo / tensão / ignição | Texto/valor | — | N/A | — | Não foram coletados |

## Dados derivados (não são PIDs diretos)

| Dado | Fórmula / origem | Exemplo exibido | Confiabilidade | Frequência | Observações |
|---|---|---|---|---|
| Combustível em litros | `raw_2F / 255 * capacidade` | 25,5 L | Média | ~0,3 Hz | Depende da capacidade assumida de **44,0 L** (`tankCapacityDeciL = 440`) |
| Percentual de gasolina | `100 - percentual_etanol` | 60% | Média | ~0,5–1 Hz | Derivado; não existe PID separado de gasolina |
| Rótulo `FLEX` | Etanol entre 1% e 99% | FLEX | Média | ~0,5–1 Hz | Mostrado na página 3; o PID `0151` diz “Etanol” |
| Velocidade em km/h | `010D` (A) | 0 km/h | Alta | ~2,5 Hz | Direto |
| RPM | `(256A+B)/4` de `010C` | 1.100 rpm | Alta | ~2,5 Hz | Direto |

## O que ficou sem identificação completa

| Item | Evidência | Motivo / pendência |
|---|---|---|
| Bytes extras do `0168` | Resposta de 9 bytes: `41 68 03 53 59 28 28 28 28` | PID padrão tem 3 bytes de dados; os quatro `28` extras não têm definição clara |
| Bit `0x02` do `0900` | `49 00 55 40 02 00` | PIDs `02/04/06/08/0A` são claros; o bit `0x02` não mapeia para um PID padrão do Mode 09 |
| Mode 06 (`0601`) | Resposta multi-frame de 73 bytes, truncada no log | Falta a resposta completa e as tabelas J1979-DA de TIDs/MIDs |
| Calibration ID completo (`0904`) | Resposta de 99 bytes, truncada no log | Só foi possível ver o contador (`06`) e parte do primeiro ID |
| `014F` | `00 00 00 00` | Limites máximos vieram zerados; não há valor útil |
| `0103` | `00 00` | Nenhum estado de fuel system reportado |
| `010F` | `53` (43 °C) no bench e `00` (-40 °C) em outra leitura | Inconsistência; provável sensor frio/ignição ou ruído, não confirmado |
| `0151` × `0152` | Tipo `03` (Etanol), etanol `38–40%` | O tipo do combustível e o percentual parecem conflitar |
| Protocolo do carro | `ATSP0` funcionou | `ATDP` não foi consultado; “CAN 11-bit” é inferência |
| PIDs do fabricante | Nunca consultados | Mode `21`/`22`/UDS podem expor câmbio, óleo, bateria etc. |

## Resumo

- **43 PIDs Mode 01** suportados e mapeados para a norma.
- **VIN, nome da ECU e 6 CVNs** decodificados.
- **DTCs**: nenhum armazenado, pendente ou permanente.
- **Cal ID** e **Mode 06**: apenas parciais.
- **Litros de combustível** e **% gasolina** são derivados, não medidos diretamente.
- **Freeze frame**: não suportado pelo carro.
