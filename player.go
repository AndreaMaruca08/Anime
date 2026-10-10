// player.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"UpscalerCLI/streaming"
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

// -----------------------------------------------------------------------------
// UI / Terminal Helpers
// -----------------------------------------------------------------------------

type UI struct {
	scanner *bufio.Scanner
	writer  io.Writer
}

func NewUI(r io.Reader, w io.Writer) *UI {
	return &UI{
		scanner: bufio.NewScanner(r),
		writer:  w,
	}
}

func (ui *UI) Print(s string) {
	fmt.Fprint(ui.writer, s)
}

func (ui *UI) Println(s string) {
	fmt.Fprintln(ui.writer, s)
}

func (ui *UI) ReadInput(prompt string) string {
	if prompt != "" {
		ui.Print(prompt)
	}
	if ui.scanner.Scan() {
		return strings.TrimSpace(ui.scanner.Text())
	}
	return ""
}

func (ui *UI) PrintBanner() {
	ui.Println("\u001B[0;37m    ▄▄\u001B[0;90m▄\u001B[0;37m     \u001B[0;97m▄▄\u001B[0;37m▄▄    \u001B[0;97m▄\u001B[0;37m▄\u001B[0;90m▄▄▄\u001B[0;37m  \u001B[0;97m▄▄\u001B[0;37m▄▄▄\u001B[0;90m▄▄\u001B[0;37m  \u001B[0;97m▄▄\u001B[0;37m▄▄▄      \u001B[0;90m▄▄▄\u001B[0;37m  \u001B[0;97m▄▄\u001B[0;37m▄▄▄\u001B[0;90m▄▄▄▄▄\u001B[0;37m    \u001B[0;97m▄▄\u001B[0;37m▄▄▄▄\u001B[0;90m▄\u001B[0;37m \u001B[0;97m▄\u001B[0;37m▄\u001B[0;90m▄\u001B[0;37m▄▄   \u001B[0;97m▄\u001B[0;37m▄▄▄\u001B[0;90m▄\u001B[0;37m   \u001B[0;90m▄▄▄▄\u001B[0;37m \u001B[0m\n\u001B[0;37m  \u001B[0;97m▄\u001B[0;97;41m▀\u001B[0;91;41m█▓▓\u001B[0;90;41m▀\u001B[0;90m▄\u001B[0;37m  \u001B[0;97m▀\u001B[0;97;41m▄\u001B[0;91;41m░▒▒▒\u001B[0;90m▄\u001B[0;37m ▀\u001B[0;90;41m▄\u001B[0;31m▓\u001B[0;91;41m▒░\u001B[0;90;41m▀\u001B[0;90m▌\u001B[0;97m▀\u001B[0;97;41m▄\u001B[0;91;41m░▒▒▒▓\u001B[0;90m█▀\u001B[0;97m▀\u001B[0;97;41m▄\u001B[0;91;41m░▒▒▒\u001B[0;90m█\u001B[0;37m   \u001B[0;97m▄\u001B[0;37;41m▀\u001B[0;91;41m▒░\u001B[0;90;41m▄\u001B[0;90m▀\u001B[0;97m▀\u001B[0;97;41m▄\u001B[0;91;41m░▒▒▒▓▓▓█\u001B[0;90;41m▐\u001B[0;37m    \u001B[0;97m▓\u001B[0;91;41m░▒▒▒\u001B[0;90m█\u001B[0;37m  \u001B[0;97;47m░\u001B[0;91;41m▒▒░\u001B[0;90;41m▄\u001B[0;37m  \u001B[0;97;47m▌\u001B[0;91;41m░▒▒▒\u001B[0;90m▌\u001B[0;37m  \u001B[0;90;41m▄\u001B[0;91;41m▒▒░\u001B[0;90;41m▄\u001B[0;90m▌\u001B[0m\n\u001B[0;37m  \u001B[0;97;41m▌\u001B[0;91;41m▓\u001B[0;90;41m▄\u001B[0;90m▀\u001B[0;90;41m▄\u001B[0;91;41m▒\u001B[0;90;41m▐\u001B[0;37m    \u001B[0;31;47m▐\u001B[0;91;41m░▒▓▒\u001B[0;90m█\u001B[0;37m  \u001B[0;90;47m▌\u001B[0;91;41m░\u001B[0;90;41m▓\u001B[0;31m█\u001B[0;90m▌\u001B[0;37m  \u001B[0;31;47m▐\u001B[0;91;41m░▒\u001B[0;90;41m▄\u001B[0;90m█\u001B[0;37m    \u001B[0;31;47m▐\u001B[0;91;41m░▒▓▒\u001B[0;90;47m▄\u001B[0;37m▄\u001B[0;37;41m▀\u001B[0;91;41m░▓░\u001B[0;90;41m▐\u001B[0;90m▌\u001B[0;37m   \u001B[0;31;47m▐\u001B[0;91;41m░▒\u001B[0;90;41m▄\u001B[0;90m▀▀▀\u001B[0;90;41m▄\u001B[0;90m█\u001B[0;37m    \u001B[0;97m▐\u001B[0;31;47m▐\u001B[0;91;41m░▒\u001B[0;90;41m▄\u001B[0;90m▌\u001B[0;37m  ▐\u001B[0;90;41m▌\u001B[0;91;41m░\u001B[0;90;41m░▐\u001B[0;37m  \u001B[0;97m▐\u001B[0;31;47m▐\u001B[0;91;41m░▒\u001B[0;90;41m▄\u001B[0;37m   ▐\u001B[0;90;41m▌\u001B[0;91;41m░\u001B[0;90;41m▐\u001B[0;90m▀\u001B[0;37m \u001B[0m\n\u001B[0;37m ▐\u001B[0;91;41m▒▒\u001B[0;90m▌\u001B[0;37m ▐\u001B[0;91;41m░▒\u001B[0;90m▌\u001B[0;37m   \u001B[0;31;47m▐\u001B[0;31m▓\u001B[0;91;41m░▒▓▒\u001B[0;90m▌\u001B[0;37m \u001B[0;90;47m▐\u001B[0;90;41m▓░\u001B[0;31m█\u001B[0;90m▌\u001B[0;37m  \u001B[0;31;47m▐\u001B[0;31m▓\u001B[0;31;41m \u001B[0;90;41m▓\u001B[0;90m▌\u001B[0;37m    \u001B[0;31;47m▐\u001B[0;31m▓\u001B[0;31;41m \u001B[0;90;41m▐\u001B[0;90m█\u001B[0;31m█▒█▓\u001B[0;90;41m░▓\u001B[0;90m█\u001B[0;37m    \u001B[0;31;47m▐\u001B[0;31m▓\u001B[0;31;41m \u001B[0;90;41m▐\u001B[0;90m▄▄▄\u001B[0;37m       \u001B[0;31;47m▐\u001B[0;31m▓\u001B[0;31;41m \u001B[0;90;41m▐\u001B[0;90m▄\u001B[0;37m▄▄\u001B[0;97;41m▀\u001B[0;90;41m░▓░\u001B[0;90m█\u001B[0;37m   \u001B[0;31;47m▐\u001B[0;31m▓\u001B[0;31;41m \u001B[0;90;41m▐\u001B[0;90m▄\u001B[0;37m▄▄\u001B[0;97;41m▀\u001B[0;90;41m░\u001B[0;90m▀\u001B[0;37m   \u001B[0m\n\u001B[0;37m \u001B[0;90m▐\u001B[0;31m█▓\u001B[0;90;41m▀\u001B[0;37;41m▀\u001B[0;97;41m▀\u001B[0;31m▓█\u001B[0;90m▌\u001B[0;37m   \u001B[0;90;41m▌\u001B[0;31m▒▓\u001B[0;90m█▌\u001B[0;31m█\u001B[0;91;41m░\u001B[0;31m▄▐▒▓▓\u001B[0;90m█\u001B[0;37m  \u001B[0;90;41m▌\u001B[0;31m▒▓▓\u001B[0;90m▌\u001B[0;37m    \u001B[0;90;41m▌\u001B[0;31m▒▓\u001B[0;90m█▌▀\u001B[0;90;41m▄\u001B[0;90m▀█\u001B[0;31m▓\u001B[0;90;41m░▐\u001B[0;90m▌\u001B[0;37m   \u001B[0;90;41m▌\u001B[0;31m▒▓▓▒░\u001B[0;90m▌\u001B[0;37m        ▀\u001B[0;90;41m▄\u001B[0;31m▓▒░\u001B[0;90m░\u001B[0;31m░▓\u001B[0;90;41m░▐▐\u001B[0;37m   \u001B[0;90;41m▌\u001B[0;31m▒▓▓▒░\u001B[0;37m \u001B[0;31m░\u001B[0;90;41m▀\u001B[0;90m▄\u001B[0;37m   \u001B[0m\n\u001B[0;37m \u001B[0;90;41m▌\u001B[0;31m▓▒\u001B[0;90m▄▄▄\u001B[0;31m▒▓\u001B[0;90;41m▐\u001B[0;37m   \u001B[0;90m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90m█\u001B[0;37m \u001B[0;90m▐\u001B[0;31m░▒░\u001B[0;37m \u001B[0;31m░░\u001B[0;90m▌\u001B[0;37m  \u001B[0;90m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90;41m▐\u001B[0;90m▌\u001B[0;37m    \u001B[0;90m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90m█\u001B[0;37m    \u001B[0;90m█\u001B[0;31m░▓░\u001B[0;90m█\u001B[0;37m   \u001B[0;90m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90;41m▐\u001B[0;90m▀▀\u001B[0;37m           \u001B[0;90m▀▀▀▀\u001B[0;37;41m▄\u001B[0;31m░▓░\u001B[0;90m█\u001B[0;37m   \u001B[0;90m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90m▄█▀▀\u001B[0;37m▄\u001B[0;31m░▓\u001B[0;90m▀▄\u001B[0;37m \u001B[0m\n\u001B[0;37m \u001B[0;90m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90m▌\u001B[0;37m ▐ \u001B[0;31m░\u001B[0;90m▐\u001B[0;37m  \u001B[0;90m▐▌\u001B[0;37m  \u001B[0;90m▐▌\u001B[0;37m \u001B[0;90m▀▄\u001B[0;37m \u001B[0;31m░\u001B[0;37m \u001B[0;90m▐▌\u001B[0;37m  \u001B[0;90m▌\u001B[0;37m   \u001B[0;90m█\u001B[0;37m   \u001B[0;90m▐▌\u001B[0;37m  \u001B[0;90m▐▌\u001B[0;37m   \u001B[0;97m▐\u001B[0;37m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90m▐▌\u001B[0;37m  \u001B[0;90m▌\u001B[0;37m  \u001B[0;90m▀▄▄▄▀█\u001B[0;37m            █ \u001B[0;31m░\u001B[0;37m \u001B[0;90m█\u001B[0;37m  \u001B[0;90m▐▌\u001B[0;37m  \u001B[0;90m▐▌\u001B[0;37m  \u001B[0;97m▐\u001B[0;37m▌\u001B[0;31m░\u001B[0;37m \u001B[0;90m▐▌\u001B[0m\n\u001B[0;37m▐  \u001B[0;90m▐\u001B[0;37m   ▌  \u001B[0;90m▌\u001B[0;37m \u001B[0;90m█\u001B[0;37m    \u001B[0;90m▀▄\u001B[0;37m \u001B[0;90m▀▄\u001B[0;37m  \u001B[0;90m█\u001B[0;37m \u001B[0;90m▄▀\u001B[0;37m  \u001B[0;90m░▓█▄\u001B[0;37m  \u001B[0;90m█\u001B[0;37m    \u001B[0;90m▀▄\u001B[0;37m  █    \u001B[0;90m█▄▀\u001B[0;37m        \u001B[0;90m▐\u001B[0;37m            \u001B[0;90;47m▓\u001B[0;90m░\u001B[0;37m  \u001B[0;90m▐▌\u001B[0;37m \u001B[0;90m█\u001B[0;37m    \u001B[0;90m▀▄\u001B[0;37m █    \u001B[0;90m█\u001B[0m\n\u001B[0;37m▀\u001B[0;90m▀▀▀\u001B[0;37m   ▀\u001B[0;90m▀▀▀\u001B[0;37m  \u001B[0;90m▀▀▀▀▀\u001B[0;37m    \u001B[0;90m▀▀\u001B[0;37m   \u001B[0;90m▀▀▀▀▀▀\u001B[0;37m    \u001B[0;90m▀▀▀▀▀\u001B[0;37m    \u001B[0;90m▀▀▀▀\u001B[0;37m  \u001B[0;90m▀▀▀▀▀▀▀▀▀▀\u001B[0;37m            \u001B[0;90m▀▀▀▀▀\u001B[0;37m   \u001B[0;90m▀▀▀▀▀\u001B[0;37m   \u001B[0;90m▀▀▀▀\u001B[0;37m \u001B[0m\nBy Andrea Maruca  | https://github.com/AndreaMaruca08")
}

