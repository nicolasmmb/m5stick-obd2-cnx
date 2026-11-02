# Ideias de projetos de ML — M5StickC Plus2 + OBD2

> Status: **ideias e hipóteses**, não implementado.
> Base: 43 PIDs Mode 01, dados de identidade (VIN/CVN/Cal ID), DTCs e
> readiness já capturados com o ELM327 BLE `OBDII`.
>
> Hoje temos uma sessão curta, com motor desligado/em marcha lenta e sem
> labels. Isso é suficiente para **modelagem de dados e protótipos**, mas
> **não** para treinar um modelo supervisionado confiável.

## 1. Premissas

- O OBD2 entrega ~43 PIDs, mas a taxa é compartilhada: ~5–7 Hz para um PID,
  ~0,14 Hz para varrer todos.
- Não temos km/L, marcha, temperatura de óleo, pressão de pneus nem câmbio.
- Litros no tanque e % gasolina são **derivados**, não medidos.
- Uma única ECU/carro limita a generalização.
- O M5Stick já tem IMU e microfone; GPS e CAN sniffing exigem hardware extra.

## 2. Modelagem de dados

### 2.1 Tabelas principais

```text
obd_sample
- session_id, trip_id, vehicle_id, adapter_id
- ts_utc, ts_monotonic
- pid, raw_hex, value, unit
- quality: OK | TIMEOUT | STALE | ENGINE_OFF | OUT_OF_RANGE | UNSUPPORTED
- latency_ms, sample_rate_hz
```

```text
obd_features_1s
- trip_id, ts
- rpm, speed, coolant, intake, throttle, load
- map_kpa, maf_gps, stft_b1, ltft_b1, lambda
- fuel_level_pct, catalyst_temp_c, module_voltage
- rpm_roll_5s_mean, rpm_roll_30s_std
- speed_delta, accel_ms2, jerk
- idle_ratio_30s, throttle_std_30s
- engine_on, quality_score
```

```text
trip
- trip_id, driver_id, vehicle_id
- started_at, ended_at, distance_km
- road_type, weather, ac_on, passengers
```

```text
fuel_fill
- trip_id, liters, odometer_km, price, ethanol_pct
```

```text
dtc_event
- trip_id, code, status, timestamp, freeze_frame
```

```text
driving_event
- trip_id, type (hard_brake, hard_accel, idle, high_rpm)
- start_ts, end_ts, severity
```

```text
model_run
- model_id, version, target, features
- train_period, validation_period
- metrics, params, artifact_path
```

### 2.2 Regras de qualidade

- Separar leituras com motor desligado (`engine_on = false`).
- Não interpolar PIDs rápidos (RPM, velocidade, throttle).
- Forward-fill apenas PIDs lentos (temperatura, nível de combustível).
- Guardar `quality`; `-40 °C` ou `TIMEOUT` não é dado válido.
- Split de treino/validação **por viagem**, nunca por linha.
- Hash/anonimização para VIN e GPS.

## 3. Projetos com OBD + M5Stick atual

| Projeto | O que faz | Modelo/ML | Dificuldade |
|---|---|---|---|
| **Consumo km/L e L/100 km** | Estima consumo e autonomia | Física + Gradient Boosting/Random Forest | Média |
| **Detector de abastecimento** | Percebe salto no tanque e registra litros | Regras + change point | Baixa |
| **Score de direção** | Aceleração/frenagem forte, RPM alto, marcha lenta | Regras + clustering/classificação | Baixa |
| **Saúde da bateria/alternador** | Tensão vs RPM, partida, queda em marcha lenta | Regressão + change point | Baixa |
| **Cold start advisor** | Tempo de aquecimento e carga | Regressão/classificação | Baixa |
| **Start-stop advisor** | Detecta paradas longas | Regras + HMM | Baixa |
| **Marcha virtual** | Estima marcha por RPM/velocidade | HMM/classificação | Média |
| **Coach de faixa de potência** | RPM/carga ideais para economia | Regras + otimização | Baixa |
| **Monitor de sensores** | ECT/IAT/O2/fuel trim fora do padrão | Isolation Forest, Mahalanobis | Média |
| **Eficiência do catalisador** | Compara O2 antes/depois do catalisador | Regressão/anomalia | Média |
| **Detector de misfire** | RPM, O2, fuel trims e microfone | Anomalia + classificação | Média/alta |
| **Preditor de DTC** | Antecipa falha por sinais precursores | GBM + survival analysis | Alta |
| **Vida do óleo** | Partidas a frio, runtime e temperatura | Regressão | Média |
| **Validação pós-reparo** | Baseline antes/depois | Change point | Baixa |
| **Aquisição adaptativa** | Escolhe PIDs dado o limite de ~7 Hz | Contextual bandit | Alta |
| **Black box de eventos** | Grava frenagens/acelerações fortes | Regras + detecção de eventos | Baixa |

## 4. Com o IMU do M5Stick

