package main

import (
	"UpscalerCLI/streaming"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	if _, err := exec.LookPath("mpv"); err != nil {
		fmt.Println("Errore: 'mpv' non è installato o non è presente nel PATH di sistema.")
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Uso: anime <video/url> <qualità>")
		fmt.Println("Usa 'anime -player' per avviare la modalità player interattivo.")
		fmt.Println("Usa 'anime help' per maggiori informazioni.")
		return
	}

	first := strings.ToLower(strings.Trim(os.Args[1], " "))

	if first == "-player" {
		handlePlayer()
		return
	}

	if strings.Contains(first, "help") {
		help()
		return
	}

	if len(os.Args) < 3 {
		fmt.Println("Uso: anime <video> <qualità>")
		return
	}

	second, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Il secondo parametro deve essere un intero.")
		return
	}

	play(first, second)
}

func help() {
	var sb strings.Builder
	sb.WriteString("Uso:\n")
	sb.WriteString(WrapIn("--Puoi anche cercare e vedere direttamente da qui anime senza scaricarli, aggiungi il flag : '-player' --", Green))
	sb.WriteString("\n  anime <percorso_video / url con file diretto> <qualità>\n\nQualità disponibili:\n")
	for _, p := range QualityPresets {
		sb.WriteString(fmt.Sprintf("  %d - %-32s (%s)\n", p.ID, p.Name, p.Description))
	}
	fmt.Print(sb.String())
}

func play(url string, qualityNum int) {
	preset := GetPresetByID(qualityNum)
	if preset == nil {
		fmt.Printf("Qualità non valida. Seleziona un valore da 1 a %d.\n", len(QualityPresets))
		return
	}

	shaderDir, err := ensureShaders()
	if err != nil {
		fmt.Printf("Attenzione: impossibile caricare gli shader interni (%v). Verranno usati quelli di default di mpv se disponibili.\n", err)
	}

	resolveShader := func(name string) string {
		if shaderDir != "" {
			return getShaderPath(shaderDir, name)
		}
		return "~~/shaders/" + name
	}

	shaders := make([]string, len(preset.ShaderFiles))
	for i, file := range preset.ShaderFiles {
		shaders[i] = resolveShader(file)
	}

	fmt.Printf("Video:   %s\n", url)
	fmt.Printf("Qualità: [%d] %s (%s)\n", preset.ID, preset.Name, preset.Description)

	quality := "--glsl-shaders=" + strings.Join(shaders, string(os.PathListSeparator))

	mpvArgs := []string{
		url,
		quality,
	}

	// Header HTTP richiesti da VixCloud / vix-content / AnimeUnity
	if strings.Contains(url, "vixcloud") || strings.Contains(url, "vix-content") || strings.Contains(url, "animeunity") {
		mpvArgs = append(mpvArgs,
			"--referrer=https://vixcloud.cc/",
			"--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/27.0.1 Safari/605.1.15",
		)
	} else {
		mpvArgs = append(mpvArgs,
			"--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/27.0.1 Safari/605.1.15",
		)
	}

	cmd := exec.Command(
		"mpv",
		mpvArgs...,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		fmt.Println("Errore durante l'esecuzione di mpv:", err)
		return
	}
}

func handlePlayer() {
	configs, err := LoadConfig()
	if err != nil {
		fmt.Println("Attenzione: impossibile caricare la configurazione. Verrà usata quella predefinita.")
		configs = DefaultConfig()
	}

	provider := getProviderByConfig(configs)

	player := NewPlayerTUI(provider)
	player.Run()
}

func getProviderByConfig(config *Config) streaming.Provider {
	switch config.Preference {
	case "animeunity":
		return streaming.NewAnimeUnityClient()
	default:
		return streaming.NewAnimeWorldClient()
	}
}
