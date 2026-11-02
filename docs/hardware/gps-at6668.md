> Nota técnica preservada do README antigo. O POC correspondente está em
> `../../../m5stick-hardware-pocs/`.

# Unit GPS v1.1 (AT6668)

Tela de teste do GPS M5Stack (**v1.1**, AT6668 + MAX2659):

- UART do Grove: **TX=G32, RX=G33**; **115200 8N1** (o U032 antigo usa 9600;
  a tela testa 115200 e cai para 9600 se não houver NMEA).
- Driver puro Go separado:
  [M5Unit-GPS-Go](https://github.com/nicolasmmb/M5Unit-GPS-Go) v0.3.1 —
  parser NMEA com **0 alocações por sentença**.
- NMEA: `GGA` (satélites, qualidade, altitude), `RMC` (UTC, lat/lon, velocidade,
  rumo), `GSV` (satélites em vista e melhor SNR, somados por constelação) e
  `TXT` (antena).
- Páginas (**A/B**, **C** avança): POSIÇÃO (lat/lon/alt, satélites em uso,
  satélites em vista e melhor SNR), TEMPO (UTC, velocidade, rumo), STATUS
  (antena, qualidade, baud, sentenças/checksums e taxa de perda).
- Otimização medida (60 s no hardware): o estado incluía UTC e velocidade, que
  mudam a cada segundo, forçando **58 redraws** e perda de sentenças por
  overflow do ring UART de 128 B. Agora só entra no estado o que a página
  mostra, o SNR é quantizado em 5 dB e o SPI subiu para 80 MHz: **13 redraws**
  e **0% de checksum errors em regime** (5 no boot, durante a troca de telas).
- `ANTENNA OPEN` é reportado pelo módulo mesmo com a cerâmica embutida em uso;
  com vista para o céu o fix é obtido normalmente.
- Grove compartilhado com Fingerprint2/Encoder/ENV IV: um por vez.