type PlayerCLI struct {
	ui     *UI
	client *streaming.Client
}

func NewPlayerCLI(ui *UI, client *streaming.Client) *PlayerCLI {
	return &PlayerCLI{
		ui:     ui,
		client: client,
	}
}

func isQuitInput(input string) bool {
	return strings.EqualFold(input, "q")
}

func isBackInput(input string) bool {
	return strings.EqualFold(input, "b")
}

func parseSelection(input string, max int) (int, bool) {
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > max {
		return 0, false
	}
	return idx - 1, true
}

func (p *PlayerCLI) Run() {
	p.ui.PrintBanner()

	for {
		p.ui.Print("\n" + WrapIn("Cerca un anime (o 'q' per uscire): ", Yellow))
		query := p.ui.ReadInput("")
		if isQuitInput(query) || query == "" {
			p.ui.Println("Uscita dal player.")
			return
		}

		p.ui.Println(WrapIn("Ricerca in corso...", Cyan))
		ctx := context.Background()
		results, err := p.client.Search(ctx, query)
		if err != nil {
			p.ui.Println(WrapIn("Errore durante la ricerca: "+err.Error(), Red))
			continue
		}

		if len(results) == 0 {
			p.ui.Println(WrapIn("Nessun anime trovato per la ricerca effettuata.", Yellow))
			continue
		}

		if shouldExit := p.selectAnimeLoop(ctx, query, results); shouldExit {
			return
		}

		p.ui.Println("")
		p.ui.PrintBanner()
	}
}