| Projeto | Dados | Modelo |
|---|---|---|
| **Detector de buracos/rua ruim** | IMU + GPS | Anomalia, CNN 1D/TSFresh + GBM |
| **Score de conforto** | IMU + velocidade | Clustering/regressão |
| **Identificação do motorista** | Estilo de acelerar/frear/curvar | Classificação (RF/GBM) |
| **Estimativa de carga/passageiros** | Aceleração vs RPM/carga | Regressão |
| **Detecção de curva agressiva** | IMU + velocidade | Regras + classificação |
| **Detecção de reboque/carga pesada** | IMU + load/RPM | Classificação |
| **Detecção de impacto/acidente** | IMU | Anomalia/limiar |
| **Assinatura de vibração do motor** | IMU + RPM | FFT + anomalia |
| **Desbalanceamento de pneu** | IMU + velocidade | FFT + classificação |

## 5. Com GPS / celular

| Projeto | O que entrega | Modelo |
|---|---|---|
| **Previsão de autonomia** | “Dá para chegar?” com incerteza | Regressão quantílica |
| **Eco-routing** | Rota com menor consumo | Otimização + GBM |
| **Classificação de trajeto** | cidade/estrada/trânsito | Clustering/classificação |
| **Efeito da altitude** | Consumo vs altitude/temperatura | Regressão |
| **Score de risco/seguro** | Telemetria para seguro | GBM + calibração |
| **Mapa colaborativo de buracos** | IMU + GPS + upload | Clustering geoespacial |
| **Limite de velocidade** | GPS + mapas | Regras + classificação |
| **Previsão de tempo de viagem** | Histórico de rotas | Gradient Boosting/séries temporais |

## 6. Com microfone

| Projeto | O que detecta | Modelo |
|---|---|---|
| **Misfire/knock** | Batida, falha de combustão | MFCC + CNN/GBM |
| **Correia/rolamento** | Ruído anormal | Autoencoder de áudio |
| **Qualidade da rua** | Ruído + IMU | Classificação |
| **Assistente de diagnóstico** | “Que barulho é esse?” | LLM/RAG + áudio |

## 7. Com CAN sniffing

Exige MCP2515/TJA1050 ou similar.

| Projeto | O que ganha | Modelo |
|---|---|---|
| **Engenharia reversa do CAN** | Sinais não-OBD | Clustering/descoberta |
| **Temperatura do câmbio** | Se estiver no barramento | Regressão |
| **Dados de ABS/TPMS** | Rodas, pressão, velocidade | Fusão de sensores |
| **Anomalia no CAN** | Cybersecurity automotiva | Autoencoder/Isolation Forest |
| **Gêmeo digital do carro** | Simulação com dados reais | Modelo físico + ML |
| **Injeção/ignição** | Sinais brutos | Classificação de séries temporais |

## 8. Algoritmos por problema

| Problema | Algoritmos | Quando usar |
|---|---|---|
| Anomalia/falha | Isolation Forest, Mahalanobis, LOF, Autoencoder, CUSUM/ruptures | Primeiro projeto de ML; não precisa de labels |
| Consumo/autonomia | Física + Random Forest/Gradient Boosting; ARIMA/SARIMAX; LightGBM/XGBoost | Precisa de viagens e abastecimentos |
| Estilo de direção | Random Forest, Gradient Boosting, Regressão Logística, SVM | Precisa de labels |
| Agrupamento de viagens | K-Means, DBSCAN/HDBSCAN, GMM | Sem labels; exploração |
| Séries complexas | LSTM/GRU, Temporal CNN, Transformer | Só com milhares de sequências |
| Estado oculto | Kalman, HMM | Combustível, marcha, estado de condução |
| Falha ao longo do tempo | Survival Analysis | Tempo até DTC/manutenção |
| Aquisição adaptativa | Contextual Bandit | Escolher PIDs dado o limite de Hz |

**Recomendação:** começar com baseline físico + modelos tabulares clássicos.
Deep learning só depois de volume e labels.

## 9. Roadmap sugerido

### Fase 0 — Logger e schema

- Salvar `obd_sample` em formato long.
- Flags de qualidade e `engine_on`.
- Exportar CSV/Parquet.
- Separar sessões/viagens.

### Fase 1 — Analytics sem ML

- Consumo por viagem.
- Tempo de marcha lenta.
- Temperaturas e fuel trims.
- Outliers simples.

### Fase 2 — Coletar 20–50 viagens

- Horários, trânsito e estrada.
- Anotar abastecimentos.
- Anotar DTCs/manutenção.
- Se possível, 2 motoristas.

### Fase 3 — ML não supervisionado

- Isolation Forest para anomalias.
- Clustering de viagens.
- Change point.

### Fase 4 — ML supervisionado

- Consumo.
- Estilo de direção.
- Risco de DTC.

## 10. Cuidados

- Split por viagem, não por linha.
- Não usar litros/% gasolina como ground truth.
- Tratar motor desligado separadamente.
- Guardar a taxa real de cada PID.
- Uma ECU/carro limita generalização.
- VIN e GPS são dados sensíveis.

## 11. Próximos passos

1. Definir o schema final (SQL/Parquet).
2. Implementar o logger com qualidade e viagens.
3. Escolher o primeiro MVP:
   - consumo/autonomia;
   - score de direção;
   - manutenção preditiva.
4. Coletar dados rotulados.
5. Só então treinar o primeiro modelo.
