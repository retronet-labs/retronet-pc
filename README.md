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
- **`device`** — periferiche XT: **8237 DMA** (0x00-0x0F, pagine 0x81-0x8F),
  **8259 PIC** (interrupt, 0x20-0x21), **8253 PIT** (timer, 0x40-0x43, → IRQ0),
  **8255 PPI** (tastiera/speaker/DIP, 0x60-0x63), **MDA** (testo 80x25 col 6845,
  0x3B4-0x3BB, RAM a 0xB0000), **FDC NEC 765** (floppy, 0x3F0-0x3F7, → IRQ6 via DMA
  canale 2) e il latch **POST** (0x80).
- **`disk`** — immagini floppy raw con geometria standard (360 KB … 1.44 MB) e
  conversione CHS.
- **`cmd/retronet-pc`** — CLI: carica un BIOS (e un floppy) ed esegue, mostrando
  schermo e codice POST.
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

- Interrupt **8259/8253/8255** ✅; **DMA 8237** + **FDC 765** + immagini floppy ✅;
  tastiera (self-test) e refresh DRAM ✅.
- **Boot di un BIOS reale** ✅: GLaBIOS esegue il **POST senza errori**, disegna
  sull'MDA e **avvia dal floppy** (settore di boot via FDC→DMA→0x7C00, con servizi
  BIOS come `INT 10h`). Vedi [docs/architettura.md](docs/architettura.md).
- **Input da tastiera** ✅: coda di codici di scansione con ritardo di trasmissione
  (handshake INT9); da CLI `-keys "testo"` digita dopo l'avvio.
- Da fare: **CGA**, tasti modificatori (Shift/Ctrl) e tasti estesi, controller
  disco fisso, timing più fedele.

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
