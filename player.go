package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"UpscalerCLI/streaming"

	"github.com/nsf/termbox-go"
)

const (
	Red    string = "\033[31m"
	Green  string = "\033[32m"
	Yellow string = "\033[33m"
	Cyan   string = "\033[36m"
	Reset  string = "\033[0m"
)

func WrapIn(s string, fixes ...string) string {
	for _, fix := range fixes {
		s = fix + s
	}
	return s + Reset
}

var (
	colDefault = termbox.ColorDefault
	colWhite   = termbox.ColorWhite | termbox.AttrBold
	colGreen   = termbox.ColorGreen | termbox.AttrBold
	colYellow  = termbox.ColorYellow | termbox.AttrBold
	colCyan    = termbox.ColorCyan | termbox.AttrBold
	colRed     = termbox.ColorRed | termbox.AttrBold
	colBlack   = termbox.ColorBlack
	colMagenta = termbox.ColorMagenta | termbox.AttrBold
)

func tbFill(x, y, w int, ch rune, fg, bg termbox.Attribute) {
	for i := range w {
		termbox.SetCell(x+i, y, ch, fg, bg)
	}
}

func tbPrint(x, y int, fg, bg termbox.Attribute, text string) int {
	col := x
	for _, r := range text {
		termbox.SetCell(col, y, r, fg, bg)
		col++
	}
	return col
}

func tbPrintCentered(y int, fg, bg termbox.Attribute, text string) {
	w, _ := termbox.Size()
	runes := []rune(text)
	x := (w - len(runes)) / 2
	if x < 0 {
		x = 0
	}
	tbPrint(x, y, fg, bg, text)
}

func tbDrawBox(x, y, w, h int, fg, bg termbox.Attribute) {
	// top
	termbox.SetCell(x, y, '╔', fg, bg)
	tbFill(x+1, y, w-2, '═', fg, bg)
	termbox.SetCell(x+w-1, y, '╗', fg, bg)
	// bottom
	termbox.SetCell(x, y+h-1, '╚', fg, bg)
	tbFill(x+1, y+h-1, w-2, '═', fg, bg)
	termbox.SetCell(x+w-1, y+h-1, '╝', fg, bg)
	// sides
	for i := 1; i < h-1; i++ {
		termbox.SetCell(x, y+i, '║', fg, bg)
		termbox.SetCell(x+w-1, y+i, '║', fg, bg)
	}
}

func tbClear() {
	termbox.Clear(colDefault, colDefault)
}

func tbFlush() {
	termbox.Flush()
}

// screenWidth ritorna la larghezza corrente del terminale
func screenWidth() int {
	w, _ := termbox.Size()
	return w
}

// screenHeight ritorna l'altezza corrente del terminale
func screenHeight() int {
	_, h := termbox.Size()
	return h
}

var bannerLines = []string{
	"                                                                                              ",
	"               L.                                              ,;                .   G:      ",
	"               EW:        ,ft t                              f#i                ,W   E#,    :",
	"            .. E##;       t#E Ej            ..       :     .E#t                i##   E#t  .GE",
	"           ;W, E###t      t#E E#,          ,W,     .Et    i#W,                f###   E#t j#K",
	"           j##, E#fE#f     t#E E#t         t##,    ,W#t   L#D.                G####   E#GK#f  ",
	"    G###, E#t D#G    t#E E#t        L###,   j###t :K#Wfff;            .K#Ki##   EEE",
	"  :E####, E#t  f#E.  t#E E#t      .E#j##,  G#fE#t i##WLLLLt          ,W#D.,##   EEE",
	"      ;W#DG##, E#t   t#K: t#E E#t     ;WW; ##,:K#i E#t  .E#L             i##E,,i##,  E#jL#D:",
	"      j###DW##, E#t    ;#W,t#E E#t    j#E.  ##f#W,  E#t    f#E:          ;DDDDDDE##DGiE#t ,K#j",
	"     G##i,,G##, E#t     :K#D#E E#t  .D#L    ###K:   E#t     ,WW;                ,##   E#t   jD",
	"   :K#K:   L##, E#t      .E##E E#t :K#t     ##D.    E#t      .D#;               ,##   j#t     ",
	"  ;##D.    L##, ..         G#E E#t ...      #G      ..         tt               .E#    ,;     ",
	"  ,,,      .,,              fE ,;.          j                                     t           ",
	"                             ,                                                                "}

