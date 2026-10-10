package main

import (
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

	cmd := exec.Command(
		"mpv",
		url,
		quality,
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
