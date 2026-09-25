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
		fmt.Println("Uso: anime <video> <qualità>")
		fmt.Println("Usa 'anime help' per maggiori informazioni.")
		return
	}

	first := os.Args[1]

	if strings.Contains(first, "help") {
		fmt.Println("Uso:")
		fmt.Println("  anime <percorso_video> <qualità>")
		fmt.Println()
		fmt.Println("Qualità:")
		fmt.Println("  1 - LOW")
		fmt.Println("  2 - MID")
		fmt.Println("  3 - MID-HIGH")
		fmt.Println("  4 - HIGH")
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

	var shaders []string
	var qualityName string

	switch second {

	case 1:
		qualityName = "LOW"

		shaders = []string{
			"~~/shaders/Anime4K_Clamp_Highlights.glsl",
			"~~/shaders/Anime4K_Restore_CNN_Soft_M.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_S.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x2.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x4.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_S.glsl",
		}

	case 2:
		qualityName = "MID"

		shaders = []string{
			"~~/shaders/Anime4K_Clamp_Highlights.glsl",
			"~~/shaders/Anime4K_Restore_CNN_M.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_M.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x2.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x4.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_M.glsl",
		}

	case 3:
		qualityName = "MID-HIGH"

		shaders = []string{
			"~~/shaders/Anime4K_Clamp_Highlights.glsl",
			"~~/shaders/Anime4K_Restore_CNN_Soft_M.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_VL.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x2.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x4.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_M.glsl",
		}

	case 4:
		qualityName = "HIGH"

		shaders = []string{
			"~~/shaders/Anime4K_Clamp_Highlights.glsl",
			"~~/shaders/Anime4K_Restore_CNN_VL.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_VL.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x2.glsl",
			"~~/shaders/Anime4K_AutoDownscalePre_x4.glsl",
			"~~/shaders/Anime4K_Upscale_CNN_x2_M.glsl",
		}

	default:
		fmt.Println("Qualità non valida. Usa un valore da 1 a 4.")
		return
	}

	fmt.Printf("Video: %s\n", first)
	fmt.Printf("Qualità: %s\n", qualityName)

	quality := "--glsl-shaders=" + strings.Join(shaders, ":")

	cmd := exec.Command(
		"mpv",
		first,
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
