# Guia: como criar uma lib de unit M5Stack em Go

> Padrão usado em `M5Unit-Fingerprint2-Go`, `M5Unit-Encoder-Go`,
> `M5Unit-GPS-Go`, `M5Unit-ToF90-Go`, `M5Unit-Joystick2-Go` e
> `M5Unit-NFC-Go`.

## 0. Princípios

1. **100% Go** — sem C, C++, CGo, Arduino ou ESP-IDF.
2. **Barramento injetável** — `I2C`/`io.ReadWriter` para testes de host.
3. **0 alocações** no caminho crítico (buffers fixos no `Device`).
4. **Timeouts + `context`** em toda operação que faz polling.
5. **Paridade funcional** com a lib oficial (UIFlow e/ou C/C++), não
   necessariamente a mesma assinatura.
6. **Testes de host** com emulador de registrador/protocolo.
7. **Benchmark realista** com `tx/op`, allocs e footprint.
8. **Validação em hardware** antes de fechar cada épico.
9. **Commit + tag + CI** em cada entrega.
10. **Documentação viva** (README, BENCHMARKS, PLAN).

---

## 1. Pesquisa e escopo (antes de codar)

- [ ] Localizar a página oficial do produto (SKU, chip, I2C/UART, endereço).
- [ ] Baixar/ler a **lib UIFlow** (`uiflow-micropython`) do unit.
- [ ] Baixar/ler a **lib C/C++** (`M5Unit-*`) e o firmware interno, se houver.
- [ ] Anotar o mapa de registradores/comandos e o fluxo de init.
- [ ] Definir o **alvo de paridade**: UIFlow, C/C++, ou ambos.
- [ ] Listar o que **não** será feito (ex.: emulação, NDEF) e por quê.
- [ ] Definir critérios de aceite e hardware necessário (tags/cartões).

Entregável: um `PLAN.md` com escopo, fases, riscos e DoD.

---

## 2. Criar o repositório

Nome: `M5Unit-<Unit>-Go` (ex.: `M5Unit-GPS-Go`).

```bash
mkdir M5Unit-XXX-Go && cd M5Unit-XXX-Go
go mod init github.com/nicolasmmb/M5Unit-XXX-Go
```

Estrutura mínima:

```
M5Unit-XXX-Go/
├── go.mod
├── LICENSE                 # MIT (copiar de outra lib)
├── README.md
├── BENCHMARKS.md
├── Makefile
├── .gitignore              # *.bin *.elf *.map bench/*.txt ...
├── .github/workflows/test.yml
├── device.go               # transporte, Device, erros, helpers
├── init.go                 # reset/init/tabelas de registradores
├── <protocolo>.go          # leitura/escrita específicas
├── device_test.go          # emulador + testes
├── bench_test.go           # benchmarks
├── scripts/bench.sh        # run/save/compare/alloc/profile/escape
├── examples/read/main.go   # build tag esp32
└── testdata/               # capturas reais, se aplicável
```

Identidade dos commits:

```bash
git config user.name "Nícolas Marques"
git config user.email "59735468+nicolasmmb@users.noreply.github.com"
```

---

## 3. Definir a API (antes de implementar)

Padrões que funcionaram:

```go
// Barramento mínimo
type I2C interface { Tx(addr uint16, w, r []byte) error }

// Construção
func New(bus I2C) *Device
func NewAt(bus I2C, addr uint8) *Device
// UART: func New(port io.ReadWriter) *Device

// Ciclo de vida
func (d *Device) Init() error
func (d *Device) InitContext(ctx context.Context) error

// Operações
func (d *Device) Read...() (T, error)
func (d *Device) Write...(v T) error
func (d *Device) ReadContext(ctx context.Context) (T, error)

// Config
func (d *Device) SetTimeout(time.Duration)
func (d *Device) SetAddress(uint8) error
```

Regras:

- `context` **somente** em operações com polling/espera (Init, Measure,
  ReadRange, Detect, auth...). Setters curtos podem ser bloqueantes.
- Erros sentinela (`ErrTimeout`, `ErrNotConnected`, `ErrNotAck`, ...) para
  `errors.Is`.
