# CLAUDE.md — retronet-pc

Emulatore **IBM PC/XT** (Go) costruito sopra la CPU `retronet-8086`: il core
8086/8088 fa da motore, questo repo aggiunge la **macchina** intorno — bus di
memoria a 1 MB, spazio di I/O e periferiche XT. Panoramica utente:
[README.md](README.md); mappa memoria/I/O e percorso interrupt:
[docs/architettura.md](docs/architettura.md).

## Setup su una macchina nuova (handoff)

1. Clona i repo come cartelle **sibling** sotto la stessa radice:
   ```
   work/source/
   ├── retronet-logic/
   ├── retronet-hardware/
   ├── retronet-8086/    (v0.1.0+)
   └── retronet-pc/      (questo repo)
   ```
   Un clone pulito compila dalle versioni pubblicate; i sibling servono solo per
   il co-sviluppo.
2. Workspace locale (`go.work` **non versionato**, da ricreare):
   ```sh
   go work init . ../retronet-8086 ../retronet-hardware ../retronet-logic
   go test ./...
   ```
3. **Asset esterni non versionati** (`*.rom`/`*.bin`/`*.img` sono gitignored):
   - **BIOS**: GLaBIOS (XT, GPLv3) — scarica una release `.ROM` da
     github.com/640-KB/GLaBIOS (testato con **0.4.2 8X**). Non redistribuito nel
     repo: lo fornisce l'utente.
   - **Floppy/boot sector**: immagini `.img`/`.rom` (es. assemblate con
     `retronet-asm`, backend `i8086`). `disk.NewFloppy` riempie le immagini
     piccole al formato standard, quindi un boot sector da 512 byte si usa diretto
     come `-floppy`.

## Comandi

- Test: `go test ./...` (richiede `go.work`)
- CLI:
  ```sh
  go run ./cmd/retronet-pc -bios GLABIOS_x_8X.ROM -floppy disco.img
  go run ./cmd/retronet-pc -bios <ROM> -floppy <img> -video cga   # CGA invece di MDA
  go run ./cmd/retronet-pc -bios <ROM> -floppy <img> -alu gate    # ALU a porte logiche
  go run ./cmd/retronet-pc -bios <ROM> -floppy <img> -keys "ciao" # digita dopo l'avvio
  go run ./cmd/retronet-pc -bios <ROM> -floppy <img> -interactive  # uso interattivo (Ctrl+] esce)
  ```
  Flag: `-steps` (limite istruzioni), `-video mda|cga` (default mda),
  `-alu native|gate` (**default native**, più veloce; `gate` = aritmetica dai gate),
  `-interactive` (schermo a 60 Hz + tastiera reale in raw mode; `-ips` = passi per
  fotogramma, cioè la velocità percepita).

## Componenti

- **`memory`**: bus 1 MB con regioni ROM protette (BIOS in cima) + RAM; `cpu.Bus`.
- **`io`**: dispatcher per intervalli di porte → `Device`; `cpu.Ports`; non mappate → 0xFF.
- **`device`**: **8237 DMA** (0x00-0x0F, pagine 0x81-0x8F), **8259 PIC** (0x20-0x21),
  **8253 PIT** (0x40-0x43, counter0→IRQ0, counter1→refresh DRAM via DMA ch0),
  **8255 PPI** (tastiera/speaker/DIP, 0x60-0x63; tastiera set 1 completa: testo,
  Shift, **Ctrl/Alt**, tasti **estesi `0xE0`** e **funzione**, via `PressKey`/`Type`),
  **MDA** (0x3B4, 0xB0000) e
  **CGA** (0x3D4, 0xB8000) via `TextVideo` generico, **FDC NEC 765** (0x3F0-0x3F7,
  IRQ6 via DMA ch2), latch **POST** (0x80).
