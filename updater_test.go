package main

import (
	"runtime"
	"testing"
)

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "1.0.0"},
		{"V2.1.3", "2.1.3"},
		{" 1.4.0 ", "1.4.0"},
		{"v0.0.1-alpha", "0.0.1-alpha"},
	}

	for _, tt := range tests {
		got := CleanVersion(tt.input)
		if got != tt.expected {
			t.Errorf("CleanVersion(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.2.0", "1.2.0", false},
		{"v1.3.0", "v1.2.0", false},
		{"2.0.0", "1.9.9", false},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.current, tt.latest)
		if got != tt.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.expected)
		}
	}
}

func TestFindMatchingAsset(t *testing.T) {
	updater := NewUpdater("AndreaMaruca08/UpscalerCLI", "v1.0.0", "anime")

	assets := []ReleaseAsset{
		{Name: "anime_1.0.1_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
		{Name: "anime_1.0.1_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64"},
		{Name: "anime_1.0.1_darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_amd64"},
		{Name: "anime_1.0.1_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	asset, err := updater.FindMatchingAsset(assets)
	if err != nil {
		t.Fatalf("FindMatchingAsset fallito per %s/%s: %v", runtime.GOOS, runtime.GOARCH, err)
	}

	if asset == nil {
		t.Fatal("asset trovato è nil")
	}
}
