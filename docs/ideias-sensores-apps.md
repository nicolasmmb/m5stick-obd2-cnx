# Ideias de sensores e apps — M5StickC Plus2 + TinyGo

Catálogo de ideias para aproveitar o M5StickC Plus2, os sensores M5Stack e o
projeto OBD2. Atualizado em 2026-11-01.

## O que já está pronto no repositório

| Recurso | POC/lib |
|---|---|
| TinyGo 0.44 + flash + serial | `poc/hello` |
| Wi-Fi (scan) | `poc/wifi-scan` |
| BLE (scan, nomes, RSSI) | `poc/ble-scan` |
| Tela ST7789 + abas + botões | `poc/display-scan` |
| Menu de sensores na tela | `poc/sensors` |
| IMU MPU6886 (g, °/s, temperatura) | `poc/sensors` |
| RTC BM8563 | `poc/sensors` |
| Bateria (ADC) | `poc/sensors` |
| Buzzer PWM | `poc/sensors` |
| LED/IR (G19) | `poc/sensors` |
| Microfone PDM SPM1423 | `lib/spm1423`, `poc/mic-pdm` |
| Espectro FFT / waterfall | `poc/sensors` |
| Bola com física do IMU | `poc/sensors` |
| Areia com física do IMU | `poc/sensors` |
| Gopls no Zed | `.zed/settings.json`, `scripts/gopls-tinygo.sh` |

## ⚡ Quick wins (1–2 dias)

| Ideia | Sensores/periféricos | Por que é legal |
|---|---|---|
| G-meter | IMU | G lateral/longitudinal, pico e histórico; track day |
| 0–100 km/h / frenagem | IMU + OBD2 (velocidade) | Cronometra 0–60, 0–100, 400 m e frenagem |
| Nível/inclinômetro | IMU + buzzer | Bolha eletrônica que apita quando nivelado |
| Detector de impacto | IMU + buzzer/LED | Alarme se baterem/levantarem o carro ou a placa |
| Medidor de som | Microfone + FFT | dBFS/SPL, alerta de ruído, palma/assobio |
| Relógio/alarme/Pomodoro | RTC + buzzer | Alarmes, timer, cronômetro, Pomodoro |
| Monitor de bateria | ADC | Tensão, %, alerta de bateria fraca, autonomia |
| Controle IR | LED/IR G19 | Controlar TV, ar-condicionado, som (NEC) |
| Scanner BLE | BLE | Achar fone/relógio/celular, RSSI, proximidade |
| Analisador Wi-Fi | Wi-Fi | Gráfico de canais, RSSI, redes abertas |
| Jogos | botões + IMU + buzzer | Pong com inclinação, Snake, labirinto, dado |

## 🚗 Apps para o carro (com OBD2)

| Ideia | O que faz |
|---|---|
| Dashboard OBD2 | RPM, velocidade, temperatura, carga, acelerador, admissão, fuel trims |
| Scanner de DTC | Lê e apaga códigos (modo 03/04), monitores de prontidão |
| Computador de bordo | Consumo médio/instantâneo, autonomia, distância, médias |
| Telemetria de track day | OBD2 + IMU + GPS: G-G, mapa, tempos de volta, delta |
| Driver score | Detecta aceleração/freada/curva brusca e gera nota |
| Anti-furto / reboque | IMU detecta movimento; buzzer + notificação Wi-Fi/BLE |
| Monitor de TPMS | Lê sensores BLE de pressão dos pneus |
| Análise de som do motor | Mic + FFT para batida/misfire (experimental) |
| 0–100 / arrancada | Velocidade OBD2 + IMU |
| Dashboard web | M5 cria Wi-Fi AP e serve gauges no celular |
| MQTT / Home Assistant | Publica RPM, temperatura, bateria e eventos |

### Bloqueio atual do OBD2

O ELM327 **Bluetooth Classic** não é suportado pelo TinyGo/espradio (só BLE).
Para o dashboard OBD2, usar:

1. adaptador OBD2 **BLE** (recomendado);
2. adaptador OBD2 **Wi-Fi** (TCP `192.168.0.10:35000`);
3. ponte **HC-05** mestre na UART do M5 (reaproveita o Classic).

## 🌡️ Ambiente / qualidade do ar

| Sensor | Ideias |
|---|---|
| ENV III / ENV Pro (temp/umidade/pressão) | Clima da cabine, altitude, estação meteorológica, janela aberta, compensar leituras |
| CO₂ (SCD40/SCD41) | Ar viciado na cabine, sonolência, acionar ventilação |
| PM2.5 | Eficiência do filtro de ar, qualidade do ar, alerta de poluição |
| Gás (MQ-2/7/135, TVOC) | Fumaça, CO, vapor de combustível; alarme de segurança |
| Luz/UV | Brilho automático da tela, farol alto, incidência solar |
| Cor (TCS34725) | Cor da luz ambiente, semáforo (experimental) |

## 📏 Movimento / distância / presença

| Sensor | Ideias |
|---|---|
| PIR / mmWave / radar | Anti-furto, presença no carro, acordar a tela, contar pessoas |
| ToF / ultrassônico / LiDAR | Estacionamento, obstáculo, nível de líquido |
| IMU Pro / magnetômetro | Bússola, heading, G-meter, drift, inclinação |
| Vibração | Batida de motor, desbalanceamento, impacto |
| Encoder / joystick / teclado | Menu, volume, controle de gauges |

## 🌡️ Temperatura / térmica

| Sensor | Ideias |
|---|---|
| IR temp (MLX90614) | Freio/disco, escape, eletrônica, cabine |
| Câmera térmica (MLX90640) | Pontos quentes no motor, freios, HVAC |
| Termopar (MAX31855) | EGT, cabeçote, temperatura de freio |
| DS18B20 | Água, óleo, ar ambiente, coolant |

