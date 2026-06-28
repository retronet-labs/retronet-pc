# Architettura di retronet-pc

retronet-pc costruisce un **IBM PC/XT** attorno alla CPU
[retronet-8086](https://github.com/retronet-labs/retronet-8086). La CPU non sa
nulla della macchina: vede solo due interfacce, il **bus di memoria** (`cpu.Bus`)
e lo **spazio di I/O** (`cpu.Ports`). retronet-pc fornisce le implementazioni
concrete e ci collega le periferiche.

```
            +---------------------- machine.Machine ----------------------+
            |                                                             |
   cpu.CPU8086 --Mem--> memory.Bus (RAM + ROM)                           |
        |        --IO--> io.Ports ---> device.PIC  (0x20-0x21)           |
        |                          \-> device.PIT  (0x40-0x43)           |
        |                          \-> device.PPI  (0x60-0x63)           |
        |                                                                 |
        +<---- Interrupt(vettore) <---- PIC <---- IRQ0 <---- PIT          |
            +-------------------------------------------------------------+
```

## Mappa della memoria (1 MB)

| Intervallo            | Uso                                  |
|-----------------------|--------------------------------------|
| `0x00000`–`0x9FFFF`   | RAM convenzionale (fino a 640 KB)    |
| `0xA0000`–`0xBFFFF`   | RAM video (CGA `0xB8000`, MDA `0xB0000`) |
| `0xC0000`–`0xEFFFF`   | option ROM                           |
| `0xF0000`–`0xFFFFF`   | ROM di sistema (BIOS)                |

Il `memory.Bus` è una memoria piatta da 1 MB; le regioni caricate con `LoadROM`
diventano di sola lettura (le scritture vengono ignorate, come sull'hardware).
All'accensione la CPU parte dal **reset vector** `0xFFFF0` (CS=0xFFFF, IP=0), dove
risiede il punto d'ingresso del BIOS.

## Mappa dell'I/O (porte usate)

| Porte         | Periferica | Note                                   |
|---------------|------------|----------------------------------------|
| `0x00`–`0x0F` | 8237 DMA   | indirizzo/conteggio/modo/maschera      |
| `0x20`–`0x21` | 8259 PIC   | comandi / maschera                     |
| `0x40`–`0x43` | 8253 PIT   | contatori 0-2 / parola di controllo    |
| `0x60`–`0x63` | 8255 PPI   | tastiera, speaker, DIP switch          |
| `0x80`        | POST       | latch diagnostico (codici di avvio)    |
| `0x81`–`0x8F` | 8237 DMA   | registri di pagina                     |
| `0x3B4`–`0x3BB` | MDA (6845) | indice/dato CRTC, modo, stato        |
| `0x3F0`–`0x3F7` | FDC (765) | controllore floppy (DOR/MSR/dati)     |

`io.Ports` instrada ogni accesso alla periferica il cui intervallo contiene la
porta; le porte non mappate leggono `0xFF` e ignorano le scritture.

## Percorso degli interrupt

1. Il **contatore 0** del PIT, programmato dal BIOS (~18,2 Hz), produce un
   azzeramento periodico la cui uscita alza **IRQ0** sul PIC.
2. Il **PIC** registra la richiesta (IRR), applica maschera e priorità e, se l'IRQ
   è il più prioritario non bloccato, lo propone alla CPU.
3. La `Machine`, a ogni passo, se il PIC ha un IRQ pronto **e** il flag `IF` è
   abilitato, riconosce l'interrupt (`Acknowledge`) ottenendo il numero di vettore
   e chiama `CPU.Interrupt(vettore)`.
4. La CPU impila FLAGS/CS/IP, azzera IF/TF e salta al gestore puntato dalla tabella
   dei vettori. Il gestore, a fine routine, invia un **EOI** al PIC (porta `0x20`)
   e ritorna con **IRET**.

Un interrupt risveglia inoltre la CPU dallo stato **HLT**: il programma principale
può attendere il prossimo tick con `HLT`.

## Modello temporale

Il modello **non è cycle-accurate**. A ogni passo della macchina il PIT viene
fatto avanzare di un piccolo numero fisso di colpi di clock (`timerCycles`): è
un'approssimazione del rapporto tra il clock della CPU (~4,77 MHz) e quello del
timer (~1,19 MHz), sufficiente a un tick periodico credibile per far girare il
software, ma non a riprodurre tempi esatti.

## Avvio di un BIOS reale

Con un BIOS open XT-compatibile (es. **GLaBIOS**, GPLv3) caricato con `LoadBIOS`,
la macchina esegue il POST e disegna sull'MDA: rileva RAM (640 KB), tipo video
(Mono), CPU (8088) e il numero di floppy. Le ROM **non** sono incluse nel repo:

```bash
# scaricare una ROM GLaBIOS (es. variante 8X) da
#   https://github.com/640-KB/GLaBIOS/releases
go run ./cmd/retronet-pc -bios GLABIOS_x.x.x_8X.ROM -floppy disco.img
```

## Limiti attuali e prossimi passi

- DMA (8237) e FDC (765) sono in versione **funzionale**: i registri sono fedeli e
  il floppy legge/scrive settori via DMA canale 2, ma il trasferimento avviene in
  blocco (non ciclo per ciclo) e il refresh della DRAM non è simulato.
- Video: l'**MDA** (testo 80x25 monocromatico) è implementato col 6845 e un render
  testuale (`Machine.Screen()`); manca la **CGA** (colori e grafica).
- **Tastiera**: manca il controllore di tastiera (PPI + IRQ1 + self-test): per
  questo il POST del BIOS segnala un errore KB e attende un tasto. Con la tastiera
  e il superamento del test DMA il BIOS proseguirebbe fino al boot dal floppy.