// tbDrawHeader disegna un header con banner ASCII art centrato su palette rossa
func tbDrawHeader(y int) int {
	w := screenWidth()

	// Riga superiore di confine
	tbFill(0, y, w, '═', colRed, colDefault)
	y++

	// Logo ASCII art centrato — rosso brillante
	for _, line := range bannerLines {
		runes := []rune(line)
		x := (w - len(runes)) / 2
		if x < 0 {
			x = 0
		}
		tbPrint(x, y, colRed, colDefault, line)
		y++
	}

	// Sottotitolo: descrizione a sinistra, autore a destra
	subLeft := "  ◈  4K Streaming Player"
	subRight := "by Andrea Maruca  ◈  "
	tbPrint(2, y, colMagenta, colDefault, subLeft)
	tbPrint(w-len([]rune(subRight))-1, y, colMagenta, colDefault, subRight)
	y++

	// Riga inferiore di confine
	tbFill(0, y, w, '═', colRed, colDefault)
	y++

	return y
}

// tbDrawFooter disegna una barra in fondo con i controlli disponibili
func tbDrawFooter(hints string) {
	_, h := termbox.Size()
	w := screenWidth()
	tbFill(0, h-2, w, '─', colMagenta, colDefault)
	tbPrint(2, h-1, colMagenta, colDefault, "  "+hints)
}

// tbDrawSeparator disegna una riga separatrice
func tbDrawSeparator(y int) {
	w := screenWidth()
	tbFill(0, y, w, '─', colMagenta, colDefault)
}

func readLine(x, y int, fg, bg termbox.Attribute) string {
	var buf []rune
	termbox.SetCursor(x, y)
	termbox.Flush()

	for {
		ev := termbox.PollEvent()
		switch ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc:
				termbox.HideCursor()
				return ""
			case termbox.KeyEnter:
				termbox.HideCursor()
				return strings.TrimSpace(string(buf))
			case termbox.KeyBackspace, termbox.KeyBackspace2:
				if len(buf) > 0 {
					buf = buf[:len(buf)-1]
					termbox.SetCell(x+utf8.RuneCountInString(string(buf)), y, ' ', fg, bg)
					termbox.SetCursor(x+len(buf), y)
					termbox.Flush()
				}
			case termbox.KeySpace:
				buf = append(buf, ' ')
				termbox.SetCell(x+len(buf)-1, y, ' ', fg, bg)
				termbox.SetCursor(x+len(buf), y)
				termbox.Flush()
			default:
				if ev.Ch != 0 {
					buf = append(buf, ev.Ch)
					termbox.SetCell(x+len(buf)-1, y, ev.Ch, fg, bg)
					termbox.SetCursor(x+len(buf), y)
					termbox.Flush()
				}
			}
		case termbox.EventResize:
			termbox.Flush()
		}
	}
}

type menuItem struct {
	label    string
	subLabel string
}

// azione: "select" | "back" | "quit"
func showMenu(title string, items []menuItem, startIndex int, extraHint string) (int, string) {
	selected := startIndex
	if selected >= len(items) {
		selected = 0
	}
	offset := 0

	for {
		tbClear()
		_, h := termbox.Size()
		w := screenWidth()

		// Header
		row := tbDrawHeader(0)

		// Titolo pagina
		tbPrint(2, row, colRed, colDefault, title)
		row += 2

		listHeight := max(h-row-3, 1)

		if selected < offset {
			offset = selected
		}
		if selected >= offset+listHeight {
			offset = selected - listHeight + 1
		}

		// Disegna lista
		for i := 0; i < listHeight && offset+i < len(items); i++ {
			idx := offset + i
			item := items[idx]
			y := row + i

			isCurrent := idx == selected
			var rowFg, rowBg termbox.Attribute
			var prefix string
			if isCurrent {
				rowFg = colWhite
				rowBg = termbox.ColorRed
				prefix = "▶ "
			} else {
				rowFg = colWhite
				rowBg = colDefault
				prefix = "  "
			}

			// riempi la riga di sfondo
			tbFill(0, y, w, ' ', rowFg, rowBg)

			label := fmt.Sprintf("%s %s", prefix, item.label)
			tbPrint(1, y, rowFg, rowBg, label)
			if item.subLabel != "" {
				tbPrint(w-len([]rune(item.subLabel))-2, y, colMagenta, rowBg, item.subLabel)
			}
		}

		// Freccia scroll se ci sono altri elementi
		if offset+listHeight < len(items) {
			tbPrint(w/2, row+listHeight, colRed, colDefault, "▼ scorri")
		}
		if offset > 0 {
			tbPrint(w/2, row-1, colRed, colDefault, "▲ scorri")
		}

		hint := "↑↓ naviga  Enter seleziona  B torna  Q esci"
		if extraHint != "" {
			hint = extraHint
		}
		tbDrawFooter(hint)
		tbFlush()

		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			switch ev.Key {
			case termbox.KeyArrowUp:
				if selected > 0 {
					selected--
				}
			case termbox.KeyArrowDown:
				if selected < len(items)-1 {
					selected++
				}
			case termbox.KeyEnter:
				return selected, "select"
			case termbox.KeyEsc:
				return selected, "back"
			case termbox.KeyDelete:
				return selected, "delete"
			}
			switch ev.Ch {
			case 'b', 'B':
				return selected, "back"
			case 'q', 'Q':
				return selected, "quit"
			case 'd', 'D', 'r', 'R':
				return selected, "delete"
			}
		}
	}
}