func (p *PlayerCLI) selectAnimeLoop(ctx context.Context, query string, results []streaming.SearchResult) bool {
	for {
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(WrapIn(fmt.Sprintf("--- Risultati trovati per '%s' ---", query), Green))
		sb.WriteString("\n")
		for i, res := range results {
			sb.WriteString(fmt.Sprintf("[%s%d%s] %s\n", Yellow, i+1, Reset, res.Name))
		}
		sb.WriteString(WrapIn("[b] Torna alla ricerca | [q] Esci", Cyan))
		sb.WriteString("\n")
		sb.WriteString(WrapIn("Seleziona il numero dell'anime: ", Yellow))

		p.ui.Print(sb.String())
		input := p.ui.ReadInput("")

		if isQuitInput(input) {
			p.ui.Println("Uscita dal player.")
			return true
		}
		if isBackInput(input) {
			return false
		}

		idx, ok := parseSelection(input, len(results))
		if !ok {
			p.ui.Println(WrapIn("Selezione non valida.", Red))
			continue
		}

		selectedResult := results[idx]
		p.ui.Println(WrapIn(fmt.Sprintf("Caricamento dettagli di: %s...", selectedResult.Name), Cyan))

		animeObj, err := p.client.GetAnime(ctx, selectedResult.URL)
		if err != nil {
			p.ui.Println(WrapIn("Errore nel recupero dell'anime: "+err.Error(), Red))
			continue
		}

		p.handleAnimeEpisodes(ctx, animeObj)
	}
}

