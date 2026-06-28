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
| `0x20`–`0x21` | 8259 PIC   | comandi / maschera                     |
| `0x40`–`0x43` | 8253 PIT   | contatori 0-2 / parola di controllo    |
| `0x60`–`0x63` | 8255 PPI   | tastiera, speaker, DIP switch          |
| `0x80`        | POST       | latch diagnostico (codici di avvio)    |

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

## Limiti attuali e prossimi passi

- Periferiche minime (PIC singolo, PIT/PPI in versione funzionale): le sottigliezze
  elettriche delle modalità del PIT e la matrice completa dei DIP switch della PPI
  non sono riprodotte.
- Mancano video (6845 + CGA/MDA), controller floppy (NEC 765) e il boot di un
  BIOS open XT-compatibile: sono i prossimi moduli della roadmap.