// showSearchScreen mostra l'input di ricerca. Ritorna query, action ("search"|"settings"|"quit"|"favourites")
func showSearchScreen(providerName string, tui *PlayerTUI) (string, string) {
	for {
		tbClear()
		w := screenWidth()
		row := tbDrawHeader(0)
		tbPrint(2, row, colRed, colDefault, fmt.Sprintf("◈ Provider: %s", providerName))
		row += 2

		configs := tui.config

		shouldDisplayLastEpisodeWatched := configs.LastEpisodeWatched.Anime.Name != "Not watched"

		if shouldDisplayLastEpisodeWatched {
			tbPrint(2, row, colRed, colDefault, fmt.Sprintf("◈ Last episode watched: %s - ep %s", configs.LastEpisodeWatched.Anime.Name, configs.LastEpisodeWatched.EpisodeNumber))
			row += 2
		}

		inputStr := "  Cerca un anime"
		if shouldDisplayLastEpisodeWatched {
			inputStr += " (/resume to go back to the last anime watched) "
		}
		inputStr += ":"

		tbPrint(2, row, colMagenta, colDefault, inputStr)
		row++
		tbDrawSeparator(row)
		row++

		prompt := "▶ "
		tbPrint(2, row, colRed, colDefault, prompt)
		tbFill(2+len([]rune(prompt)), row, w-4-len([]rune(prompt)), '▁', colMagenta, colDefault)

		tbDrawFooter("Enter cerca  I impostazioni  Q esci  P preferiti")
		tbFlush()

		query := readLine(2+len([]rune(prompt)), row, colWhite, colDefault)
		if query == "" {
			// controlla se è stato premuto ESC (readLine ritorna "" anche con ESC)
			// In questo caso mostriamo di nuovo la schermata e catturiamo altri tasti
			// Gestiamo I e Q con un evento dopo
			tbClear()
			tbFlush()
			// leggiamo un singolo evento per gestire I/Q
			ev := termbox.PollEvent()
			if ev.Type == termbox.EventKey {
				switch ev.Ch {
				case 'i', 'I':
					return "", "settings"
				case 'q', 'Q':
					return "", "quit"
				case 'p', 'P':
					return "", "favorites"
				}
				switch ev.Key {
				case termbox.KeyEsc:
					return "", "quit"
				}
			}
			continue
		}

		lower := strings.ToLower(query)
		if lower == "q" || lower == "quit" {
			return "", "quit"
		}
		if lower == "i" {
			return "", "settings"
		}
		if lower == "p" {
			return "", "favorites"
		}
		if lower == "/resume" {
			ctx := context.Background()
			quit := tui.runAllEpisodes(ctx, &configs.LastEpisodeWatched.Anime)
			if quit {
				return "", "quit"
			}
			continue
		}
		return query, "search"
	}
}

func showMessage(title, msg string, fg termbox.Attribute) {
	tbClear()
	w := screenWidth()
	row := tbDrawHeader(0)
	row++
	tbPrintCentered(row, fg, colDefault, title)
	row += 2
	// wrap lunga
	words := strings.Fields(msg)
	line := ""
	for _, w2 := range words {
		if len(line)+len(w2)+1 > w-4 {
			tbPrintCentered(row, colWhite, colDefault, line)
			row++
			line = w2
		} else {
			if line != "" {
				line += " "
			}
			line += w2
		}
	}
	if line != "" {
		tbPrintCentered(row, colWhite, colDefault, line)
	}
	tbFlush()
}

