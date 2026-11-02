# Catálogo de dados OBD2 — M5StickC Plus2 + ELM327 BLE

> Documento vivo. Tudo aqui foi levantado no hardware real:
> adaptador `OBDII` BLE, firmware `ELM327 v2.1`, serviço `FFF0`.
> Medições de tempo no carro em novembro/2026.
>
> Tabela consolidada por dado, com confiabilidade e frequência:
> **[OBD2-DECODED-DATA.md](OBD2-DECODED-DATA.md)**.

## 1. Resumo

| Item | Valor |
|---|---|
| Adaptador | ELM327 BLE, nome `OBDII` |
| Endereço BLE | `00:10:CC:4F:36:03` |
| Serviço GATT | `FFF0` |
| Notificação (RX) | `FFF1` |
| Escrita (TX) | `FFF2` |
| MTU | 248 |
| Protocolo do carro | CAN/OBD-II (auto, `ATSP0`; confirmar com `ATDP`) |
| Norma OBD | `1C = 29` → **OBDBr-2** (Brasil fase 2) |
| Combustível | `51 = 3` → **Etanol** |
| PIDs Mode 01 suportados | **43** (sendo 3 bitmaps de suporte; ~40 de dados) |
| Latência média por PID | **166 ms** |
| PID isolado | **5–7 Hz** |
| Varredura dos 43 PIDs | **7,2 s** (~0,14 Hz) |

## 2. Como o dado chega

```text
M5StickC Plus2
  └─ BLE GATT: escreve em FFF2 / recebe notify em FFF1
       └─ ELM327 (firmware v2.1)
            └─ CAN 11-bit (ISO 15765-4)
                 └─ ECU do carro
```

O ELM327 fala ASCII. Cada comando termina em `\r` (0x0D). A resposta termina
com o prompt `>`.

### 2.1 Handshake usado

| Comando | Efeito |
|---|---|
| `ATZ` | reset do adaptador; responde `ELM327 v2.1` |
| `ATE0` | desliga eco dos comandos |
| `ATL0` | desliga linefeeds |
| `ATS0` | desliga espaços na resposta |
| `ATSP0` | seleciona protocolo automaticamente |
| `ATAT0` | opcional: desliga adaptive timing; mediu 6,9 Hz |

### 2.2 Formato de resposta

```text
410C1AF8
│ │ └── dados (A=1A, B=F8)
│ └──── PID 0C
└────── 41 = resposta do serviço 01
```

- `AA` no fim é padding do CAN; ignorar.
- `SEARCHING...` é ruído do ELM327.
- Respostas longas (VIN, Cal ID, Mode 06) vêm multi-frame com prefixos
  `0:`, `1:`, `2:`; juntar as linhas antes de decodificar.
- `NO DATA` = PID não suportado ou ECU não respondeu.
- `?` = comando inválido/sintaxe.
- `UNABLE TO CONNECT` / `STOPPED` = falha de enlace.

## 3. Serviços (modes) OBD

| Mode | Comando | Prefixo da resposta | O que entrega | Neste carro |
|---|---|---|---|---|
| 01 | `01XX` | `41` | dados atuais (PIDs) | ✅ 43 PIDs |
| 02 | `02XX` | `42` | freeze frame (foto do DTC) | ❌ `NO DATA` |
| 03 | `03` | `43` | DTCs armazenados | ✅ (nenhum) |
| 04 | `04` | `44` | apaga DTCs e readiness | ⚠️ não implementado |
| 05 | `05XX` | `45` | teste de O2 (não-CAN) | ❌ não se aplica |
| 06 | `0600`, `0601` | `46` | resultados de monitores/testes | ✅ suporte + dados brutos |
| 07 | `07` | `47` | DTCs pendentes | ✅ (nenhum) |
| 08 | `08XX` | `48` | controle de atuadores | ⚠️ perigoso, não implementado |
| 09 | `09XX` | `49` | VIN, Cal ID, CVN, nome da ECU | ✅ |
| 0A | `0A` | `4A` | DTCs permanentes | ✅ (nenhum) |

## 4. Descobrir PIDs suportados

O carro responde a páginas de 32 PIDs:

```text
0100 -> 41 00 BE 3F B8 13
0120 -> 41 20 80 07 A0 11
0140 -> 41 40 FE D2 C0 01
0160 -> 41 60 03 00 00 00
```

Cada bit `1` significa “PID suportado”. O último bit da página indica se existe
a página seguinte. Neste carro:

```text
01 03 04 05 06 07 0B 0C 0D 0E 0F 10 11 13 14 15
1C 1F 20 21 2E 2F 30 31 33 3C 40 41 42 43 44 45
46 47 49 4A 4C 4F 51 52 60 67 68
```

Não suportados (amostra): `02 08 09 0A 12 16–1B 1D 1E 22–2D 32 34–3B
3D–3F 48 4B 4D 4E 50 53–5F 61–66 69–7F 80+`.

## 5. Tabela mestra — Mode 01 (dados ao vivo)

Legenda de prioridade:

- **R** = rápido (gauge; 1,5–2 Hz)
- **M** = médio (0,5–1 Hz)
- **L** = lento (0,1–0,2 Hz)
- **E** = estático/1× por sessão

### Página 01–20

| PID | Sigla/nome | Bytes | Fórmula | Unidade | Grupo | Prio |
|---|---|---|---|---|---|---|
| `01` | Monitor status since DTCs cleared | 4 | A7=MIL; A6–A0=nº DTCs; B/C/D readiness | — | status | L |
| `03` | Fuel system status | 2 | A/B = bitmask por banco | — | combustível | M |
| `04` | Calculated engine load | 1 | `A*100/255` | % | motor | R |
| `05` | Engine coolant temperature (ECT) | 1 | `A-40` | °C | temperatura | M |
| `06` | Short term fuel trim bank 1 (STFT) | 1 | `(A-128)*100/128` | % | combustível | M |
| `07` | Long term fuel trim bank 1 (LTFT) | 1 | `(A-128)*100/128` | % | combustível | M |
| `0B` | Intake manifold absolute pressure (MAP) | 1 | `A` | kPa | motor | R |
| `0C` | Engine speed (RPM) | 2 | `(256A+B)/4` | rpm | motor | R |
| `0D` | Vehicle speed (VSS) | 1 | `A` | km/h | veículo | R |
| `0E` | Timing advance | 1 | `A/2-64` | ° BTDC | motor | R |
| `0F` | Intake air temperature (IAT) | 1 | `A-40` | °C | temperatura | M |
| `10` | MAF air flow rate | 2 | `(256A+B)/100` | g/s | motor | R |
| `11` | Throttle position (TPS) | 1 | `A*100/255` | % | motor | R |
| `13` | Oxygen sensors present (2 banks) | 1 | A0–A3=B1S1–S4; A4–A7=B2S1–S4 | — | emissões | L |
| `14` | O2 sensor 1 bank 1 | 2 | `A/200` V; `(B-128)*100/128` % | V, % | emissões | M |
| `15` | O2 sensor 2 bank 1 | 2 | igual `14` | V, % | emissões | M |
| `1C` | OBD standard | 1 | enum (ver §8.1) | — | status | E |
| `1F` | Run time since engine start | 2 | `256A+B` | s | motor | L |
| `20` | PIDs supported 21–40 | 4 | bitmask | — | status | E |

### Página 21–40

| PID | Sigla/nome | Bytes | Fórmula | Unidade | Grupo | Prio |
|---|---|---|---|---|---|---|
| `21` | Distance traveled with MIL on | 2 | `256A+B` | km | status | L |
| `2E` | Commanded evaporative purge | 1 | `A*100/255` | % | emissões | M |
| `2F` | Fuel tank level input | 1 | `A*100/255` | % | combustível | L |
| `30` | Warm-ups since codes cleared | 1 | `A` | contagem | status | L |
| `31` | Distance since codes cleared | 2 | `256A+B` | km | status | L |
| `33` | Absolute barometric pressure | 1 | `A` | kPa | ambiente | L |
| `3C` | Catalyst temperature B1S1 | 2 | `(256A+B)/10-40` | °C | emissões | M |
| `40` | PIDs supported 41–60 | 4 | bitmask | — | status | E |

### Página 41–60

