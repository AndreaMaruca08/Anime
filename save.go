package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configAppName  = "Anime"
	configFileName = "settings.json"
)

type Config struct {
	Preference string `json:"server_preference"`
}

func DefaultConfig() *Config {
	return &Config{
		Preference: "default",
	}
}

// GetConfigDir
// - macOS: ~/Library/Application Support/Anime
// - Windows: %APPDATA%\Anime
// - Linux: ~/.config/Anime (o $XDG_CONFIG_HOME/Anime)
func GetConfigDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {

		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("impossibile determinare la cartella di configurazione utente: %w", err)
		}
		baseDir = filepath.Join(home, ".config")
	}

	appDir := filepath.Join(baseDir, configAppName)
	return appDir, nil
}

func GetConfigFilePath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func LoadConfig() (*Config, error) {
	filePath, err := GetConfigFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			if saveErr := SaveConfig(cfg); saveErr != nil {
				return cfg, fmt.Errorf("file di config non trovato e salvataggio default fallito: %w", saveErr)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("errore durante la lettura di %s: %w", filePath, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("errore nella decodifica JSON di %s: %w", filePath, err)
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config non può essere nil")
	}

	filePath, err := GetConfigFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("impossibile creare la cartella di configurazione %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("errore durante la serializzazione JSON della configurazione: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("impossibile scrivere il file di configurazione %s: %w", filePath, err)
	}

	return nil
}