func showLoading(msg string) {
	showMessage("◈  Caricamento...", msg, colRed)
}

type PlayerTUI struct {
	client streaming.Provider
	config *Config
}

func NewPlayerTUI(client streaming.Provider, configs *Config) *PlayerTUI {
	return &PlayerTUI{client: client, config: configs}
}

func (p *PlayerTUI) Run() {
	if err := termbox.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "Impossibile inizializzare termbox:", err)
		os.Exit(1)
	}
	defer termbox.Close()

	termbox.SetInputMode(termbox.InputEsc)

	for {
		query, action := showSearchScreen(p.client.Name(), p)
		switch action {
		case "quit":
			return
		case "favorites":
			if quit := p.runFavourites(); quit {
				return
			}
			continue
		case "settings":
			if quit := p.runSettings(); quit {
				return
			}
			continue
		}

		// ricerca
		showLoading("Ricerca in corso per: " + query)
		ctx := context.Background()
		results, err := p.client.Search(ctx, query)
		if err != nil {
			p.showError("Errore ricerca", err.Error())
			continue
		}
		if len(results) == 0 {
			p.showError("Nessun risultato", "Nessun anime trovato per: "+query)
			continue
		}

		if quit := p.runSelectAnime(ctx, query, results); quit {
			return
		}
	}
}

func (p *PlayerTUI) runFavourites() bool {
	if len(p.config.SavedAnime) == 0 {
		p.showError("Preferiti", "Nessun anime salvato nei preferiti.")
		return false
	}

	lastSel := 0
	for {
		if len(p.config.SavedAnime) == 0 {
			p.showError("Preferiti", "Nessun altro preferito salvato.")
			return false
		}

		if lastSel >= len(p.config.SavedAnime) {
			lastSel = len(p.config.SavedAnime) - 1
		}

		items := make([]menuItem, len(p.config.SavedAnime))
		for i, anime := range p.config.SavedAnime {
			items[i] = menuItem{label: anime.Name}
		}

		idx, action := showMenu(
			"◈ Anime Preferiti",
			items, lastSel,
			"↑↓ naviga  Enter riproduci  D rimuovi  B torna  Q esci",
		)
		switch action {
		case "quit":
			return true
		case "back":
			return false
		case "delete":
			removed := p.config.SavedAnime[idx]
			p.config.SavedAnime = slices.Delete(p.config.SavedAnime, idx, idx+1)
			if err := SaveConfig(p.config); err != nil {
				p.showError("Errore salvataggio", err.Error())
			} else {
				// Mostra conferma se non ci sono più elementi, altrimenti aggiorna la lista
				if len(p.config.SavedAnime) == 0 {
					p.showError("Preferiti", fmt.Sprintf("\"%s\" rimosso dai preferiti.", removed.Name))
					return false
				}
			}
			lastSel = idx
		case "select":
			lastSel = idx
			selected := &p.config.SavedAnime[idx]
			ctx := context.Background()
			if quit := p.runAllEpisodes(ctx, selected); quit {
				return true
			}
		}
	}
}

// runSelectAnime — pagina lista risultati di ricerca
func (p *PlayerTUI) runSelectAnime(ctx context.Context, query string, results []streaming.SearchResult) bool {
	items := make([]menuItem, len(results))
	for i, r := range results {
		items[i] = menuItem{label: r.Name}
	}
	lastSel := 0
	for {
		idx, action := showMenu(
			fmt.Sprintf("Risultati per \"%s\"", query),
			items, lastSel,
			"↑↓ naviga  Enter seleziona  B nuova ricerca  Q esci",
		)
		switch action {
		case "quit":
			return true
		case "back":
			return false
		case "select":
			lastSel = idx
			selected := results[idx]

			showLoading("Caricamento dettagli: " + selected.Name)
			anime, err := p.client.GetAnime(ctx, selected.URL)
			if err != nil {
				p.showError("Errore", err.Error())
				continue
			}

			if quit := p.runAnimeMenu(ctx, anime); quit {
				return true
			}
		}
	}
}