| PID | Sigla/nome | Bytes | Fórmula | Unidade | Grupo | Prio |
|---|---|---|---|---|---|---|
| `41` | Monitor status this drive cycle | 4 | bitmask; A sempre 0 | — | status | L |
| `42` | Control module voltage | 2 | `(256A+B)/1000` | V | elétrico | M |
| `43` | Absolute load value | 2 | `(256A+B)*100/255` | % | motor | R |
| `44` | Commanded equivalence ratio (λ) | 2 | `2*(256A+B)/65536` | λ | combustível | M |
| `45` | Relative throttle position | 1 | `A*100/255` | % | motor | R |
| `46` | Ambient air temperature | 1 | `A-40` | °C | ambiente | L |
| `47` | Absolute throttle position B | 1 | `A*100/255` | % | motor | R |
| `49` | Accelerator pedal position D | 1 | `A*100/255` | % | motor | R |
| `4A` | Accelerator pedal position E | 1 | `A*100/255` | % | motor | R |
| `4C` | Commanded throttle actuator | 1 | `A*100/255` | % | motor | R |
| `4F` | Max λ/O2 voltage/O2 current/MAP | 4 | A, B, C, D×10 | λ, V, mA, kPa | status | E |
| `51` | Fuel type | 1 | enum (ver §8.2) | — | status | E |
| `52` | Ethanol fuel % | 1 | `A*100/255` | % | combustível | L |
| `60` | PIDs supported 61–80 | 4 | bitmask | — | status | E |

### Página 61–68

| PID | Sigla/nome | Bytes | Fórmula | Unidade | Grupo | Prio |
|---|---|---|---|---|---|---|
| `67` | Engine coolant temperature (2 sensores) | 3 | A=flags; `B-40`, `C-40` | °C | temperatura | M |
| `68` | Intake air temperature sensor (6 slots) | 7 | A=flags; `B-40` … `G-40` | °C | temperatura | M |

> Em `67`, A0/B1S1 e A1/B1S2.
> Em `68`, A0–A5 indicam B1S1, B1S2, B1S3, B2S1, B2S2 e B2S3;
> B–G são as temperaturas nessa ordem.
> Neste carro: `67 = 01 73 00` → sensor 1 = 75 °C;
> `68 = 03 53 59 28 28 28 28` → B1S1/B1S2 = 43/49 °C e slots
> não suportados em `28` (0 °C).

## 6. Dados de identidade e diagnóstico

| Dado | Comando | Resposta | Decodificação | Status aqui |
|---|---|---|---|---|
| VIN | `0902` | `49 02 01` + 17 ASCII | juntar frames e ler ASCII | ✅ |
| Calibration ID | `0904` | `49 04` + contador + IDs | ASCII, blocos de 16 | ✅ bruto |
| CVN | `0906` | `49 06` + contador + CVNs | 4 bytes hex por CVN | ✅ bruto |
| Nome da ECU | `090A` | `49 0A` + contador + ASCII | até 20 chars | ✅ `ECM-EngineControl` |
| Norma OBD | `011C` | 1 byte | enum §8.1 | ✅ 29 = OBDBr-2 |
| Tipo de combustível | `0151` | 1 byte | enum §8.2 | ✅ 3 = Etanol |
| DTCs armazenados | `03` | `43` + pares | 2 bytes por DTC | ✅ nenhum |
| DTCs pendentes | `07` | `47` + pares | 2 bytes por DTC | ✅ nenhum |
| DTCs permanentes | `0A` | `4A` + pares | 2 bytes por DTC | ✅ nenhum |
| Readiness | `0101`, `0141` | 4 bytes | bitmask §8.3 | ✅ |
| Freeze frame | `0202`, `02XX` | `42` | mesma escala do Mode 01 | ❌ NO DATA |
| Testes Mode 06 | `0600`, `0601` | `46` | TID/MID + valores | ✅ bruto |
| IPT/IUPR | `0908` (spark) / `090B` (diesel) | `49 08` / `49 0B` | contadores/razões de uso | ✅ suporte, ❌ não coletado |
| Info do adaptador | `ATI`, `AT@1`, `ATDP`, `ATRV`, `ATIGN` | ASCII | — | parcial (`ATI` = `ELM327 v2.1`) |

### 6.1 O que são VIN, Cal ID, CVN e nome da ECU

