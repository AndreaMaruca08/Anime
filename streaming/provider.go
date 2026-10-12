package streaming

import "context"

// Provider defines the generic methods for an anime streaming service.
type Provider interface {
	// Name returns the identifier of the streaming service (e.g. "AnimeWorld", "AnimeUnity").
	Name() string
	// Search searches for anime matching the query keyword.
	Search(ctx context.Context, query string) ([]SearchResult, error)
	// GetAnime retrieves anime details from a page URL.
	GetAnime(ctx context.Context, pageURL string) (*Anime, error)
	// GetEpisodes retrieves the episodes for an anime, optionally filtered by episode numbers.
	GetEpisodes(ctx context.Context, anime *Anime, numbers ...string) ([]Episode, error)
}

// SearchResult represents a single result from an anime search.
type SearchResult struct {
	Name  string
	URL   string
	Cover string
}

// Anime represents the details and metadata of an anime.
type Anime struct {
	Name        string
	URL         string
	Cover       string
	Description string
	Info        map[string]any
}

// EpisodeLink represents a link to a video server for an episode.
type EpisodeLink struct {
	ID         string
	URL        string
	ServerID   int
	ServerName string
	StreamURL  string
}

// Episode represents an episode of an anime with its links and stream URL.
type Episode struct {
	Number    string
	ID        string
	Links     []EpisodeLink
	StreamURL string
	Err       error
}
