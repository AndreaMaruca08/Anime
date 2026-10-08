package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed shaders/*.glsl
var embeddedShaders embed.FS

var (
	extractedShaderDir string
	extractOnce        sync.Once
)

type QualityPreset struct {
	ID          int
	Name        string
	Description string
	ShaderFiles []string
}

var QualityPresets = []QualityPreset{
	{
		ID:          1,
		Name:        "FAST / LOW",
		Description: "Leggero e veloce, ideale per GPU integrate o laptop a batteria",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Restore_CNN_Soft_S.glsl",
			"Anime4K_Upscale_CNN_x2_S.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_S.glsl",
		},
	},
	{
		ID:          2,
		Name:        "BALANCED / MID",
		Description: "Ottimo compromesso tra prestazioni e pulizia video",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Restore_CNN_M.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
		},
	},
	{
		ID:          3,
		Name:        "HIGH",
		Description: "Qualità elevata con modelli CNN Large",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Restore_CNN_L.glsl",
			"Anime4K_Upscale_CNN_x2_L.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
		},
	},
	{
		ID:          4,
		Name:        "ULTRA (VL)",
		Description: "Modelli Very Large, massima nitidezza (richiede GPU dedicata)",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Restore_CNN_VL.glsl",
			"Anime4K_Upscale_CNN_x2_VL.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
		},
	},
	{
		ID:          5,
		Name:        "ULTRA+ (UL - Maximum Quality)",
		Description: "Modelli Ultra Large per il massimo dettaglio assoluto",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Restore_CNN_UL.glsl",
			"Anime4K_Upscale_CNN_x2_UL.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_L.glsl",
		},
	},
	{
		ID:          6,
		Name:        "DENOISE + UPSCALE (MID)",
		Description: "Rimuove rumore/grana e artefatti di compressione prima dell'upscale",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Denoise_Bilateral_Median.glsl",
			"Anime4K_Restore_CNN_M.glsl",
			"Anime4K_Upscale_Denoise_CNN_x2_M.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
		},
	},
	{
		ID:          7,
		Name:        "DENOISE + UPSCALE (HIGH)",
		Description: "Denoise avanzato con modelli CNN Large, perfetto per anime vecchi o a bassa risoluzione",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Denoise_Bilateral_Mode.glsl",
			"Anime4K_Restore_CNN_L.glsl",
			"Anime4K_Upscale_Denoise_CNN_x2_VL.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
		},
	},
	{
		ID:          8,
		Name:        "LINE ENHANCE (Darken & Thin Lines)",
		Description: "Line-art più marcata, sottile e contrastata (linee nere definite stile moderno)",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Restore_CNN_M.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
			"Anime4K_Thin_HQ.glsl",
			"Anime4K_Darken_HQ.glsl",
		},
	},
	{
		ID:          9,
		Name:        "DEBLUR + CRISP",
		Description: "Recupero sfocatura e linee ultra-nitide con filtri DoG",
		ShaderFiles: []string{
			"Anime4K_Clamp_Highlights.glsl",
			"Anime4K_Deblur_DoG.glsl",
			"Anime4K_Restore_CNN_M.glsl",
			"Anime4K_Upscale_Deblur_DoG_x2.glsl",
			"Anime4K_AutoDownscalePre_x2.glsl",
			"Anime4K_AutoDownscalePre_x4.glsl",
			"Anime4K_Upscale_CNN_x2_M.glsl",
		},
	},
}

func GetPresetByID(id int) *QualityPreset {
	for i := range QualityPresets {
		if QualityPresets[i].ID == id {
			return &QualityPresets[i]
		}
	}
	return nil
}

func ensureShaders() (string, error) {
	var err error
	extractOnce.Do(func() {
		userCache, cacheErr := os.UserCacheDir()
		if cacheErr != nil {
			userCache = os.TempDir()
		}
		targetDir := filepath.Join(userCache, "upscalercli", "shaders")

		if mkErr := os.MkdirAll(targetDir, 0755); mkErr != nil {
			err = fmt.Errorf("impossibile creare la cartella shader (%s): %w", targetDir, mkErr)
			return
		}

		entries, readErr := fs.ReadDir(embeddedShaders, "shaders")
		if readErr != nil {
			err = fmt.Errorf("impossibile leggere gli shader incorporati: %w", readErr)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			destPath := filepath.Join(targetDir, entry.Name())

			// Se il file esiste già ed ha dimensione > 0, saltiamo l'estrazione
			if info, statErr := os.Stat(destPath); statErr == nil && info.Size() > 0 {
				continue
			}

			data, readErr := embeddedShaders.ReadFile("shaders/" + entry.Name())
			if readErr != nil {
				err = fmt.Errorf("lettura shader %s fallita: %w", entry.Name(), readErr)
				return
			}

			if writeErr := os.WriteFile(destPath, data, 0644); writeErr != nil {
				err = fmt.Errorf("scrittura shader %s fallita: %w", destPath, writeErr)
				return
			}
		}

		extractedShaderDir = targetDir
	})

	return extractedShaderDir, err
}

func getShaderPath(dir, shaderName string) string {
	return filepath.Join(dir, shaderName)
}