Esses quatro **não são sensores**: são dados de identidade do veículo e do
software da ECU.

| Sigla | Nome | O que é | Para que serve |
|---|---|---|---|
| **VIN** | Vehicle Identification Number | Número de 17 caracteres gravado no chassi | Identifica o veículo; usado em documentos, seguro e peças |
| **Cal ID / CAL** | Calibration Identification | Identificador ASCII da calibração de software da ECU | Saber qual versão de software/calibração está gravada |
| **CVN** | Calibration Verification Number | Checksum de 4 bytes de cada calibração | Verificar se a calibração é original/homologada; emissões |
| **ECU name** | Electronic Control Unit name | Nome ASCII do módulo (ex.: `ECM-EngineControl`) | Identificar qual módulo respondeu |
| **DTC** | Diagnostic Trouble Code | Código de falha (ex.: `P0133`) | Diagnosticar problemas |
| **MIL** | Malfunction Indicator Lamp | Luz de anomalia no painel (“check engine”) | Indica falha ativa |
| **Readiness** | Readiness monitors | Monitores internos completos ou não | Base para inspeção de emissões |

Analogia rápida:

- **VIN** = “RG” do carro.
- **Cal ID** = versão do software/calibração da ECU.
- **CVN** = “hash/checksum” dessa calibração; serve para conferir se o software
  é o original homologado.
- **ECU name** = nome do módulo que respondeu (ex.: `ECM-EngineControl`).

### 6.2 Decodificação de DTC

Cada DTC usa 2 bytes. Exemplo `01 33`:

```text
A = 0x01 = 0000 0001
B = 0x33 = 0011 0011
```

- A7–A6: tipo → `00`=P (Powertrain), `01`=C (Chassis), `10`=B (Body),
  `11`=U (Network).
- A5–A4: primeiro dígito.
- A3–A0: segundo dígito.
- B7–B4: terceiro dígito.
- B3–B0: quarto dígito.

`01 33` → **P0133** (resposta lenta do sensor de O2, banco 1 sensor 1).

### 6.3 Readiness (PID 01 / 41)

Byte A do PID 01:

- A7 = MIL ligada/desligada.
- A6–A0 = quantidade de DTCs confirmados.

Byte B:

- B7 reservado.
- B6–B4 = completude dos testes comuns (Componentes, Fuel System, Misfire).
- B3 = tipo de ignição (0=faísca, 1=compressão).
- B2–B0 = disponibilidade dos testes comuns.

Bytes C/D = disponibilidade/completude dos testes específicos do tipo de motor
(catalisador, O2, EVAP, EGR/VVT, etc.). Para gasolina/etanol, os bits comuns
são:

| Teste | Disponível | Completo |
|---|---|---|
| Componentes | B2 | B6 |
| Fuel system | B1 | B5 |
| Misfire | B0 | B4 |
| Catalisador | C0 | D0 |
| Heated catalyst | C1 | D1 |
| EVAP | C2 | D2 |
| Secondary air | C3 | D3 |
| O2 sensor | C5 | D5 |
| O2 heater | C6 | D6 |
| EGR/VVT | C7 | D7 |

> Para cada teste: bit de **disponibilidade** em `1` = disponível; bit de
> **completude** em `0` = completo. Em `41` a lógica é a mesma, mas A é
> sempre 0.

## 7. Tabela de siglas — geral

