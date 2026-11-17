package streaming

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAnimeUnityClient_LiveIntegration(t *testing.T) {
	client := NewAnimeUnityClient()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// 1. Test Search
	results, err := client.Search(ctx, "Naruto")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("Search returned 0 results")
	}

	first := results[0]
	t.Logf("Search result #1: %s (URL: %s, Cover: %s)", first.Name, first.URL, first.Cover)
	if !strings.Contains(first.URL, "/anime/") {
		t.Errorf("Expected URL to contain '/anime/', got %s", first.URL)
	}

	// 2. Test GetAnime
	anime, err := client.GetAnime(ctx, first.URL)
	if err != nil {
		t.Fatalf("GetAnime failed: %v", err)
	}
	t.Logf("Anime details: Name=%s, EpisodesCount=%v, DescLen=%d", anime.Name, anime.Info["episodes_count"], len(anime.Description))
	if anime.Name == "" {
		t.Errorf("Expected anime name to be non-empty")
	}

	// 3. Test GetEpisodes (single episode)
	episodes, err := client.GetEpisodes(ctx, anime, "1")
	if err != nil {
		t.Fatalf("GetEpisodes failed: %v", err)
	}
	if len(episodes) == 0 {
		t.Fatalf("GetEpisodes returned 0 episodes")
	}

	ep1 := episodes[0]
	t.Logf("Episode 1: Number=%s, StreamURL=%s, Links=%d, Err=%v", ep1.Number, ep1.StreamURL, len(ep1.Links), ep1.Err)
	if ep1.StreamURL == "" {
		t.Errorf("Expected episode 1 to have a non-empty StreamURL")
	}
	if !strings.HasPrefix(ep1.StreamURL, "http") {
		t.Errorf("Expected StreamURL to start with http, got: %s", ep1.StreamURL)
	}
}

func TestAnimeUnityClient_AllEpisodes(t *testing.T) {
	client := NewAnimeUnityClient()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	anime, err := client.GetAnime(ctx, "https://animeunity.cx/anime/5660-dan-da-dan")
	if err != nil {
		t.Fatalf("GetAnime failed: %v", err)
	}

	episodes, err := client.GetEpisodes(ctx, anime)
	if err != nil {
		t.Fatalf("GetEpisodes failed: %v", err)
	}
	if len(episodes) == 0 {
		t.Fatalf("Expected episodes to be found")
	}
	t.Logf("Dan Da Dan retrieved %d episodes", len(episodes))
	validStreams := 0
	for _, ep := range episodes {
		if ep.StreamURL != "" {
			validStreams++
		}
	}
	t.Logf("Valid resolved streams: %d / %d", validStreams, len(episodes))
	if validStreams == 0 {
		t.Errorf("Expected at least one episode to have a resolved stream URL")
	}
}