func (p *PlayerCLI) handleAnimeEpisodes(ctx context.Context, anime *streaming.Anime) {
	for {
		var menuSb strings.Builder
		menuSb.WriteString("\n")
		menuSb.WriteString(WrapIn(fmt.Sprintf("=== %s ===", anime.Name), Green))
		menuSb.WriteString("\n")
		menuSb.WriteString(WrapIn("Scelte disponibili per gli episodi:", Cyan))
		menuSb.WriteString("\n1. Inserisci numero episodio direttamente (veloce)\n")
		menuSb.WriteString("2. Visualizza tutti gli episodi disponibili e scegline uno\n")
		menuSb.WriteString("[b] Torna alla lista anime | [q] Esci\n")
		menuSb.WriteString(WrapIn("Scegli un'opzione o inserisci direttamente il numero dell'episodio (es. 1, 2, ...): \n", Yellow))

		p.ui.Print(menuSb.String())
		choice := p.ui.ReadInput("")

		if isQuitInput(choice) {
			p.ui.Println("Uscita dal player.")
			os.Exit(0)
		}
		if isBackInput(choice) {
			return
		}

		if choice == "2" {
			p.fetchAndSelectFromAllEpisodes(ctx, anime)
		} else {
			p.playSingleEpisodeByInput(ctx, anime, choice)
		}
	}
}

func (p *PlayerCLI) fetchAndSelectFromAllEpisodes(ctx context.Context, anime *streaming.Anime) {
	p.ui.Println(WrapIn("Recupero lista completa degli episodi in corso (potrebbe richiedere qualche secondo)...", Cyan))
	episodes, err := p.client.GetEpisodes(ctx, anime)
	if err != nil {
		p.ui.Println(WrapIn("Errore nel caricamento degli episodi: "+err.Error(), Red))
		return
	}
	if len(episodes) == 0 {
		p.ui.Println(WrapIn("Nessun episodio trovato.", Red))
		return
	}

	p.selectAndPlayEpisodeList(episodes)
}