| Sigla | Significado | Português |
|---|---|---|
| OBD | On-Board Diagnostics | Diagnóstico de bordo |
| OBD-II | Second-generation OBD | OBD de 2ª geração |
| EOBD | European OBD | OBD europeu |
| JOBD | Japanese OBD | OBD japonês |
| KOBD | Korean OBD | OBD coreano |
| IOBD | India OBD | OBD indiano |
| OBDBr | Brazil OBD | OBD brasileiro |
| WWH-OBD | World-Wide Harmonized OBD | OBD harmonizado mundial |
| PID | Parameter ID | Identificador de parâmetro |
| DTC | Diagnostic Trouble Code | Código de falha |
| MIL | Malfunction Indicator Lamp | Luz de falha |
| CEL | Check Engine Light | Luz de verificação do motor |
| ECU | Electronic Control Unit | Unidade eletrônica de controle |
| ECM | Engine Control Module | Módulo de controle do motor |
| PCM | Powertrain Control Module | Módulo do trem de força |
| TCM | Transmission Control Module | Módulo do câmbio |
| BCM | Body Control Module | Módulo da carroceria |
| ABS | Anti-lock Braking System | Sistema antibloqueio |
| SRS | Supplemental Restraint System | Airbag |
| VIN | Vehicle Identification Number | Número de identificação do veículo |
| CALID / CAL | Calibration Identification | Identificação da calibração |
| CVN | Calibration Verification Number | Número de verificação da calibração |
| CAN | Controller Area Network | Rede CAN |
| ISO-TP | ISO 15765-2 Transport Protocol | Protocolo de transporte CAN |
| UDS | Unified Diagnostic Services | Serviços unificados de diagnóstico |
| BLE | Bluetooth Low Energy | Bluetooth de baixa energia |
| GATT | Generic Attribute Profile | Perfil de atributos BLE |
| MTU | Maximum Transmission Unit | Unidade máxima de transmissão |
| RSSI | Received Signal Strength Indicator | Indicador de força do sinal |
| TID | Test ID | ID de teste (Mode 06) |
| MID | Monitor ID | ID de monitor (Mode 06) |
| OBDMID | OBD Monitor ID | ID de monitor OBD |
| AECD | Auxiliary Emission Control Device | Dispositivo auxiliar de controle de emissões |
| NTE | Not-To-Exceed | Limite de emissão “não exceder” |
| PTO | Power Take-Off | Tomada de força |
| FF | Freeze Frame | Quadro congelado no DTC |
| IPT | In-Use Performance Tracking | Rastreamento de desempenho em uso |
| IUPR | In-Use Performance Ratio | Razão de desempenho em uso |

## 8. Tabelas de enumeração

### 8.1 PID `1C` — norma OBD

| Valor | Norma |
|---|---|
| 1 | OBD-II CARB |
| 2 | OBD EPA |
| 3 | OBD + OBD-II |
| 4 | OBD-I |
| 5 | Não conforme OBD |
| 6 | EOBD |
| 7 | EOBD + OBD-II |
| 8 | EOBD + OBD |
| 9 | EOBD + OBD + OBD-II |
| 10 | JOBD |
| 11 | JOBD + OBD-II |
| 12 | JOBD + EOBD |
| 13 | JOBD + EOBD + OBD-II |
| 14 | OBD + EOBD + KOBD |
| 15 | OBD + OBD-II + EOBD + KOBD |
| 17 | EMD |
| 18 | EMD+ |
| 19 | HD OBD-C |
| 20 | HD OBD |
| 21 | WWH OBD |
| 23–26 | HD EOBD I/II |
| 27 | Heavy Duty ZEV |
| 28 | OBDBr-1 |
| 29 | **OBDBr-2 (este carro)** |
| 30 | KOBD |
| 31 | IOBD I |
| 32 | IOBD II |
| 33 | HD EOBD-IV |
| 34 | OBD + OBD-II + HD OBD |
| 35 | OBDBr-3 |

### 8.2 PID `51` — tipo de combustível

| Valor | Combustível |
|---|---|
| 0 | Não disponível |
| 1 | Gasolina |
| 2 | Metanol |
| 3 | **Etanol (este carro)** |
| 4 | Diesel |
| 5 | LPG |
| 6 | CNG |
| 7 | Propano |
| 8 | Elétrico |
| 9 | Bifuel gasolina |
| 10 | Bifuel metanol |
| 11 | Bifuel etanol |
| 12 | Bifuel LPG |
| 13 | Bifuel CNG |
| 14 | Bifuel propano |
| 15 | Bifuel eletricidade |
| 16 | Bifuel elétrico + combustão |
| 17 | Híbrido gasolina |
| 18 | Híbrido etanol |
| 19 | Híbrido diesel |
| 20 | Híbrido elétrico |
| 23 | Bifuel diesel |

### 8.3 PID `03` — status do sistema de combustível

Bitmask por banco (A = banco 1, B = banco 2):

| Bit | Significado |
|---|---|
| 0 | Open loop |
| 1 | Closed loop |
| 2 | Closed loop com falha no O2 |
| 3 | Open loop por carga do motor |
| 4 | Open loop por temperatura alta |
| 5 | Open loop por velocidade |
| 6 | Open loop por throttle |
| 7 | Reservado |

