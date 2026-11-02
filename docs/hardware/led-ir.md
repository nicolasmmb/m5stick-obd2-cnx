# LED vermelho / IR — GPIO19

> Nota técnica preservada do README antigo. O POC que usa este sensor está em
> `../../../m5stick-hardware-pocs/board/sensors/`.

## Como funciona

O LED vermelho/IR do M5StickC Plus2 fica em **GPIO19**. É uma saída digital
simples: nível alto liga, nível baixo desliga.

O POC mostra o estado do LED e permite alternar com o botão **C**.

## Cuidados

- É um LED/IR, não um sensor de luz.
- Não deixe ligado por longos períodos sem necessidade.
- Para comunicação IR seria necessário modular a portadora e usar um receptor
  externo; o Stick só tem o emissor.