func (p *PlayerCLI) playSingleEpisodeByInput(ctx context.Context, anime *streaming.Anime, choice string) {
	epNum := choice
	if choice == "1" {
		epNum = p.ui.ReadInput(WrapIn("Inserisci il numero dell'episodio da guardare: ", Yellow))
		if isBackInput(epNum) {
			return
		}
		if isQuitInput(epNum) {
			p.ui.Println("Uscita dal player.")
			os.Exit(0)
		}
	}

	if epNum == "" {
		return
	}

	p.ui.Println(WrapIn(fmt.Sprintf("Risoluzione link per episodio %s...", epNum), Cyan))
	episodes, err := p.client.GetEpisodes(ctx, anime, epNum)
	if err != nil {
		p.ui.Println(WrapIn("Errore nel recupero dell'episodio: "+err.Error(), Red))
		return
	}

	var targetEp *streaming.Episode
	for i := range episodes {
		if episodes[i].Number == epNum {
			targetEp = &episodes[i]
			break
		}
	}

	if targetEp == nil || targetEp.StreamURL == "" {
		errMsg := "Stream non trovato per questo episodio."
		if targetEp != nil && targetEp.Err != nil {
			errMsg = targetEp.Err.Error()
		}
		p.ui.Println(WrapIn("Errore: "+errMsg, Red))
		return
	}

	p.askQualityAndPlay(targetEp.StreamURL, targetEp.Number)
}

// selectAndPlayEpisodeList mostra la lista di tutti gli episodi e permette di selezionarne uno.
func (p *PlayerCLI) selectAndPlayEpisodeList(episodes []streaming.Episode) {
	for {
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(WrapIn("--- Episodi Disponibili ---", Green))
		sb.WriteString("\n")
		for i, ep := range episodes {
			status := ""
			if ep.StreamURL == "" {
				status = " (Non disponibile)"
			}
			sb.WriteString(fmt.Sprintf("[%s%d%s] Episodio %s%s\n", Yellow, i+1, Reset, ep.Number, status))
		}
		sb.WriteString(WrapIn("[b] Torna indietro | [q] Esci", Cyan))
		sb.WriteString("\n")
		sb.WriteString(WrapIn("Seleziona il numero della lista da riprodurre: ", Yellow))

		p.ui.Print(sb.String())
		input := p.ui.ReadInput("")

		if isQuitInput(input) {
			p.ui.Println("Uscita dal player.")
			os.Exit(0)
		}
		if isBackInput(input) {
			return
		}

		idx, ok := parseSelection(input, len(episodes))
		if !ok {
			p.ui.Println(WrapIn("Selezione non valida.", Red))
			continue
		}

		ep := episodes[idx]
		if ep.StreamURL == "" {
			errMsg := "Stream non disponibile."
			if ep.Err != nil {
				errMsg = ep.Err.Error()
			}
			p.ui.Println(WrapIn("Errore: "+errMsg, Red))
			continue
		}

		p.askQualityAndPlay(ep.StreamURL, ep.Number)
	}
}

func (p *PlayerCLI) askQualityAndPlay(streamURL string, epNum string) {
	for {
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(WrapIn(fmt.Sprintf("Configura qualità per Episodio %s:", epNum), Yellow))
		sb.WriteString("\n")
		for _, preset := range QualityPresets {
			sb.WriteString(fmt.Sprintf("[%s%d%s] %-28s - %s\n", Yellow, preset.ID, Reset, preset.Name, preset.Description))
		}
		sb.WriteString(WrapIn("[b] Annulla | [q] Esci", Cyan))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("%sInserisci la qualità desiderata (1-%d): %s", Yellow, len(QualityPresets), Reset))

		p.ui.Print(sb.String())
		input := p.ui.ReadInput("")

		if isQuitInput(input) {
			p.ui.Println("Uscita dal player.")
			os.Exit(0)
		}
		if isBackInput(input) {
			return
		}

		qualityNum, err := strconv.Atoi(input)
		preset := GetPresetByID(qualityNum)
		if err != nil || preset == nil {
			p.ui.Println(WrapIn(fmt.Sprintf("Qualità non valida. Seleziona un valore tra 1 e %d.", len(QualityPresets)), Red))
			continue
		}

		p.ui.Println(WrapIn(fmt.Sprintf("Avvio mpv per l'episodio %s con preset '%s'...", epNum, preset.Name), Green))
		play(streamURL, qualityNum)
		p.ui.Println(WrapIn(fmt.Sprintf("Riproduzione episodio %s terminata. Puoi scegliere un altro episodio o qualità.", epNum), Green))
		return
	}
}