## 9. Tabela de siglas — sensores e grandezas

| Sigla | Inglês | Português | O que mede |
|---|---|---|---|
| **ECT** | Engine Coolant Temperature | Temperatura do líquido de arrefecimento | °C |
| **IAT** | Intake Air Temperature | Temperatura do ar admitido | °C |
| **MAF** | Mass Air Flow | Fluxo de ar por massa | g/s |
| **MAP** | Manifold Absolute Pressure | Pressão absoluta do coletor | kPa |
| **TPS** | Throttle Position Sensor | Posição da borboleta | % |
| **APP** | Accelerator Pedal Position | Posição do pedal do acelerador | % |
| **VSS** | Vehicle Speed Sensor | Sensor de velocidade | km/h |
| **O2 / HO2S** | Oxygen Sensor / Heated O2 Sensor | Sonda lambda (aquecida) | V, % |
| **WRAF** | Wide Range Air-Fuel | Sonda lambda de banda larga | λ |
| **AFR** | Air-Fuel Ratio | Relação ar/combustível | — |
| **λ** | Lambda / equivalence ratio | Lambda | 1,0 = estequiométrico |
| **STFT** | Short Term Fuel Trim | Correção rápida de combustível | % |
| **LTFT** | Long Term Fuel Trim | Correção aprendida de combustível | % |
| **FRP** | Fuel Rail Pressure | Pressão do trilho de combustível | kPa |
| **EGT** | Exhaust Gas Temperature | Temperatura dos gases de escape | °C |
| **CATT** | Catalyst Temperature | Temperatura do catalisador | °C |
| **EVAP** | Evaporative Emission Control | Controle de vapores do tanque | % / Pa |
| **EGR** | Exhaust Gas Recirculation | Recirculação de gases de escape | % |
| **VVT** | Variable Valve Timing | Fase variável do comando | — |
| **AIR** | Secondary Air Injection | Injeção de ar secundária | — |
| **CACT** | Charge Air Cooler Temperature | Temperatura do intercooler | °C |
| **DPF** | Diesel Particulate Filter | Filtro de partículas diesel | — |
| **GPF** | Gasoline Particulate Filter | Filtro de partículas gasolina | — |
| **SCR** | Selective Catalytic Reduction | Redução catalítica seletiva (ARLA/AdBlue) | — |
| **NOx** | Nitrogen Oxides | Óxidos de nitrogênio | ppm |
| **PM** | Particulate Matter | Material particulado | — |
| **NMHC** | Non-Methane Hydrocarbons | Hidrocarbonetos não-metano | — |
| **TDC** | Top Dead Center | Ponto morto superior | ° BTDC |
| **BARO** | Barometric Pressure | Pressão barométrica | kPa |
| **B1S1** | Bank 1 Sensor 1 | Banco 1, sensor 1 | — |
| **B2S2** | Bank 2 Sensor 2 | Banco 2, sensor 2 | — |

### 9.1 Termos que aparecem nas tabelas

| Termo | Significado |
|---|---|
| **Banco 1 / Bank 1** | Fileira de cilindros que inclui o cilindro nº 1 |
| **Sensor 1** | Sonda antes do catalisador (upstream) |
| **Sensor 2** | Sonda depois do catalisador (downstream) |
| **Closed loop** | ECU usando a sonda lambda para corrigir a mistura |
| **Open loop** | ECU sem realimentação da sonda (partida/frio) |
| **Estequiométrico** | Mistura ideal; λ = 1,0 |
| **Rich** | Mistura rica (excesso de combustível) |
| **Lean** | Mistura pobre (excesso de ar) |
| **Readiness** | Monitores internos que precisam completar antes da inspeção |
| **Freeze frame** | “Foto” dos dados no momento em que o DTC foi gravado |
| **DTC confirmado** | Falha já confirmada pela ECU |
| **DTC pendente** | Falha detectada, aguardando confirmação |
| **DTC permanente** | Falha que não pode ser apagada por `04`; some após reparo/ciclos |

## 10. Unidades