// runAnimeMenu — menù episodi per un anime
func (p *PlayerTUI) runAnimeMenu(ctx context.Context, anime *streaming.Anime) bool {
	lastSel := 0
	for {
		isFav := slices.ContainsFunc(p.config.SavedAnime, func(animeCmp streaming.Anime) bool {
			return animeCmp.Name == anime.Name
		})

		favLabel := "Aggiungi tra i preferiti questo anime"
		if isFav {
			favLabel = "Rimuovi dai preferiti questo anime"
		}

		items := []menuItem{
			{label: "Inserisci numero episodio direttamente (veloce)"},
			{label: "Visualizza tutti gli episodi disponibili e scegline uno"},
			{label: favLabel},
		}

		idx, action := showMenu(
			fmt.Sprintf("=== %s ===", anime.Name),
			items, lastSel,
			"↑↓ naviga  Enter seleziona  B torna alla lista  Q esci",
		)
		switch action {
		case "quit":
			return true
		case "back":
			return false
		}
		lastSel = idx
		switch idx {
		case 0:
			if quit := p.runEpisodeInput(ctx, anime); quit {
				return true
			}
		case 1:
			if quit := p.runAllEpisodes(ctx, anime); quit {
				return true
			}
		case 2:
			if isFav {
				p.config.SavedAnime = slices.DeleteFunc(p.config.SavedAnime, func(a streaming.Anime) bool {
					return a.Name == anime.Name
				})
			} else {
				p.config.SavedAnime = append(p.config.SavedAnime, *anime)
			}
			err := SaveConfig(p.config)
			if err != nil {
				p.showError("Errore", "Errore nel salvataggio dei preferiti: "+err.Error())
				return true
			}
		}
	}
}

// runEpisodeInput — chiede numero episodio tramite input testuale
func (p *PlayerTUI) runEpisodeInput(ctx context.Context, anime *streaming.Anime) bool {
	for {
		tbClear()
		row := tbDrawHeader(0)
		tbPrint(2, row, colMagenta, colDefault, fmt.Sprintf("Episodio di: %s", anime.Name))
		row += 2
		tbPrint(2, row, colWhite, colDefault, "Numero episodio (es. 1, 2, 12...):")
		row++
		tbDrawSeparator(row)
		row++
		prompt := "▶ "
		tbPrint(2, row, colRed, colDefault, prompt)
		tbDrawFooter("Enter conferma  Esc torna  Q esci")
		tbFlush()

		epNum := readLine(2+len([]rune(prompt)), row, colWhite, colDefault)
		if epNum == "" {
			return false
		}
		lower := strings.ToLower(epNum)
		if lower == "q" {
			return true
		}
		if lower == "b" {
			return false
		}

		showLoading(fmt.Sprintf("Risoluzione link episodio %s...", epNum))
		episodes, err := p.client.GetEpisodes(ctx, anime, epNum)
		if err != nil {
			p.showError("Errore", err.Error())
			continue
		}

		var target *streaming.Episode
		for i := range episodes {
			if episodes[i].Number == epNum {
				target = &episodes[i]
				break
			}
		}
		if target == nil || target.StreamURL == "" {
			msg := "Stream non trovato."
			if target != nil && target.Err != nil {
				msg = target.Err.Error()
			}
			p.showError("Errore", msg)
			continue
		}

		if quit := p.runQualityMenu(target.StreamURL, target.Number, anime); quit {
			return true
		}
		return false
	}
}

// runAllEpisodes — scarica tutti gli episodi e mostra lista
func (p *PlayerTUI) runAllEpisodes(ctx context.Context, anime *streaming.Anime) bool {
	showLoading("Recupero lista completa episodi...")
	episodes, err := p.client.GetEpisodes(ctx, anime)
	if err != nil {
		p.showError("Errore", err.Error())
		return false
	}
	if len(episodes) == 0 {
		p.showError("Nessun episodio", "Nessun episodio trovato.")
		return false
	}

	items := make([]menuItem, len(episodes))
	for i, ep := range episodes {
		status := ""
		if ep.StreamURL == "" {
			status = "non disp."
		}
		items[i] = menuItem{
			label:    fmt.Sprintf("Episodio %s", ep.Number),
			subLabel: status,
		}
	}

	lastSel := 0
	for {
		idx, action := showMenu(
			fmt.Sprintf("Episodi — %s", anime.Name),
			items, lastSel,
			"↑↓ naviga  Enter riproduci  B torna  Q esci",
		)
		switch action {
		case "quit":
			return true
		case "back":
			return false
		}
		lastSel = idx
		ep := episodes[idx]
		if ep.StreamURL == "" {
			msg := "Stream non disponibile."
			if ep.Err != nil {
				msg = ep.Err.Error()
			}
			p.showError("Non disponibile", msg)
			continue
		}
		if quit := p.runQualityMenu(ep.StreamURL, ep.Number, anime); quit {
			return true
		}
	}
}

