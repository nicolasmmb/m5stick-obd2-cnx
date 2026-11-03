# P2 — snapshots da UI com qualidade e idade

> Status: **implementado** (pacote + integração na UI).
> Validação visual no hardware pendente.
> Pré-requisito: P0 concluído; P1 em backlog.
> Objetivo: desenhar a UI a partir de um retrato consistente do cache, com
> qualidade e idade por PID, sem alocação no caminho de render.

## 1. Problema atual

A UI lê cada tile separadamente:

```go
func reading(pid string) (obdpid.Reading, bool) {
    r, ok := readingStore.Get(pid)
    if !ok {
        return obdpid.Reading{}, false
    }
    if r.Quality == obdpid.QualityOK && r.AgeMs(time.Now()) > 3000 {
        r.Quality = obdpid.QualityStale
    }
    return r, true
}
```

Consequências:

- cada tile chama `time.Now()` e pega um instante diferente;
- um frame pode misturar valores de momentos distintos;
- o render pega o lock do cache várias vezes;
- idade/qualidade são recalculadas de formas diferentes em cada caminho;
- fica difícil distinguir “dado velho” de “UI travada”.

## 2. Desenho proposto

### 2.1 Tipos

```go
type Quality uint8

const (
    QualityOK Quality = iota
    QualityStale
    QualityTimeout
    QualityNoData
    QualityUnsupported
)

// 16 bytes por slot: float64 (8) + uint32 (4) + uint16 (2) + uint8 (1)
type Slot struct {
    Value   float64
    AtMs    uint32 // ms desde o boot
    PID     uint16 // dois bytes ASCII
    Quality Quality
    _       uint8
}

const snapshotSlots = 64

type Snapshot struct {
    AtMs   uint32
    Slots  [snapshotSlots]Slot
    Used   uint16
}
```

`PID` é a chave de dois bytes já usada pelo `obdstore`; não há string por
slot. `AtMs` é relativo ao boot; `time.Time` (24 B) não entra no slot.

### 2.2 Publicação

```go
type Store struct {
    mu     sync.Mutex
    active uint32
    bufs   [2]Snapshot
}

func (s *Store) Publish() {
    s.mu.Lock()
    next := 1 - atomic.LoadUint32(&s.active)
    s.bufs[next] = s.bufs[atomic.LoadUint32(&s.active)] // cópia de ~1 KB
    s.bufs[next].AtMs = uint32(liveMs())
    atomic.StoreUint32(&s.active, next)
    s.mu.Unlock()
}

func (s *Store) Current() *Snapshot {
    return &s.bufs[atomic.LoadUint32(&s.active)]
}
```

- produtores (live, replay, sim) atualizam o buffer inativo e publicam;
- a UI lê **um** snapshot por frame, sem lock;
- publicação por amostra (~10–20 Hz), não a 60 Hz;
- 3 buffers de ~1 KB dão dois ciclos de publicação para o frame de 16 ms
  terminar sem reutilizar o buffer que a UI está lendo.

### 2.3 Render

```go
snap := snapshots.Current()
now := uint32(liveMs())

for _, tile := range page.Tiles {
    slot, ok := snap.Get(tile.PID)
    if !ok {
        drawEmpty(tile)
        continue
    }
    age := now - slot.AtMs // mesma base para todos os tiles
    drawTile(tile, slot.Value, slot.Quality, age)
}
```

Regras:

| Qualidade | Cor/estado |
|---|---|
| `OK` | verde/ciano |
| `STALE` | amarelo, com idade no rodapé |
| `TIMEOUT`/`NO_DATA` | vermelho, valor anterior em cinza |
| `UNSUPPORTED` | cinza, `--` |

- se `now - snap.AtMs` passar de 1 s, o heartbeat/rodapé mostra `STALE`;
- se o heartbeat parar, é a UI que travou, não o dado.

## 3. Memória e performance

### 3.1 Baseline medido (build atual: P2 + `C,MEM`)

| Item | Valor |
|---|---|
| Flash | 321.488 B (3,8% de 8 MB) |
| RAM estática (`data` + `bss`) | 62.924 B (12,1% de 520 KB) |
| Arena de heap (`HeapSys`) | 67.851 B |
| Heap no boot | 25.952 B alocados / **41.899 B livres** |
| Heap com BLE habilitado | 45.120 B alocados / **22.731 B livres** |

Medido com `C,MEM` (`runtime.GC` + `runtime.ReadMemStats`).

> O P2 adiciona 3.120 B em `.bss`, então o linker reduz a arena de heap em
> praticamente o mesmo tamanho (70.923 → 67.851 B). O total de SRAM é fixo:
> o ganho é consistência, não RAM grátis. Ainda sobram 22,7 KB livres com
> BLE habilitado (alvo: ≥ 20 KB).

### 3.2 Orçamento do P2

