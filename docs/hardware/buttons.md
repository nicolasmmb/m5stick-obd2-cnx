# Botões A/B/C

> Nota técnica preservada do README antigo. O POC que usa este sensor está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Mapeamento

| Botão | GPIO | Uso típico |
|---|---|---|
| A | **G37** | próxima página / ação principal |
| B | **G39** | página anterior / subir |
| C | **G35** | ação da página / descer |

## Como funciona

Os botões são entradas digitais com pull-up. O acionamento puxa o pino para
baixo, então o evento útil é a borda de descida (**falling**).

O POC usa interrupção e contador atômico:

```go
p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
p.SetInterrupt(machine.PinFalling, func(machine.Pin) {
    atomic.AddUint32(&counter, 1)
})
```

## Cuidados

- Em alguns POCs, `GPIO37.Get()`/`GPIO39.Get()` não refletem o botão; use
  interrupção para A/B.
- `GPIO35.Get()` (C) funciona para polling.
- **Toque longo em C pode desligar o Stick.** Evite usar C como botão de ação
  contínua; prefira 2 toques ou A/B.
- Faça debounce por tempo (o POC usa ~250 ms).