// runQualityMenu — selezione qualità upscale
func (p *PlayerTUI) runQualityMenu(streamURL, epNum string, anime *streaming.Anime) bool {
	items := make([]menuItem, len(QualityPresets))
	for i, preset := range QualityPresets {
		items[i] = menuItem{
			label:    fmt.Sprintf("%-30s", preset.Name),
			subLabel: preset.Description,
		}
	}

	lastSel := 0
	for {
		idx, action := showMenu(
			fmt.Sprintf("Qualità — Episodio %s", epNum),
			items, lastSel,
			"↑↓ naviga  Enter avvia mpv  B torna  Q esci",
		)
		switch action {
		case "quit":
			return true
		case "back":
			return false
		}
		lastSel = idx
		preset := &QualityPresets[idx]

		termbox.Close()

		p.config.LastEpisodeWatched = LastEpisodeWatched{
			Anime:         *anime,
			EpisodeNumber: epNum,
		}

		err := SaveConfig(p.config)
		if err != nil {
			p.showError("Errore salvando le configurazioni per l'ultimo anime", err.Error())
			continue
		}

		fmt.Printf("\033[2J\033[H")
		fmt.Printf("%s▶  Avvio mpv — Episodio %s — Preset: %s%s"+
			"", Green, epNum, preset.Name, Reset)
		play(streamURL, preset.ID)
		fmt.Printf("%s■  Riproduzione terminata. Premi Invio per continuare...%s", Yellow, Reset)
		fmt.Scanln()

		// Reinizializza termbox dopo mpv
		if err := termbox.Init(); err != nil {
			fmt.Fprintln(os.Stderr, "Impossibile reinizializzare termbox:", err)
			os.Exit(1)
		}
		termbox.SetInputMode(termbox.InputEsc)
		return false
	}
}

// runSettings — impostazioni provider
func (p *PlayerTUI) runSettings() bool {
	items := []menuItem{
		{label: "Provider di streaming", subLabel: p.client.Name()},
	}
	for {
		idx, action := showMenu(
			"=== Impostazioni ===",
			items, 0,
			"↑↓ naviga  Enter seleziona  B torna  Q esci",
		)
		switch action {
		case "quit":
			return true
		case "back":
			return false
		}
		_ = idx
		// unica opzione: cambio provider
		if quit := p.runSelectProvider(); quit {
			return true
		}
		// aggiorna subLabel dopo cambio
		items[0].subLabel = p.client.Name()
	}
}

// runSelectProvider — scelta provider
func (p *PlayerTUI) runSelectProvider() bool {
	providerItems := []menuItem{
		{label: "AnimeWorld", subLabel: "più stabile"},
		{label: "AnimeUnity", subLabel: "qualità più alta / FHD"},
	}
	idx, action := showMenu(
		"--- Seleziona Provider ---",
		providerItems, 0,
		"↑↓ naviga  Enter seleziona  B torna  Q esci",
	)
	switch action {
	case "quit":
		return true
	case "back":
		return false
	}

	var pref string
	var client streaming.Provider
	switch idx {
	case 0:
		pref = "animeworld"
		client = streaming.NewAnimeWorldClient()
	case 1:
		pref = "animeunity"
		client = streaming.NewAnimeUnityClient()
	}

	p.client = client

	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}
	cfg.Preference = pref
	if err := SaveConfig(cfg); err != nil {
		p.showError("Attenzione", "Impossibile salvare la preferenza: "+err.Error())
	}
	return false
}

// showError — mostra un messaggio d'errore e attende un tasto
func (p *PlayerTUI) showError(title, msg string) {
	tbClear()
	row := tbDrawHeader(0)
	row++
	tbPrintCentered(row, colRed, colDefault, "✗  "+title)
	row += 2

	w := screenWidth()
	words := strings.Fields(msg)
	line := ""
	for _, wd := range words {
		if len(line)+len(wd)+1 > w-4 {
			tbPrintCentered(row, colWhite, colDefault, line)
			row++
			line = wd
		} else {
			if line != "" {
				line += " "
			}
			line += wd
		}
	}
	if line != "" {
		tbPrintCentered(row, colWhite, colDefault, line)
	}

	tbDrawFooter("Premi un tasto per continuare...")
	tbFlush()
	termbox.PollEvent()
}
