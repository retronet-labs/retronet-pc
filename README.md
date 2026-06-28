# retronet-pc — IBM PC/XT compatibile

Emulatore di un **IBM PC/XT** in Go, parte dell'ecosistema **RetroNet**, costruito
sopra la CPU [retronet-8086](https://github.com/retronet-labs/retronet-8086): il
core 8086/8088 fa da motore, retronet-pc aggiunge la **macchina** intorno — bus di
memoria mappato, spazio di I/O e periferiche.

> L'aritmetica della CPU gira (di default) sull'ALU a **porte logiche** di
> retronet-logic: un PC/XT che calcola dai gate.

## Stato

Fondamenta (testate, `go test ./...` verde):

- **`memory`** — bus a 1 MB con regioni **ROM** protette da scrittura (BIOS,
  option ROM) e RAM scrivibile; soddisfa `cpu.Bus`.
- **`io`** — dispatcher dello spazio di I/O che instrada per intervallo di porte
  verso le periferiche (`Device`); soddisfa `cpu.Ports`. Porte non mappate: 0xFF.
- **`machine`** — assembla CPU + memoria + I/O. Dopo il reset la CPU parte dal
  vettore `0xFFFF0`, dove si carica il BIOS con `Mem.LoadROM`.
- **`device`** — periferiche; per ora il latch diagnostico **POST** sulla porta
  `0x80`.

Esempio (mini-BIOS in ROM che scrive un codice POST e si ferma):

```go
m := machine.New()
post := &device.PostCode{}
m.Map(0x80, 0x80, post)
m.Mem.LoadROM(cpu.PhysAddr(0xFFFF, 0x0000), []byte{
    0xB0, 0xAA, // MOV AL,0xAA
    0xE6, 0x80, // OUT 0x80,AL
    0xF4,       // HLT
})
m.Run(100)      // post.Last == 0xAA, eseguito dalla ROM al reset vector
```

## Roadmap

- Periferiche XT: **8259** PIC, **8253** PIT, **8255** PPI (tastiera/speaker/DIP),
  con integrazione degli interrupt (linea INTR + vettori).
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