| Item | Custo medido |
|---|---|
| 64 slots × 16 B | 1.024 B por buffer |
| 3 buffers | 3.072 B |
| Cabeçalho + mutex | ~48 B |
| **Total RAM** | **3.120 B (~3,05 KB)** |
| Código (flash) | +796 B |

O limite de aceite é **não passar de 3,2 KB** de RAM adicional e **zero
alocação por frame**. O P2 consome 3 KB da folga; com BLE ainda sobram
**22,7 KB** (acima do alvo de 20 KB).

Medido no build:

| Versão | Flash | RAM estática | Heap livre (BLE) |
|---|---|---|---|
| antes do P2 | 318.832 | 59.804 | 26.064 |
| P2 + `C,MEM` + pedal | **321.488** | **62.924** | **22.731** |
| diferença | +2.656 | **+3.120** | **−3.333** |

### 3.3 Otimizações obrigatórias

- **Packing do slot**: `float64` primeiro, depois `uint32`, `uint16`, `uint8`
  → 16 B. Fora dessa ordem vira 24 B e o buffer cresce 50%.
- **`uint32` ms** em vez de `time.Time` por slot (economiza ~16 B/slot).
- **PID como `uint16`** (dois ASCII), sem string.
- **Publicar em lote** (10–20 Hz), não a cada `P` recebido.
- **3 buffers** + troca atômica; sem `map`/slice por frame.
- **Sem `runtime.GC()` no render**; o comando `C,MEM` chama GC sob demanda
  para reportar memória realmente livre.
- Manter o **dirty-tile** atual: o snapshot não redesenha a tela inteira.

### 3.4 Como medir

Estático:

```bash
tinygo build -target=m5stick-c -size short ./poc/obd2/obd-logger
```

Benchmark:

```bash
go test ./internal/obdsnapshot -bench=. -benchmem
# aceite: Publish/Get com 0 allocs/op
```

Runtime: comando `C,MEM` no firmware:

```bash
stty -f /dev/cu.usbserial-XXXX 115200 raw -echo
printf 'C,MEM\n' > /dev/cu.usbserial-XXXX
# M,mem,alloc=...,free=...,sys=...,idle=...,total=...,gc=...
```

Medir em três pontos: boot, `BLE=ON` e sessão com BLE conectado.

## 4. Benefícios

| Benefício | Ganho |
|---|---|
| Frame consistente | todos os tiles do mesmo instante |
| Menos lock/CPU | uma leitura por frame, não uma por tile |
| Zero alocação no render | mantém os ~60 fps |
| Qualidade/idade corretas | `STALE`/`TIMEOUT` visíveis e uniformes |
| Diagnóstico de trava | heartbeat + idade distinguem UI travada de dado velho |
| Testes melhores | replay gera snapshots e golden frames sem carro |
| Base para P3 | features e modelos usam a mesma semântica de qualidade |
| Alinhamento com o host | mesmo `quality`/`age_ms` do CSV wide |

## 5. Validação

- [x] `go test ./internal/obdsnapshot` cobre slot vazio, qualidade e carry-forward.
- [x] `go test -bench=. -benchmem` com **0 allocs/op**:
  - `Put` ~0,9 ns/op
  - `Get` ~0,6 ns/op
  - `Publish` ~31 ns/op
- [x] `tinygo build -size short`: RAM adicional **3.120 B** (≤ 3,2 KB).
- [x] `Slot` = 16 B e buffer de 64 slots = 1.024 B (teste com `unsafe.Sizeof`).
- [x] `C,SIM` na placa recebe `I/P/E/X` com a UI lendo snapshots.
- [x] Heap livre com BLE **habilitado**: **22.731 B** (≥ 20 KB).
- [ ] Heap livre com BLE **conectado** (medir com `C,MEM`).
- [x] Página 7 (pedal) atualizando em tempo real no hardware.
- [ ] Validação visual final: heartbeat + idade/STALE nas demais páginas.
- [x] `go vet` e `gopls check` limpos.

## 6. Commits previstos

1. `feat(obdsnapshot): add fixed-size snapshot with quality and age`
2. `test(obdsnapshot): benchmark publish and get with zero allocations`
3. `refactor(firmware): publish snapshots from live, replay and sim`
4. `refactor(ui): render one snapshot per frame`
5. `docs: document snapshot memory budget and validation`

## 7. Riscos

- **Duplo buffer e tearing**: publicar mais rápido que o frame pode reutilizar
  o buffer ativo. Mitigação: publicar ≤20 Hz ou usar ring de 3 buffers.
- **Precisão**: `float64` mantém a precisão atual; não trocar por `float32`
  sem medir o erro em lambda/tensão.
- **RAM**: se o heap com BLE cair abaixo de 20 KB, reduzir para 32 slots ou
  publicar só PIDs visíveis + página atual.
- **Complexidade**: a UI inteira passa a usar `snap.Get`; fazer em um commit
  separado e manter o replay como regressão.