- Estruturas de resultado por valor (`Card`, `Measurement`) e buffers do
  chamador (`[4]byte`, `[16]byte`) para evitar `[]byte` alocado.
- Nunca usar `fmt` no caminho crítico.
- Um `SetTimeout` com deadline em cada loop; nunca `for {}` sem saída.

---

## 4. Implementar o núcleo

### 4.1 Transporte

- I2C: registradores 8 bits; alguns chips usam **espaço-B 16 bits**
  (escrever `[0xFB, reg, val]`, ler `[0xFB, reg|0x40]`).
- UART: `io.ReadWriter` + buffer de linha fixo.
- Sempre usar buffers no `Device`:

```go
type Device struct {
    mu  sync.Mutex
    bus I2C
    tx  [32]byte
    rx  [64]byte
}
```

Isso evita que slices passados para a interface escapem para o heap.

### 4.2 Init

- Replicar a sequência oficial **na mesma ordem** (reset, IDs, tuning,
  calibração, configuração de timers).
- Tabelas estáticas `[][2]byte` para o tuning; não alocar.
- Validar identidade/versão e retornar `ErrNotConnected`/`ErrBadModel`.
- `context` checado entre estágios e entre transações.

### 4.3 Operações

- Começar pelo caminho de UI (ex.: 1–2 transações por amostra).
- Medir `tx/op` antes de otimizar; otimizar depois (benchstat).
- Escrever sempre com verificação de read-back quando aplicável.
- Escritas em flash/EEPROM: documentar desgaste e espaçar (ex.: 100 ms).

### 4.4 Concorrência

- `sync.Mutex` para operações curtas; semáforo de 1 slot quando o lock
  também precisa ser cancelável por `context`.
- Documentar se a API é thread-safe.

---

## 5. Testes de host

### 5.1 Emulador

Criar um `fakeXxx` que implementa o barramento e emula:

- registradores/comandos/FIFO;
- estados (reset, init, contínuo, endereço);
- respostas roteirizadas do cartão/sensor;
- contadores de transação.

### 5.2 Contratos

```go
func TestNoAlloc(t *testing.T) {
    allocs := testing.AllocsPerRun(100, func() { /* operação */ })
    if allocs != 0 { t.Fatalf(...) }
}

func TestFootprint(t *testing.T) {
    t.Logf("Device=%d B", unsafe.Sizeof(Device{}))
    if unsafe.Sizeof(Device{}) > 384 { t.Fatal(...) }
}
```

- `go test -race ./...`
- Vetores oficiais para criptografia (ex.: Crypto1/Proxmark).
- Testar erros: timeout, NACK, identidade errada, modo errado.

---

## 6. Benchmarks e scripts

### 6.1 Benchmarks

- Usar **dados reais** quando possível (`testdata/cycle.nmea`).
- Reportar `tx/op`:

```go
b.ReportMetric(float64(f.tx-before)/float64(b.N), "tx/op")
```

- Medir cenário realista (com delays do firmware) e cenário de lógica.
- Antes/depois: `bench-save` + `bench-compare`.

### 6.2 Scripts

Copiar `scripts/bench.sh` de uma lib existente:

```bash
cp ../M5Unit-GPS-Go/scripts/bench.sh scripts/
chmod +x scripts/bench.sh
```

Comandos: `run`, `save`, `compare`, `alloc`, `profile`, `escape`.
Makefile:

```make
test:  go test -race ./...
bench: COUNT=8 ./scripts/bench.sh run
alloc: ./scripts/bench.sh alloc
bench-save: ... NAME=before
bench-compare: ./scripts/bench.sh compare $(FROM) $(TO)
examples: tinygo build -target=$(TARGET) -size short ...
```

---

## 7. Documentação

### README

- O que é o unit/chip e o barramento.
- Features (com o que é 0 alloc).
- Hardware (pinos, endereço, velocidade).
- Install (`go get ...@v0.1.0`).
- Quick start.
- Tabela de API.
- Performance (tabela).
- Limitações e roadmap.
- Referências oficiais (UIFlow/C++/datasheet).
- Licença MIT.

### BENCHMARKS.md