| Unidade | Significado |
|---|---|
| `rpm` | rotações por minuto |
| `km/h` | quilômetros por hora |
| `°C` | graus Celsius |
| `%` | porcentagem |
| `kPa` | quilopascal (1 kPa = 0,01 bar) |
| `Pa` | pascal |
| `g/s` | gramas por segundo |
| `V` | volt |
| `A` | ampère |
| `Nm` | newton-metro |
| `L/h` | litros por hora |
| `λ` | lambda (razão de equivalência) |
| `ppm` | partes por milhão |
| `mg/stroke` | miligramas por curso |
| `s` / `min` / `h` | segundo / minuto / hora |
| `km` | quilômetro |

## 11. Processos passo a passo

### 11.1 Conectar no adaptador

1. `adapter.Enable()`, `SetScanType(Active)`.
2. Procurar `LocalName()` contendo `OBD`/`ELM` ou `HasServiceUUID(FFF0)`.
3. `adapter.Connect(addr, ConnectionParams{})`.
4. `DiscoverServices([]UUID{FFF0})`.
5. `DiscoverCharacteristics([]UUID{FFF1, FFF2})`.
6. `FFF1.EnableNotifications(callback)`.
7. Escrever comandos em `FFF2` com `WriteWithoutResponse`.

### 11.2 Handshake

```text
ATZ
ATE0
ATL0
ATS0
ATSP0
```

### 11.3 Descobrir PIDs

```text
0100
0120
0140
0160
0180
01A0
01C0
01E0
```

Parar quando o último bit for 0 ou vier `NO DATA`.

### 11.4 Ler um PID ao vivo

```text
01XX
```

Exemplo RPM:

```text
TX: 010C
RX: 410C1AF8
RPM = (0x1A*256 + 0xF8)/4 = 1726 rpm
```

### 11.5 Ler VIN

```text
TX: 0902
RX (multi-frame): 49 02 01 <17 bytes ASCII>
```

Juntar as linhas `0:`, `1:`, `2:` e ignorar o prefixo `49 02 01`.

> O VIN deste carro foi lido com sucesso, mas não foi gravado no repositório
> por ser um dado identificador do veículo.

### 11.6 Ler Cal ID e CVN

```text
TX: 0904   -> Calibration IDs (ASCII, blocos de 16)
TX: 0906   -> CVNs (4 bytes hex cada)
```

A quantidade de CVNs deve casar com a quantidade de Cal IDs. Neste carro os
dois comandos retornaram dados multi-frame brutos; falta montar/formatar os
valores finais. O `0900` também indica suporte a `0908`/`090B` (IPT/IUPR),
ainda não coletados.

### 11.7 Ler nome da ECU

```text
TX: 090A
RX: 49 0A 01 <ASCII>
```

Neste carro: `ECM-EngineControl`.

### 11.8 Ler DTCs

```text
TX: 03   -> DTCs armazenados
TX: 07   -> DTCs pendentes
TX: 0A   -> DTCs permanentes
```

Resposta: `43`/`47`/`4A` + pares de 2 bytes. `00 00` = nenhum DTC.

### 11.9 Apagar DTCs (⚠️ não implementar sem confirmação)

```text
TX: 04
```

Apaga DTCs, desliga MIL e zera readiness. Não fazer em movimento nem antes de
inspeção.

### 11.10 Mode 06 (testes de monitores)

```text
TX: 0600   -> TIDs suportados
TX: 0601   -> resultados do TID 01
```

A resposta é multi-frame e contém TID, componente, valor, mínimo e máximo.
A decodificação exige a tabela SAE J1979-DA ou documentação do fabricante.

## 12. Taxa e escalonamento

| Cenário | Tempo | Taxa |
|---|---|---|
| 1 PID | ~140–170 ms | 5–7 Hz |
| 3 PIDs | ~0,5 s | ~2 Hz cada |
| 10 PIDs | ~1,7 s | ~0,6 Hz cada |
| 43 PIDs | 7,2 s | 0,14 Hz |
| 43 PIDs + VIN/Cal/CVN/DTCs | ~10 s | 0,1 Hz |

Sugestão de scheduler:

```text
rápido:  RPM, velocidade, throttle, carga        1,5–2 Hz cada
médio:   ECT, IAT, STFT, LTFT, MAP, MAF, lambda  0,5–1 Hz
lento:   nível de combustível, distância, warmups 0,1–0,2 Hz
estático: VIN, Cal ID, CVN, DTCs, readiness      1× por sessão
```

### 12.1 Litros no tanque

O OBD2 **não entrega litros**: o PID `2F` entrega apenas a porcentagem
(`A*100/255`). Para litros:

```text
litros = (raw_2F / 255) * capacidade_do_tanque
```

No `poc/obd2/obd-display`:

```go
const tankCapacityDeciL = 440 // 44,0 L
litersX10 := raw * tankCapacityDeciL / 255
```

Ajuste `440` para a capacidade real do seu carro. Exemplo lido: `raw = 149`
→ `58,4%` → `25,7 L` com tanque de 44 L.

### 12.2 Páginas da UI

| Página | Dados (máx. 3) | Fonte |
|---|---|---|
| 1 | velocidade, RPM, combustível (L) | `010D`, `010C`, `012F` |
| 2 | ECT, IAT, catalisador | `0105`, `010F`, `013C` |
| 3 | % álcool, % gasolina, tipo | `0152`, derivado, `0151` |

Botões: **A** = próxima, **B** = anterior. A tela usa framebuffer 4bpp
(16,2 KB) e envia ao LCD somente as linhas alteradas, evitando flicker e
ghosting. O layout é calculado por margens/gaps e pela largura real dos textos
(`tinyfont.LineWidth`), com os valores centralizados nas células. Na página 3,
o tipo mostra `FLEX` quando o percentual de etanol está entre 1% e 99%.

## 13. Snapshot de exemplo (motor desligado)

| PID | Raw | Decodificado |
|---|---|---|
| `0101` | `00 07 E1 00` | MIL off, 0 DTCs, ignição por faísca |
| `0104` | `00` | 0 % de carga |
| `0105` | `73` | 75 °C |
| `0106` | `80` | 0 % STFT |
| `0107` | `7B` | -3,9 % LTFT |
| `010B` | `5F` | 95 kPa MAP |
| `010C` | `00 00` | 0 rpm |
| `010D` | `00` | 0 km/h |
| `010E` | `D5` | 42,5 ° BTDC |
| `010F` | `53` | 43 °C IAT |
| `0110` | `00 00` | 0 g/s MAF |
| `0111` | `50` | 31,4 % TPS |
| `011C` | `1D` | 29 = OBDBr-2 |
| `012F` | `95` | 58,4 % tanque |
| `0131` | `00 4D` | 77 km desde limpeza |
| `0133` | `5F` | 95 kPa baro |
| `013C` | `15 54` | 506 °C catalisador |
| `0142` | `30 FB` | 12,539 V |
| `0145` | `36` | 21,2 % throttle relativo |
| `0146` | `42` | 26 °C ambiente |
| `0147` | `50` | 31,4 % throttle B |
| `0149` | `31` | 19,2 % pedal D |
| `014A` | `1A` | 10,2 % pedal E |
| `0151` | `03` | Etanol |
| `0152` | `62` | 38,4 % etanol |
| `0167` | `01 73 00` | ECT sensor 1 = 75 °C |
| `0168` | `03 53 59 28 28 28 28` | IAT B1S1/B1S2 = 43/49 °C; demais slots em 0 °C |

> Valores variam com motor ligado/desligado, temperatura e combustível.

## 14. Lacunas e próximos passos

1. **Mode 06**: formato já identificado (blocos de 9 bytes: MID/TID/UAS/valor/mín/máx);
   falta a captura completa e os nomes OEM dos TIDs.
2. **Freeze frame**: o carro respondeu `NO DATA`; não há snapshot disponível.
3. **PIDs do fabricante** (`21`/`22` ou UDS): podem revelar câmbio, bateria,
   temperatura de óleo etc., mas exigem definições específicas do modelo.
4. **Sniffing CAN passivo**: exigiria transceiver CAN (MCP2515/TJA1050) e
   engenharia reversa dos IDs; o ELM327 só entrega o que é pedido.
5. **DTCs e readiness na tela**: próximos dados estáticos a adicionar à UI.
6. **Scheduler por prioridade**: implementar para atualizar rápido sem perder
   os lentos.
