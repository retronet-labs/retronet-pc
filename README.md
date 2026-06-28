# retronet-pc — IBM PC/XT compatibile

Emulatore di un **IBM PC/XT** in Go, parte dell'ecosistema **RetroNet**, costruito
sopra la CPU [retronet-8086](https://github.com/retronet-labs/retronet-8086): il
core 8086/8088 fa da motore, retronet-pc aggiunge la **macchina** intorno — bus di
memoria mappato, spazio di I/O e periferiche.

> L'aritmetica della CPU gira (di default) sull'ALU a **porte logiche** di
> retronet-logic: un PC/XT che calcola dai gate.

## Stato

Fatto e testato (`go test ./...` verde):

- **`memory`** — bus a 1 MB con regioni **ROM** protette da scrittura (BIOS,
  option ROM) e RAM scrivibile; soddisfa `cpu.Bus`.
- **`io`** — dispatcher dello spazio di I/O che instrada per intervallo di porte
  verso le periferiche (`Device`); soddisfa `cpu.Ports`. Porte non mappate: 0xFF.
- **`device`** — periferiche XT: **8259 PIC** (controllore interrupt, porte
  0x20-0x21), **8253 PIT** (timer, 0x40-0x43, contatore 0 → IRQ0), **8255 PPI**
  (tastiera/speaker/DIP, 0x60-0x63) e il latch **POST** (0x80).
- **`machine`** — `NewXT()` assembla CPU + memoria + I/O + periferiche già cablate
  e gestisce il **percorso degli interrupt** PIT → PIC → CPU. Dopo il reset la CPU
  parte dal vettore `0xFFFF0`, dove si carica il BIOS con `Mem.LoadROM`.

Architettura, mappa di memoria/I/O e percorso interrupt: vedi
[docs/architettura.md](docs/architettura.md).

Esempio — il timer genera IRQ0 e la CPU lo serve:

```go
m := machine.NewXT()
// (programma PIC/PIT e installa il gestore del vettore 8; vedi
//  machine/interrupt_test.go per l'esempio completo)
m.Run(2000) // il gestore IRQ0 viene eseguito a ogni tick del timer
```

## Roadmap

- Integrazione interrupt **8259/8253/8255** ✅ (percorso PIT → PIC → CPU testato).
- Video: **6845** CRTC + buffer testo **CGA/MDA** con render; controller floppy
  **NEC 765**.
- Boot di un **BIOS open** XT-compatibile (GLaBIOS / Super PC-XT, redistribuibili)
  fino al prompt. Le ROM restano fuori dal repo.

## Sviluppo locale (multi-repo)

Dipende da `retronet-8086` (`v0.1.0`) e a cascata da `retronet-hardware`/
`retronet-logic`. Un clone pulito compila dalle versioni pubblicate; per
co-sviluppare in locale si usano i checkout sibling con un `go.work` (non
versionato):

```sh
go work init . ../retronet-8086 ../retronet-hardware ../retronet-logic
go test ./...
```

## Licenza

MIT.
