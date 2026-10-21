# Anime

Player streaming da terminale (TUI) e upscaler video in tempo reale con **Anime4K** basato su **mpv**.

---

## ⚠️ Requisiti

Il programma richiede **[mpv](https://mpv.io/)** installato sul sistema e accessibile nel `PATH`.

### Installazione di MPV:
- **macOS** (Homebrew):
  ```bash
  brew install mpv
  ```
- **Linux** (Debian/Ubuntu):
  ```bash
  sudo apt install mpv
  ```
- **Linux** (Arch):
  ```bash
  sudo pacman -S mpv
  ```
- **Windows** (winget o scoop / choco):
  ```powershell
  winget install mpv.net
  # oppure
  scoop install mpv
  ```

---

## 🚀 Utilizzo

### 1. Modalità Player Interattivo (TUI)
Avvia l'interfaccia da terminale per cercare e guardare anime in streaming (con supporto ai provider AnimeWorld e AnimeUnity):

```bash
anime -player
```

**Funzionalità nel player:**
- **Cerca anime**: scrivi il titolo da cercare e premi Invio.
- **Preferiti (`P`)**: visualizza la tua lista dei preferiti. Puoi riprodurli o rimuoverli (`D` o `R`).
- **Riprendi ultimo episodio (`/resume`)**: riprende subito l'ultimo anime visto.
- **Impostazioni (`I`)**: cambia provider streaming preferito.
- **Scelta qualità**: seleziona il preset di upscaling Anime4K desiderato prima della riproduzione.

---

### 2. Riproduzione Diretta (File locale o URL)
Puoi usare il tool anche per applicare i preset Anime4K direttamente a un file video locale o a uno stream diretto:

```bash
anime <video/url> <qualità>
```

Esempio:
```bash
anime video.mp4 2
```

Visualizza la lista dei preset disponibili:
```bash
anime help
```
