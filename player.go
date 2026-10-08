package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"UpscalerCLI/streaming"
)

const (
	Red    string = "\033[31m"
	Green  string = "\033[32m"
	Blue   string = "\033[34m"
	Yellow string = "\033[33m"
	Cyan   string = "\033[36m"
	Bold   string = "\033[1m"
	Reset  string = "\033[0m"
)

func WrapIn(s string, fix string) string {
	return fix + s + Reset
}

func readTrimmed(scanner *bufio.Scanner) string {
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func printTitle() {
	fmt.Println(WrapIn("                                                                                              "+
		"\n                                                                                              "+
		"\n                L.                                              ,;                .   G:      "+
		"\n                EW:        ,ft t                              f#i                ,W   E#,    :"+
		"\n             .. E##;       t#E Ej            ..       :     .E#t                i##   E#t  .GE"+
		"\n            ;W, E###t      t#E E#,          ,W,     .Et    i#W,                f###   E#t j#K;"+
		"\n           j##, E#fE#f     t#E E#t         t##,    ,W#t   L#D.                G####   E#GK#f  "+
		"\n          G###, E#t D#G    t#E E#t        L###,   j###t :K#Wfff;            .K#Ki##   E##D.   "+
		"\n        :E####, E#t  f#E.  t#E E#t      .E#j##,  G#fE#t i##WLLLLt          ,W#D.,##   E##Wi   "+
		"\n       ;W#DG##, E#t   t#K: t#E E#t     ;WW; ##,:K#i E#t  .E#L             i##E,,i##,  E#jL#D: "+
		"\n      j###DW##, E#t    ;#W,t#E E#t    j#E.  ##f#W,  E#t    f#E:          ;DDDDDDE##DGiE#t ,K#j"+
		"\n     G##i,,G##, E#t     :K#D#E E#t  .D#L    ###K:   E#t     ,WW;                ,##   E#t   jD"+
		"\n   :K#K:   L##, E#t      .E##E E#t :K#t     ##D.    E#t      .D#;               ,##   j#t     "+
		"\n  ;##D.    L##, ..         G#E E#t ...      #G      ..         tt               .E#    ,;     "+
		"\n  ,,,      .,,              fE ,;.          j                                     t          "+
		"\n                             ,                                                                 By Andrea Maruca", Red))
}

func handlePlayer() {
	scanner := bufio.NewScanner(os.Stdin)
	client := streaming.NewClient()

	printTitle()

	for {
		fmt.Println()
		fmt.Print(WrapIn("Cerca un anime (o 'q' per uscire): ", Yellow))
		query := readTrimmed(scanner)
		if strings.EqualFold(query, "q") || query == "" {
			fmt.Println("Uscita dal player.")
			return
		}

		fmt.Println(WrapIn("Ricerca in corso...", Cyan))
		ctx := context.Background()
		results, err := client.Search(ctx, query)
		if err != nil {
			fmt.Println(WrapIn("Errore durante la ricerca: "+err.Error(), Red))
			continue
		}

		if len(results) == 0 {
			fmt.Println(WrapIn("Nessun anime trovato per la ricerca effettuata.", Yellow))
			continue
		}

		for {
			fmt.Println()
			fmt.Println(WrapIn(fmt.Sprintf("--- Risultati trovati per '%s' ---", query), Green))
			for i, res := range results {
				fmt.Printf("[%s%d%s] %s\n", Yellow, i+1, Reset, res.Name)
			}
			fmt.Println(WrapIn("[b] Torna alla ricerca | [q] Esci", Cyan))

			fmt.Print(WrapIn("Seleziona il numero dell'anime: ", Yellow))
			input := readTrimmed(scanner)
			if strings.EqualFold(input, "q") {
				fmt.Println("Uscita dal player.")
				return
			}
			if strings.EqualFold(input, "b") {
				break
			}

			idx, err := strconv.Atoi(input)
			if err != nil || idx < 1 || idx > len(results) {
				fmt.Println(WrapIn("Selezione non valida.", Red))
				continue
			}

			selectedResult := results[idx-1]
			fmt.Println(WrapIn(fmt.Sprintf("Caricamento dettagli di: %s...", selectedResult.Name), Cyan))

			animeObj, err := client.GetAnime(ctx, selectedResult.URL)
			if err != nil {
				fmt.Println(WrapIn("Errore nel recupero dell'anime: "+err.Error(), Red))
				continue
			}

			handleAnimeEpisodes(scanner, client, animeObj)
		}
	}
}

func handleAnimeEpisodes(scanner *bufio.Scanner, client *streaming.Client, anime *streaming.Anime) {
	ctx := context.Background()

	for {
		fmt.Println()
		fmt.Println(WrapIn(fmt.Sprintf("=== %s ===", anime.Name), Green))
		fmt.Println(WrapIn("Scelte disponibili per gli episodi:", Cyan))
		fmt.Println("1. Inserisci numero episodio direttamente (veloce)")
		fmt.Println("2. Visualizza tutti gli episodi disponibili e scegline uno")
		fmt.Println(WrapIn("[b] Torna alla lista anime | [q] Esci", Cyan))

		fmt.Print(WrapIn("Scegli un'opzione o inserisci direttamente il numero dell'episodio (es. 1, 2, ...): ", Yellow))
		choice := readTrimmed(scanner)
		if strings.EqualFold(choice, "q") {
			fmt.Println("Uscita dal player.")
			os.Exit(0)
		}
		if strings.EqualFold(choice, "b") {
			return
		}

		if choice == "2" {
			fmt.Println(WrapIn("Recupero lista completa degli episodi in corso (potrebbe richiedere qualche secondo)...", Cyan))
			episodes, err := client.GetEpisodes(ctx, anime)
			if err != nil {
				fmt.Println(WrapIn("Errore nel caricamento degli episodi: "+err.Error(), Red))
				continue
			}
			if len(episodes) == 0 {
				fmt.Println(WrapIn("Nessun episodio trovato.", Red))
				continue
			}

			selectAndPlayEpisodeList(scanner, episodes)
		} else {
			epNum := choice
			if choice == "1" {
				fmt.Print(WrapIn("Inserisci il numero dell'episodio da guardare: ", Yellow))
				epNum = readTrimmed(scanner)
				if strings.EqualFold(epNum, "b") {
					continue
				}
				if strings.EqualFold(epNum, "q") {
					fmt.Println("Uscita dal player.")
					os.Exit(0)
				}
			}

			if epNum == "" {
				continue
			}

			fmt.Println(WrapIn(fmt.Sprintf("Risoluzione link per episodio %s...", epNum), Cyan))
			episodes, err := client.GetEpisodes(ctx, anime, epNum)
			if err != nil {
				fmt.Println(WrapIn("Errore nel recupero dell'episodio: "+err.Error(), Red))
				continue
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
				fmt.Println(WrapIn("Errore: "+errMsg, Red))
				continue
			}

			askQualityAndPlay(scanner, targetEp.StreamURL, targetEp.Number)
		}
	}
}

func selectAndPlayEpisodeList(scanner *bufio.Scanner, episodes []streaming.Episode) {
	for {
		fmt.Println()
		fmt.Println(WrapIn("--- Episodi Disponibili ---", Green))
		for i, ep := range episodes {
			status := ""
			if ep.StreamURL == "" {
				status = " (Non disponibile)"
			}
			fmt.Printf("[%s%d%s] Episodio %s%s\n", Yellow, i+1, Reset, ep.Number, status)
		}
		fmt.Println(WrapIn("[b] Torna indietro | [q] Esci", Cyan))

		fmt.Print(WrapIn("Seleziona il numero della lista da riprodurre: ", Yellow))
		input := readTrimmed(scanner)
		if strings.EqualFold(input, "q") {
			fmt.Println("Uscita dal player.")
			os.Exit(0)
		}
		if strings.EqualFold(input, "b") {
			return
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(episodes) {
			fmt.Println(WrapIn("Selezione non valida.", Red))
			continue
		}

		ep := episodes[idx-1]
		if ep.StreamURL == "" {
			errMsg := "Stream non disponibile."
			if ep.Err != nil {
				errMsg = ep.Err.Error()
			}
			fmt.Println(WrapIn("Errore: "+errMsg, Red))
			continue
		}

		askQualityAndPlay(scanner, ep.StreamURL, ep.Number)
	}
}

func askQualityAndPlay(scanner *bufio.Scanner, streamURL string, epNum string) {
	for {
		fmt.Println()
		fmt.Println(WrapIn(fmt.Sprintf("Configura qualità per Episodio %s:", epNum), Yellow))
		for _, p := range QualityPresets {
			fmt.Printf("[%s%d%s] %-28s - %s\n", Yellow, p.ID, Reset, p.Name, p.Description)
		}
		fmt.Println(WrapIn("[b] Annulla | [q] Esci", Cyan))

		fmt.Printf("%sInserisci la qualità desiderata (1-%d): %s", Yellow, len(QualityPresets), Reset)
		input := readTrimmed(scanner)
		if strings.EqualFold(input, "q") {
			fmt.Println("Uscita dal player.")
			os.Exit(0)
		}
		if strings.EqualFold(input, "b") {
			return
		}

		qualityNum, err := strconv.Atoi(input)
		preset := GetPresetByID(qualityNum)
		if err != nil || preset == nil {
			fmt.Printf("%sQualità non valida. Seleziona un valore tra 1 e %d.%s\n", Red, len(QualityPresets), Reset)
			continue
		}

		fmt.Println(WrapIn(fmt.Sprintf("Avvio mpv per l'episodio %s con preset '%s'...", epNum, preset.Name), Green))
		play(streamURL, qualityNum)
		fmt.Println(WrapIn(fmt.Sprintf("Riproduzione episodio %s terminata. Puoi scegliere un altro episodio o qualità.", epNum), Green))
		return
	}
}