## ⚡ Elétrico / carro

| Sensor | Ideias |
|---|---|
| INA226/INA219 | Corrente/tensão da bateria e alternador, consumo, fuga parasita |
| ADC/ADS1115 | Divisores de tensão, sensores analógicos |
| Relé / optoacoplador | Ventoinha, luzes, buzina, bomba |
| CAN / RS485 | Dados reais do carro, BMS, Modbus |
| SD card | Data logger, gravação de áudio, logs de viagem |
| GPS | Velocidade, voltas, viagem, geofence, telemetria |
| RFID / NFC / biometria | Identificação do motorista, acesso, perfil por condutor |
| 4G / LoRa | Rastreamento remoto, telemetria off-grid |

## 🏠 IoT / casa

- Nó MQTT: publica IMU, temperatura, bateria e botões; recebe comandos.
- Relógio NTP / clima: sincroniza hora e mostra previsão (HTTP).
- Beacon BLE: iBeacon/Eddystone para rastrear objetos.
- Sensor de planta/água: ADC + boia/eletrodos; alerta via Wi-Fi.
- Botão inteligente: aciona cenas da casa (MQTT/HTTP).
- Alarme de porta/janela: IMU ou reed switch + buzzer.
- Monitor de som do ambiente: nível de ruído para bebê/escritório.
- Estação meteorológica: ENV + vento/chuva + MQTT.
- Automação: relé + sensores + Home Assistant.

## 🔬 Avançado / experimental

- Controle remoto BLE HID: mouse/teclado/apresentador.
- Osciloscópio: ADC + framebuffer para sinais de baixa frequência.
- Gerador de sinais: PWM/saída digital para testar circuitos.
- Gravador de áudio: mic → buffer/SD; reproduzir em speaker externo.
- Jogos multiplayer: dois M5 via BLE/Wi-Fi.
- GPS + lap timer: mapa de voltas e speed trap.
- Data logger: OBD2 + IMU + GPS em CSV no SD ou streaming Wi-Fi.
- Detecção de gestos: sensor de gesto ou IMU para controlar menus.
- Driver ID: RFID/NFC + relé/ignição + perfil no display.
- Saúde do motor: vibração + mic FFT + EGT/IR (experimental).
- Assistente de estacionamento: ToF/ultrassônico + buzzer + tela.
- Caixa-preta do carro: IMU + GPS + OBD2 + SD; impacto e log de viagem.

## 🎮 Jogos / UI / áudio

- Pong com inclinação.
- Snake com botões.
- Labirinto com IMU.
- Dado: shake para rolar.
- Simon Says com botões, buzzer e LED.
- Piano/metrônomo com buzzer.
- Visualizador de música (FFT).
- Relógio com animações e temas.
- Menu com ícones e navegação por encoder/joystick.
- Tamagotchi/pet com RTC e persistência.

## 🧰 Ferramentas

- Multímetro (ADC) com divisor de tensão.
- Osciloscópio de baixa frequência.
- Gerador de sinal PWM.
- Contador de frequência (mic/GPIO).
- Testador/transmissor IR.
- Analisador Wi-Fi/BLE.
- Testador de servo (PWM).
- Terminal serial / ponte UART via Wi-Fi.
- Scanner I2C (mostra endereços).
- Sonda lógica/continuidade.
- Monitor de bateria e consumo.

## 🎯 Shortlist recomendada

1. **Dashboard OBD2 + scanner DTC** — objetivo principal; precisa de adaptador BLE/Wi-Fi.
2. **G-meter + 0–100** — usa o IMU pronto e o OBD2 depois.
3. **Dashboard web Wi-Fi** — M5 como servidor; muito visual.
4. **Controle IR** — usa o LED/IR existente; divertido e rápido.
5. **Monitor TPMS BLE** — se houver sensores de pneu BLE.

## ⚠️ Notas de hardware

- **Pinos:** Grove I2C em G32/G33; HAT expõe G0/G25/G26/G36. Vários I2C podem
  conflitar em endereço; usar hub/mux.
- **Alimentação:** bateria de 200 mAh; Wi-Fi + sensores drenam rápido. No carro,
  usar buck 12V→5V com proteção.
- **Buzzer:** é mono-tom; para áudio real, speaker/amp externo.
- **Sem magnetômetro/GPS onboard:** adicionar módulos Grove (I2C/UART).
- **TinyGo:** alguns units têm driver em `tinygo.org/x/drivers`; outros exigem
  driver I2C/UART/ADC low-level (como o `lib/spm1423`).
- **OBD2:** Bluetooth Classic não funciona; BLE, Wi-Fi ou ponte HC-05.

## Checklist de sensores que você tem

Preencha os modelos para eu mapear drivers e sugerir a próxima POC:

- [ ] ENV III / ENV Pro: __________
- [ ] CO₂ (SCD40/41/30): __________
- [ ] PM2.5: __________
- [ ] Gás (MQ-2/7/135/TVOC): __________
- [ ] Luz/UV: __________
- [ ] Cor: __________
- [ ] PIR / mmWave / radar: __________
- [ ] ToF / ultrassônico / LiDAR: __________
- [ ] IMU Pro / magnetômetro: __________
- [ ] Vibração: __________
- [ ] Encoder / joystick / teclado: __________
- [ ] IR temp / térmica / termopar / DS18B20: __________
- [ ] INA226/INA219: __________
- [ ] Relé / opto: __________
- [ ] CAN / RS485: __________
- [ ] SD card: __________
- [ ] GPS: __________
- [ ] RFID / NFC / biometria: __________
- [ ] 4G / LoRa: __________
- [ ] Outros: __________
