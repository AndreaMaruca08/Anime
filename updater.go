package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// DefaultRepo è il repository predefinito di riferimento per gli aggiornamenti
	DefaultRepo = "AndreaMaruca08/UpscalerCLI"
	// CurrentVersion è la versione attuale dell'applicazione
	CurrentVersion = "v1.0.0"
)

// ReleaseAsset rappresenta un file allegato a una release su GitHub
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// ReleaseInfo rappresenta le informazioni essenziali di una release GitHub
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	HTMLURL     string         `json:"html_url"`
	Body        string         `json:"body"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

// Updater gestisce il controllo e l'applicazione di aggiornamenti da una repository GitHub
type Updater struct {
	Repo           string // es: "AndreaMaruca08/UpscalerCLI"
	CurrentVersion string // es: "v1.0.0"
	BinaryName     string // es: "anime"
	HTTPClient     *http.Client
}

// NewUpdater crea una nuova istanza di Updater configurabile
func NewUpdater(repo, currentVersion, binaryName string) *Updater {
	return &Updater{
		Repo:           repo,
		CurrentVersion: currentVersion,
		BinaryName:     binaryName,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// CleanVersion rimuove prefissi come 'v' o spazi da una stringa di versione
func CleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// IsNewerVersion confronta due versioni in formato semver (es. 1.0.1 vs 1.0.0)
func IsNewerVersion(current, latest string) bool {
	cClean := CleanVersion(current)
	lClean := CleanVersion(latest)
	if cClean == lClean {
		return false
	}

	cParts := strings.Split(cClean, ".")
	lParts := strings.Split(lClean, ".")
	maxLen := max(len(cParts), len(lParts))

	for i := 0; i < maxLen; i++ {
		var cVal, lVal int
		if i < len(cParts) {
			fmt.Sscanf(cParts[i], "%d", &cVal)
		}
		if i < len(lParts) {
			fmt.Sscanf(lParts[i], "%d", &lVal)
		}
		if lVal > cVal {
			return true
		}
		if lVal < cVal {
			return false
		}
	}
	return false
}

// CheckUpdate interroga l'API di GitHub per verificare se è disponibile una versione più recente
func (u *Updater) CheckUpdate(ctx context.Context) (*ReleaseInfo, bool, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("creazione richiesta fallita: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Go-Updater")

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("errore connessione a GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, fmt.Errorf("nessuna release trovata per la repo %s", u.Repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("risposta inattesa da GitHub (status: %d)", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("decodifica release JSON fallita: %w", err)
	}

	hasUpdate := IsNewerVersion(u.CurrentVersion, release.TagName)
	return &release, hasUpdate, nil
}

// FindMatchingAsset cerca l'asset corrispondente al sistema operativo e architettura correnti
func (u *Updater) FindMatchingAsset(assets []ReleaseAsset) (*ReleaseAsset, error) {
	targetOS := runtime.GOOS     // darwin, linux, windows, freebsd
	targetArch := runtime.GOARCH // amd64, arm64

	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)
		if strings.Contains(lower, targetOS) && strings.Contains(lower, targetArch) {
			if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip") {
				return &asset, nil
			}
		}
	}
	return nil, fmt.Errorf("nessun pacchetto compatibile trovato per %s/%s tra le release", targetOS, targetArch)
}

// DownloadAndExtract scarica l'archivio ed estrae il file binario
func (u *Updater) DownloadAndExtract(ctx context.Context, downloadURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Go-Updater")

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("errore durante il download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download fallito (status code %d)", resp.StatusCode)
	}

	archiveBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("errore lettura payload archivio: %w", err)
	}

	expectedBinary := u.BinaryName
	if runtime.GOOS == "windows" && !strings.HasSuffix(expectedBinary, ".exe") {
		expectedBinary += ".exe"
	}

	if strings.HasSuffix(downloadURL, ".zip") {
		return extractFromZip(archiveBytes, expectedBinary)
	}
	return extractFromTarGz(archiveBytes, expectedBinary)
}

func extractFromZip(data []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("errore apertura zip: %w", err)
	}

	for _, f := range zr.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binario %s non trovato nello zip", binaryName)
}

func extractFromTarGz(data []byte, binaryName string) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("errore decompressione gzip: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("errore lettura tar: %w", err)
		}
		if filepath.Base(header.Name) == binaryName && (header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA) {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binario %s non trovato nel file tar.gz", binaryName)
}

// ApplyUpdate sostituisce l'eseguibile corrente con i dati del nuovo binario
func (u *Updater) ApplyUpdate(binaryData []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("impossibile determinare il percorso dell'eseguibile: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("risoluzione symlink eseguibile fallita: %w", err)
	}

	dir := filepath.Dir(execPath)
	tempFile, err := os.CreateTemp(dir, "anime-update-*")
	if err != nil {
		return fmt.Errorf("impossibile creare file temporaneo in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(binaryData); err != nil {
		tempFile.Close()
		return fmt.Errorf("errore scrittura nuovo binario: %w", err)
	}
	if err := tempFile.Chmod(0o755); err != nil {
		tempFile.Close()
		return fmt.Errorf("errore chmod sul nuovo binario: %w", err)
	}
	tempFile.Close()

	// Su Windows un file in esecuzione non può essere sovrascritto, ma può essere rinominato
	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("errore spostamento vecchio eseguibile: %w", err)
		}
		if err := os.Rename(tempPath, execPath); err != nil {
			_ = os.Rename(oldPath, execPath) // rollback
			return fmt.Errorf("errore installazione nuovo binario: %w", err)
		}
		return nil
	}

	// Su Unix (Linux, macOS, FreeBSD), atomic rename sovrascrive un file in esecuzione
	if err := os.Rename(tempPath, execPath); err != nil {
		return fmt.Errorf("errore sostituzione binario: %w", err)
	}

	return nil
}

// UpdateTo esegue il download dell'asset compatibile e aggiorna l'applicazione
func (u *Updater) UpdateTo(ctx context.Context, release *ReleaseInfo) error {
	asset, err := u.FindMatchingAsset(release.Assets)
	if err != nil {
		return err
	}

	binaryData, err := u.DownloadAndExtract(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return err
	}

	return u.ApplyUpdate(binaryData)
}