- Método (emulador, dados reais, `tx/op`).
- Tabela antes/depois com `benchstat`.
- O que mudou e por quê.

### PLAN.md

- Objetivo, fases, status, pendências, melhorias, matriz de hardware,
  fontes de referência e comparação com as libs originais.

---

## 8. Publicar

Commits agrupados (não um commit gigante):

```bash
git add go.mod LICENSE .gitignore device.go init.go <protocolo>.go
git commit -m "feat: add pure-Go driver for <unit>"

git add device_test.go bench_test.go
git commit -m "test: add register emulator, allocation and latency contracts"

git add examples Makefile scripts BENCHMARKS.md README.md .github
git commit -m "docs: add README, benchmarks, MIT license and CI"
```

Criar repo e subir:

```bash
gh repo create nicolasmmb/M5Unit-XXX-Go --public --source=. \
  --remote=origin --push \
  --description "Pure-Go driver for the M5Stack <unit>"

git tag v0.1.0
git push origin v0.1.0
```

Conferir CI:

```bash
gh run list --limit 3
```

---

## 9. POC e validação de hardware

No repo do projeto (`m5stick-obd2-cnx`):

```bash
GOPROXY=direct go get github.com/nicolasmmb/M5Unit-XXX-Go@v0.1.0
mkdir -p poc/xxx
```

POC:

- Grove I2C1/G32-G33 ou UART conforme o unit.
- Tela com páginas por tipo/protocolo.
- Botões A/B/C com debounce; evitar toque longo se desligar o Stick.
- Log serial a 1 Hz.
- `make xxx` e seção no README.
- `make check` inclui o novo arquivo.

Validação:

- [ ] Init/identidade OK.
- [ ] Leitura bate com o esperado (valores conhecidos).
- [ ] Escrita com verificação (se aplicável).
- [ ] Repetibilidade/estabilidade.
- [ ] Logs sem erros de I2C.
- [ ] UI legível e navegável.

**3 beeps** no terminal antes de pedir ação do usuário.

---

## 10. Iteração e releases

- Correções de bug: `v0.1.1`, `v0.1.2`...
- Features novas: `v0.2.0`, `v0.3.0`...
- Sempre atualizar o `go.mod` do POC e rebuildar.
- Atualizar `PLAN.md` (checkboxes) e `BENCHMARKS.md`.
- Rodar `make check` e CI antes de cada tag.

---

## 11. Checklist final da lib

- [ ] 100% Go, sem CGo.
- [ ] `go vet` + `go test -race` + CI verdes.
- [ ] Emulador de host + testes de erro.
- [ ] 0 alocações no caminho crítico.
- [ ] Footprint documentado/testado.
- [ ] Benchmarks com `tx/op` e antes/depois.
- [ ] README/BENCHMARKS/PLAN completos.
- [ ] Exemplo TinyGo compila (`make examples`).
- [ ] Repo público, MIT, tag `v0.1.0`.
- [ ] POC consome a tag publicada.
- [ ] Hardware validado e registrado no PLAN.

---

## 12. Lições aprendidas (evitar retrabalho)

| Problema | Solução |
|---|---|
| `I2C.Tx` de interface faz slice escapar | buffers no `Device` (`tx`/`rx`/`parityBuf`) |
| Buffer pequeno em escrita grande | dimensionar para o pior frame + CRC + paridade |
| Leitura depois de gravar flash/NACK | esperar ~20–50 ms (settle) |
| I2C 400 kHz instável em alguns units | testar 100 kHz e documentar |
| Flush SPI longo derruba UART | limitar redraw/contadores |
| Botão A/B não lê por `Get()` | usar interrupção; C costuma ler |
| Toque longo em C desliga o Stick | usar B 2×/toque curto |
| Emulação FeliCa sem PT memory/TSN | PT memory F + TSN + wildcard 0xFFFF |
| Crypto1/CRC errado | vetores oficiais + teste de ida/volta |
| `Write` sem CRC_A | MIFARE sempre leva CRC no frame cifrado |
| Erro genérico na escrita | erros por fase (auth/read/ack/verify) |
| Paridade inconsistente | matriz de comparação + `PLAN.md` |