- **`disk`**: immagini floppy raw (360 KB … 1.44 MB), conversione CHS.
- **`machine`**: `NewXT()` cabla CPU+memoria+I/O+periferiche; reset da `0xFFFF0`.
  Default `SetALU(cpu.Native)`. `UseCGA()` imposta i DIP su video a colori.
  Il ciclo macchina (`Step`) avanza il PIT, riconosce gli IRQ del PIC e li consegna
  con `CPU.Interrupt()` se IF=1; `Ppi.Tick` gestisce il ritardo di trasmissione
  della tastiera.

## Trappole già risolte (NON regredire)

- **Tastiera Shift**: il ritardo di trasmissione si avvia sul fronte di **salita**
  dell'ack PB7 (non discesa). INT9 fa `STI` presto e l'ack è un impulso a due
  fronti: avviando il ritardo sulla discesa, tra i due fronti il tasto successivo
  veniva presentato subito e rientrava annidato, sfasando i modificatori.
- **PIT**: l'accesso solo-LSB deve azzerare il byte alto (un reload spurio
  rallentava il refresh DRAM → POST error 0400 sul test DMA). Serve anche il bit
  **Terminal Count TC0** nello status DMA (0x08). `machine.timerCycles=8` (non
  cycle-accurate) è scelto perché TC0 sia pronto in tempo per GLaBIOS.
- **AF della sottrazione** (a monte, in `bridge/i8086`): bit 4 di `a^b^risultato`
  con la **b originale**. Già corretto (hardware v0.7.1).
- **FDC Read/Write multi-settore ↔ Terminal Count del DMA** (`device/fdc.go`): il
  765 parte sempre dal settore **R** (ne legge almeno uno) e prosegue verso EOT, ma
  è il **TC del DMA** a fissare la lunghezza reale. Due trappole: (1) NON fermarsi al
  TC faceva leggere l'intera traccia quando il DMA era programmato per un solo
  settore; (2) usare **EOT come limite del ciclo** (`for s:=r; s<=eot`) non leggeva
  nulla quando `R>EOT`. GLaBIOS carica il kernel un settore alla volta in un buffer
  di rimbalzo, con **EOT fisso** (es. 8) e richieste `R=9,10,…`, affidandosi al TC:
  col vecchio modello dal 9° settore restava nel buffer roba stantia → KERNEL.SYS
  corrotto → decompressione UPX che produce un `jmp far` nel vuoto → loop infinito
  (il boot di FreeDOS si bloccava qui). Ora `TransferToMemory`/`TransferFromMemory`
  riportano il TC e il ciclo legge sempre R e si ferma su TC o a EOT.

## Stato

`go test ./...` verde. **GLaBIOS 0.4.2 completa il POST senza errori e BOOTA dal
floppy** sul nostro core (anche con `-alu gate`): POST → settore di boot
(FDC→DMA ch2→0x7C00) → salto → codice di boot coi servizi BIOS (`INT 10h`,
`INT 16h`) → schermo MDA/CGA. Tastiera set 1 completa (testo, Shift, Ctrl/Alt,
estesi, funzione) verificata in eco. Validazione incrociata con `retronet-asm`: boot
sector assemblati che bootano qui. **Boota anche un floppy FreeDOS 1.44 MB
completo**: boot sector FAT12 → caricamento di KERNEL.SYS (UPX) via FDC/DMA →
decompressione → kernel + FreeCom 0.86 fino al prompt; input da tastiera fino al
`A:\>` (test d'integrazione `machine/boot_freedos_test.go`, si salta senza gli asset).

**Frontend interattivo** (`cmd/retronet-pc -interactive`): raw mode su stdin
(darwin/linux, `rawmode_*.go`), schermo testuale ridisegnato a 60 Hz con cursore
hardware, traduzione dei tasti dell'host in scancodi (testo/Ctrl-x via `Type`,
sequenze ANSI frecce/navigazione/F1-F4 via `PressKey`); uscita con Ctrl+]. Il
parser dei tasti e' coperto da test (`cmd/retronet-pc/interactive_test.go`).

Tag: `v0.1.0`. Prossimi passi: modi **grafici CGA** (framebuffer a pixel sopra il
frontend), controller **disco fisso**, seriale/parallela, timing più fedele.
