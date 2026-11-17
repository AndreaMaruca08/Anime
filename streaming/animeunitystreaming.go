package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Regex patterns for URL matching, Inertia data extraction and VixCloud player extraction.
var (
	animeURLPattern           = regexp.MustCompile(`(?:/anime/)?(\d+)(?:-([^/?#\s]+))?`)
	dataPagePattern           = regexp.MustCompile(`data-page="([^"]+)"`)
	downloadLinkPattern       = regexp.MustCompile(`window\.downloadUrl\s*=\s*'([^']+)'`)
	streamURLPattern          = regexp.MustCompile(`https?://[^\s"'<>]+\.(?:mp4|m3u8)[^\s"'<>]*`)
	playlistURLPattern        = regexp.MustCompile(`https?://[^\s"'<>]*/playlist/\d+[^\s"'<>]*`)
	masterPlaylistURLRegex    = regexp.MustCompile(`window\.masterPlaylist\s*=\s*\{[\s\S]*?url\s*:\s*['"]([^'"]+)['"]`)
	masterPlaylistParamsRegex = regexp.MustCompile(`window\.masterPlaylist\s*=\s*\{[\s\S]*?params\s*:\s*\{([^}]*)\}`)
	paramPairRegex            = regexp.MustCompile(`['"]?([a-zA-Z0-9_]+)['"]?\s*:\s*['"]([^'"]*)['"]`)
	canPlayFHDRegex           = regexp.MustCompile(`window\.canPlayFHD\s*=\s*(true|1)`)
	streamsURLRegex           = regexp.MustCompile(`window\.streams\s*=\s*\[[\s\S]*?['"]url['"]\s*:\s*['"]([^'"]+)['"]`)
)

const (
	defaultAnimeUnityHost    = "animeunity.cx"
	defaultBatchSize         = 120
	defaultInertiaAppVersion = "8aa74ded7645fe64ee2d1a852c51b7bf"
)

// UnityError represents an error occurring during AnimeUnity scraping or API calls.
type UnityError struct {
	Message    string
	StatusCode int
}

func (e *UnityError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("AnimeUnityError (%d): %s", e.StatusCode, e.Message)
	}
	return "AnimeUnityError: " + e.Message
}

// AnimeUnityClient implements the Provider interface for AnimeUnity.
type AnimeUnityClient struct {
	BaseURL        string
	HTTP           *http.Client
	UserAgent      string
	Concurrency    int
	inertiaVersion string
	versionMu      sync.RWMutex
}

var _ Provider = (*AnimeUnityClient)(nil)

// NewAnimeUnityClient creates and configures a new AnimeUnityClient.
func NewAnimeUnityClient() *AnimeUnityClient {
	return &AnimeUnityClient{
		BaseURL: "https://" + defaultAnimeUnityHost,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
		Concurrency:    8,
		inertiaVersion: defaultInertiaAppVersion,
	}
}

// Name returns the provider name.
func (c *AnimeUnityClient) Name() string {
	return "AnimeUnity"
}

func (c *AnimeUnityClient) getInertiaVersion() string {
	c.versionMu.RLock()
	defer c.versionMu.RUnlock()
	if c.inertiaVersion != "" {
		return c.inertiaVersion
	}
	return defaultInertiaAppVersion
}

func (c *AnimeUnityClient) setInertiaVersion(v string) {
	if strings.TrimSpace(v) == "" {
		return
	}
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	c.inertiaVersion = strings.TrimSpace(v)
}

// --- Internal HTTP helpers ---

func validateURL(rawURL string) string {
	return strings.TrimRight(rawURL, "/")
}

func extractHostDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return defaultAnimeUnityHost
	}
	host := strings.TrimPrefix(parsed.Host, "www.")
	if host == "animeunity.to" || host == "animeunity.co" {
		return defaultAnimeUnityHost
	}
	return host
}

func (c *AnimeUnityClient) getHeaders(referer string) http.Header {
	h := http.Header{}
	h.Set("User-Agent", c.UserAgent)
	h.Set("Accept", "application/json, text/html, */*")
	h.Set("Accept-Language", "it-IT,it;q=0.9,en-US;q=0.8,en;q=0.7")
	h.Set("DNT", "1")
	h.Set("Connection", "keep-alive")
	if referer != "" {
		h.Set("Referer", referer)
	}
	return h
}

func (c *AnimeUnityClient) fetchWithRetries(ctx context.Context, targetURL string, referer string, retries int) (string, int, error) {
	if retries < 1 {
		retries = 3
	}

	var lastErr error
	var lastStatus int

	for attempt := 0; attempt < retries; attempt++ {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return "", 0, err
		}
		req.Header = c.getHeaders(referer)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))*400+float64(rand.Intn(300))) * time.Millisecond)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastStatus = resp.StatusCode

		if resp.StatusCode == http.StatusOK {
			return string(body), resp.StatusCode, nil
		}

		lastErr = &UnityError{
			Message:    fmt.Sprintf("request to %s failed with status %d", targetURL, resp.StatusCode),
			StatusCode: resp.StatusCode,
		}

		// Don't retry standard client errors like 404 or 409
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusConflict {
			return string(body), resp.StatusCode, lastErr
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))*800+float64(rand.Intn(500))) * time.Millisecond)
			continue
		}

		return string(body), resp.StatusCode, lastErr
	}

	return "", lastStatus, lastErr
}

// --- API Data Models ---

type searchItemImages struct {
	Banner *string `json:"banner"`
	Cover  *string `json:"cover"`
	Poster *string `json:"poster"`
	Logo   *string `json:"logo"`
}

type searchItemVariant struct {
	Language      string `json:"language"`
	Status        string `json:"status"`
	EpisodesCount int    `json:"episodes_count"`
}

type searchItem struct {
	ID            int                 `json:"id"`
	Slug          string              `json:"slug"`
	Title         string              `json:"title"`
	TitleJa       *string             `json:"title_ja"`
	TitleIt       *string             `json:"title_it"`
	Type          string              `json:"type"`
	Season        string              `json:"season"`
	Year          int                 `json:"year"`
	TotalEpisodes int                 `json:"total_episodes"`
	Score         string              `json:"score"`
	Images        searchItemImages    `json:"images"`
	Variants      []searchItemVariant `json:"variants"`
}

type searchResponse struct {
	Data    []searchItem `json:"data"`
	Count   int          `json:"count"`
	HasMore bool         `json:"hasMore"`
}

// Search queries the AnimeUnity /api/search endpoint.
func (c *AnimeUnityClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	host := extractHostDomain(c.BaseURL)
	searchURL := fmt.Sprintf("https://%s/api/search?q=%s", host, url.QueryEscape(query))

	body, _, err := c.fetchWithRetries(ctx, searchURL, fmt.Sprintf("https://%s/", host), 3)
	if err != nil {
		return nil, err
	}

	var resp searchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("impossibile interpretare la risposta di ricerca: %w", err)
	}

	var results []SearchResult
	seen := make(map[string]bool)

	for _, item := range resp.Data {
		name := strings.TrimSpace(item.Title)
		if name == "" && item.TitleIt != nil && strings.TrimSpace(*item.TitleIt) != "" {
			name = strings.TrimSpace(*item.TitleIt)
		}
		if name == "" && item.TitleJa != nil && strings.TrimSpace(*item.TitleJa) != "" {
			name = strings.TrimSpace(*item.TitleJa)
		}
		if name == "" {
			name = strings.Title(strings.ReplaceAll(item.Slug, "-", " "))
		}

		cover := ""
		if item.Images.Poster != nil && *item.Images.Poster != "" {
			cover = *item.Images.Poster
		} else if item.Images.Cover != nil && *item.Images.Cover != "" {
			cover = *item.Images.Cover
		} else if item.Images.Banner != nil && *item.Images.Banner != "" {
			cover = *item.Images.Banner
		}

		animeURL := fmt.Sprintf("https://%s/anime/%d-%s", host, item.ID, item.Slug)
		if seen[animeURL] {
			continue
		}
		seen[animeURL] = true

		results = append(results, SearchResult{
			Name:  name,
			URL:   animeURL,
			Cover: cover,
		})
	}

	return results, nil
}

// --- Anime Details Models ---

type animeInertiaProps struct {
	Anime *struct {
		ID            int              `json:"id"`
		Slug          string           `json:"slug"`
		Title         string           `json:"title"`
		TitleJa       *string          `json:"title_ja"`
		TitleIt       *string          `json:"title_it"`
		Type          string           `json:"type"`
		Season        string           `json:"season"`
		Year          int              `json:"year"`
		TotalEpisodes int              `json:"total_episodes"`
		Score         string           `json:"score"`
		Plot          string           `json:"plot"`
		Images        searchItemImages `json:"images"`
		Variants      []any            `json:"variants"`
		Genres        []any            `json:"genres"`
		Studios       []any            `json:"studios"`
		MalID         any              `json:"mal_id"`
		AnilistID     any              `json:"anilist_id"`
	} `json:"anime"`
	LoadedEpisodes *struct {
		Total   int `json:"total"`
		TabSize int `json:"tab_size"`
	} `json:"loadedEpisodes"`
	EmbedURL string `json:"embedUrl"`
}

type inertiaPageData struct {
	Component string            `json:"component"`
	Version   string            `json:"version"`
	Props     animeInertiaProps `json:"props"`
}

// parseInertiaDataPage extracts and parses the JSON from data-page="..." attribute.
func parseInertiaDataPage(content string) (*inertiaPageData, error) {
	m := dataPagePattern.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil, fmt.Errorf("attributo data-page non trovato")
	}
	unescaped := html.UnescapeString(m[1])
	var page inertiaPageData
	if err := json.Unmarshal([]byte(unescaped), &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetAnime extracts metadata for a given anime from the Inertia page.
func (c *AnimeUnityClient) GetAnime(ctx context.Context, pageURL string) (*Anime, error) {
	pageURL = validateURL(pageURL)
	host := extractHostDomain(pageURL)

	match := animeURLPattern.FindStringSubmatch(pageURL)
	var animeID string
	var slug string
	if len(match) > 1 {
		animeID = match[1]
	}
	if len(match) > 2 {
		slug = match[2]
	}

	targetURL := pageURL
	if !strings.HasPrefix(targetURL, "http") {
		if slug != "" {
			targetURL = fmt.Sprintf("https://%s/anime/%s-%s", host, animeID, slug)
		} else {
			targetURL = fmt.Sprintf("https://%s/anime/%s-_", host, animeID)
		}
	}

	body, _, err := c.fetchWithRetries(ctx, targetURL, fmt.Sprintf("https://%s/", host), 3)
	if err != nil {
		return nil, err
	}

	pageData, err := parseInertiaDataPage(body)
	if err != nil {
		return nil, fmt.Errorf("errore lettura dettagli anime: %w", err)
	}

	if pageData.Version != "" {
		c.setInertiaVersion(pageData.Version)
	}

	animeData := pageData.Props.Anime
	if animeData == nil {
		return nil, fmt.Errorf("dati anime non presenti nella pagina")
	}

	title := strings.TrimSpace(animeData.Title)
	if title == "" && animeData.TitleIt != nil && strings.TrimSpace(*animeData.TitleIt) != "" {
		title = strings.TrimSpace(*animeData.TitleIt)
	}
	if title == "" && animeData.TitleJa != nil && strings.TrimSpace(*animeData.TitleJa) != "" {
		title = strings.TrimSpace(*animeData.TitleJa)
	}
	if title == "" {
		title = strings.Title(strings.ReplaceAll(animeData.Slug, "-", " "))
	}

	cover := ""
	if animeData.Images.Poster != nil && *animeData.Images.Poster != "" {
		cover = *animeData.Images.Poster
	} else if animeData.Images.Cover != nil && *animeData.Images.Cover != "" {
		cover = *animeData.Images.Cover
	} else if animeData.Images.Banner != nil && *animeData.Images.Banner != "" {
		cover = *animeData.Images.Banner
	}

	canonicalURL := fmt.Sprintf("https://%s/anime/%d-%s", host, animeData.ID, animeData.Slug)

	infoMap := map[string]any{
		"id":             animeData.ID,
		"ID":             animeData.ID,
		"slug":           animeData.Slug,
		"type":           animeData.Type,
		"season":         animeData.Season,
		"year":           animeData.Year,
		"total_episodes": animeData.TotalEpisodes,
		"episodes_count": animeData.TotalEpisodes,
		"score":          animeData.Score,
		"variants":       animeData.Variants,
		"genres":         animeData.Genres,
		"studios":        animeData.Studios,
		"mal_id":         animeData.MalID,
		"anilist_id":     animeData.AnilistID,
	}

	defaultLang := "ita"
	hasITA := false
	for _, v := range animeData.Variants {
		if vm, ok := v.(map[string]any); ok {
			if l, _ := vm["language"].(string); strings.EqualFold(l, "ita") {
				hasITA = true
				break
			}
		}
	}
	if !hasITA && len(animeData.Variants) > 0 {
		defaultLang = "jap"
	}
	infoMap["language"] = defaultLang

	return &Anime{
		Name:        title,
		URL:         canonicalURL,
		Cover:       cover,
		Description: strings.TrimSpace(animeData.Plot),
		Language:    defaultLang,
		Info:        infoMap,
	}, nil
}

// --- Episodes & Videos Models ---

type apiVideoInfo struct {
	ID            int    `json:"id"`
	Language      string `json:"language"`
	Runtime       int    `json:"runtime"`
	SCWSID        int    `json:"scws_id"`
	IsHidden      bool   `json:"is_hidden"`
	HasTimestamps bool   `json:"has_timestamps"`
}

type apiEpisodeItem struct {
	ID             int            `json:"id"`
	Number         string         `json:"number"`
	SeqNumberStart int            `json:"seq_number_start"`
	SeqNumberEnd   int            `json:"seq_number_end"`
	IsFiller       bool           `json:"is_filler"`
	Videos         []apiVideoInfo `json:"videos"`
}

type apiEpisodesResponse struct {
	Episodes []apiEpisodeItem `json:"episodes"`
	Start    int              `json:"start"`
	TabSize  int              `json:"tab_size"`
	Total    int              `json:"total"`
	Offset   int              `json:"offset"`
}

type unityEpisodeEntry struct {
	ep         Episode
	primaryVid int
}

// GetEpisodes fetches episodes for an anime and resolves their playable stream URLs.
func (c *AnimeUnityClient) GetEpisodes(ctx context.Context, anime *Anime, numbers ...string) ([]Episode, error) {
	animeURL := validateURL(anime.URL)
	host := extractHostDomain(animeURL)

	var animeID int
	var slug string

	if anime.Info != nil {
		if idVal, ok := anime.Info["id"].(int); ok && idVal > 0 {
			animeID = idVal
		} else if idVal, ok := anime.Info["ID"].(int); ok && idVal > 0 {
			animeID = idVal
		}
		if sVal, ok := anime.Info["slug"].(string); ok && sVal != "" {
			slug = sVal
		}
	}

	if animeID == 0 {
		m := animeURLPattern.FindStringSubmatch(animeURL)
		if len(m) > 1 {
			animeID, _ = strconv.Atoi(m[1])
		}
		if len(m) > 2 && slug == "" {
			slug = m[2]
		}
	}

	if animeID == 0 {
		return nil, &UnityError{Message: "Formato URL anime non valido: " + animeURL}
	}
	if slug == "" {
		slug = "_"
	}

	allowed := make(map[string]bool)
	for _, n := range numbers {
		allowed[strings.TrimSpace(n)] = true
	}

	var allEpItems []apiEpisodeItem

	// Fast path: if only a single episode is requested, calculate its batch start
	if len(numbers) == 1 {
		if reqNum, err := strconv.Atoi(numbers[0]); err == nil && reqNum >= 1 {
			batchStart := ((reqNum - 1) / defaultBatchSize) * defaultBatchSize
			if batchStart < 1 {
				batchStart = 1
			}
			batchURL := fmt.Sprintf("https://%s/api/anime/%d/episodes?start=%d", host, animeID, batchStart)
			body, _, err := c.fetchWithRetries(ctx, batchURL, animeURL, 3)
			if err == nil {
				var bResp apiEpisodesResponse
				if err := json.Unmarshal([]byte(body), &bResp); err == nil {
					for _, ep := range bResp.Episodes {
						if ep.Number == numbers[0] {
							allEpItems = append(allEpItems, ep)
							break
						}
					}
				}
			}
		}
	}

	// If not found via fast path or multiple/all episodes requested, fetch all batches
	if len(allEpItems) == 0 {
		firstBatchURL := fmt.Sprintf("https://%s/api/anime/%d/episodes?start=1", host, animeID)
		body, _, err := c.fetchWithRetries(ctx, firstBatchURL, animeURL, 3)
		if err != nil {
			return nil, err
		}

		var firstResp apiEpisodesResponse
		if err := json.Unmarshal([]byte(body), &firstResp); err != nil {
			return nil, fmt.Errorf("errore lettura lista episodi: %w", err)
		}

		allEpItems = append(allEpItems, firstResp.Episodes...)

		tabSize := firstResp.TabSize
		if tabSize <= 0 {
			tabSize = defaultBatchSize
		}
		total := firstResp.Total

		if total > tabSize {
			var starts []int
			for s := 1 + tabSize; s <= total; s += tabSize {
				starts = append(starts, s)
			}

			conc := c.Concurrency
			if conc < 1 {
				conc = 4
			}
			sem := make(chan struct{}, conc)
			var wg sync.WaitGroup
			batchResults := make([][]apiEpisodeItem, len(starts))

			for idx, sVal := range starts {
				wg.Add(1)
				sem <- struct{}{}
				go func(i int, startOffset int) {
					defer wg.Done()
					defer func() { <-sem }()

					target := fmt.Sprintf("https://%s/api/anime/%d/episodes?start=%d", host, animeID, startOffset)
					bBody, _, bErr := c.fetchWithRetries(ctx, target, animeURL, 3)
					if bErr != nil {
						return
					}
					var bResp apiEpisodesResponse
					if err := json.Unmarshal([]byte(bBody), &bResp); err == nil && len(bResp.Episodes) > 0 {
						batchResults[i] = bResp.Episodes
					}
				}(idx, sVal)
			}
			wg.Wait()

			for _, bList := range batchResults {
				allEpItems = append(allEpItems, bList...)
			}
		}
	}

	// Deduplicate and filter by requested numbers
	seenEps := make(map[string]bool)
	var epList []unityEpisodeEntry

	for _, item := range allEpItems {
		epNumStr := item.Number
		if len(allowed) > 0 && !allowed[epNumStr] {
			continue
		}
		if seenEps[epNumStr] {
			continue
		}
		seenEps[epNumStr] = true

		epIDStr := strconv.Itoa(item.ID)
		var links []EpisodeLink
		var chosenVideoID int

		targetLang := "ita"
		pref := strings.ToLower(strings.TrimSpace(anime.Language))
		if pref == "" && anime.Info != nil {
			if l, ok := anime.Info["language"].(string); ok {
				pref = strings.ToLower(strings.TrimSpace(l))
			}
		}
		if pref == "jap" || pref == "jpn" || pref == "sub" {
			targetLang = "jpn"
		} else if pref == "ita" || pref == "dub" {
			targetLang = "ita"
		} else if pref != "" {
			targetLang = pref
		}

		// First pass: find video matching target language
		for _, vid := range item.Videos {
			if strings.EqualFold(vid.Language, targetLang) {
				chosenVideoID = vid.ID
				break
			}
		}
		// Fallback pass: if target language not found, use first available video
		if chosenVideoID == 0 && len(item.Videos) > 0 {
			chosenVideoID = item.Videos[0].ID
		}

		for _, vid := range item.Videos {
			langLabel := strings.ToUpper(vid.Language)
			if langLabel == "JPN" {
				langLabel = "JAP"
			}
			serverName := "VixCloud"
			if langLabel != "" {
				serverName = fmt.Sprintf("VixCloud (%s)", langLabel)
			}
			videoURL := fmt.Sprintf("https://%s/anime/%d-%s/%d", host, animeID, slug, vid.ID)
			links = append(links, EpisodeLink{
				ID:         strconv.Itoa(vid.ID),
				URL:        videoURL,
				ServerName: serverName,
			})
		}

		// Place preferred language link first in Links slice
		if len(links) > 1 && chosenVideoID != 0 {
			chosenIDStr := strconv.Itoa(chosenVideoID)
			sort.SliceStable(links, func(a, b int) bool {
				return links[a].ID == chosenIDStr
			})
		}

		epList = append(epList, unityEpisodeEntry{
			ep: Episode{
				Number: epNumStr,
				ID:     epIDStr,
				Links:  links,
			},
			primaryVid: chosenVideoID,
		})
	}

	if len(epList) == 0 {
		return nil, &UnityError{Message: "Nessun episodio trovato per " + anime.Name}
	}

	// Sort episodes numerically when possible
	sort.SliceStable(epList, func(i, j int) bool {
		ni, oki := safeFloat(epList[i].ep.Number)
		nj, okj := safeFloat(epList[j].ep.Number)
		if oki && okj {
			return ni < nj
		}
		return epList[i].ep.Number < epList[j].ep.Number
	})

	// Concurrently resolve stream URLs
	episodes := make([]Episode, len(epList))
	for i := range epList {
		episodes[i] = epList[i].ep
	}

	c.resolveStreams(ctx, host, animeID, slug, epList, episodes)

	return episodes, nil
}

// resolveStreams concurrently resolves each episode's video into a playable stream URL.
func (c *AnimeUnityClient) resolveStreams(ctx context.Context, host string, animeID int, slug string, epItems []unityEpisodeEntry, episodes []Episode) {
	conc := c.Concurrency
	if conc < 1 {
		conc = 4
	}
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	for i := range episodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			ep := &episodes[idx]
			vidID := epItems[idx].primaryVid
			if vidID == 0 {
				ep.Err = &UnityError{Message: "Nessun video disponibile per l'episodio " + ep.Number}
				return
			}

			streamURL, err := c.resolveVideoPlayableStream(ctx, host, animeID, slug, vidID)
			if err != nil {
				ep.Err = err
				return
			}

			ep.StreamURL = streamURL
			for j := range ep.Links {
				if ep.Links[j].ID == strconv.Itoa(vidID) {
					ep.Links[j].StreamURL = streamURL
					break
				}
			}
		}(i)
	}
	wg.Wait()
}

// resolveVideoPlayableStream fetches the VixCloud embed URL for a video and resolves it into a stream URL.
func (c *AnimeUnityClient) resolveVideoPlayableStream(ctx context.Context, host string, animeID int, slug string, videoID int) (string, error) {
	embedURL, err := c.fetchEmbedURL(ctx, host, animeID, slug, videoID)
	if err != nil {
		return "", err
	}
	if embedURL == "" {
		return "", &UnityError{Message: fmt.Sprintf("Nessun embedUrl trovato per il video %d", videoID)}
	}

	return c.extractPlayableStreamFromVix(ctx, host, embedURL)
}

// fetchEmbedURL retrieves the VixCloud embed URL using Inertia request or HTML fallback.
func (c *AnimeUnityClient) fetchEmbedURL(ctx context.Context, host string, animeID int, slug string, videoID int) (string, error) {
	videoURL := fmt.Sprintf("https://%s/anime/%d-%s/%d", host, animeID, slug, videoID)

	// Step 1: Attempt fast Inertia request
	inertiaVer := c.getInertiaVersion()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, videoURL, nil)
	if err == nil {
		req.Header = c.getHeaders(fmt.Sprintf("https://%s/", host))
		req.Header.Set("X-Inertia", "true")
		req.Header.Set("X-Inertia-Version", inertiaVer)
		req.Header.Set("X-Inertia-Partial-Component", "Anime")
		req.Header.Set("X-Inertia-Partial-Data", "embedUrl")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")

		resp, err := c.HTTP.Do(req)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				var res struct {
					Props struct {
						EmbedURL string `json:"embedUrl"`
					} `json:"props"`
					Version string `json:"version"`
				}
				if jsonErr := json.Unmarshal(body, &res); jsonErr == nil && res.Props.EmbedURL != "" {
					if res.Version != "" {
						c.setInertiaVersion(res.Version)
					}
					return res.Props.EmbedURL, nil
				}
			}
		}
	}

	// Step 2: Fallback to full page HTML request (also updates Inertia version if changed)
	htmlBody, _, fetchErr := c.fetchWithRetries(ctx, videoURL, fmt.Sprintf("https://%s/", host), 3)
	if fetchErr != nil {
		return "", fetchErr
	}

	pageData, pErr := parseInertiaDataPage(htmlBody)
	if pErr == nil && pageData != nil {
		if pageData.Version != "" {
			c.setInertiaVersion(pageData.Version)
		}
		if pageData.Props.EmbedURL != "" {
			return pageData.Props.EmbedURL, nil
		}
	}

	return "", fmt.Errorf("impossibile ricavare embedUrl per il video %d", videoID)
}

// extractPlayableStreamFromVix fetches the VixCloud embed page and extracts the video stream URL.
func (c *AnimeUnityClient) extractPlayableStreamFromVix(ctx context.Context, host string, embedURL string) (string, error) {
	// If the embedURL is already a direct video URL, return it
	if streamURLPattern.MatchString(embedURL) {
		return embedURL, nil
	}

	referer := fmt.Sprintf("https://%s/", host)
	vixHTML, _, err := c.fetchWithRetries(ctx, embedURL, referer, 3)
	if err != nil {
		return "", err
	}

	// 1. Direct MP4 download link (fastest and highest quality for mpv)
	if dlLink := extractDownloadLink(vixHTML); dlLink != "" {
		return dlLink, nil
	}

	// 2. Signed HLS master playlist URL
	if streamURL := extractStreamFromVixHTML(vixHTML, embedURL); streamURL != "" {
		return streamURL, nil
	}

	// 3. Fallback: regex search for mp4 or m3u8 in page
	if m := streamURLPattern.FindString(vixHTML); m != "" {
		return strings.ReplaceAll(m, `\/`, `/`), nil
	}

	// 4. Fallback: playlist url
	if p := playlistURLPattern.FindString(vixHTML); p != "" {
		return strings.ReplaceAll(p, `\/`, `/`), nil
	}

	// 5. Fallback to embed player URL itself
	if strings.HasPrefix(embedURL, "http") {
		return embedURL, nil
	}

	return "", &UnityError{Message: "Impossibile trovare uno stream valido per " + embedURL}
}

func extractDownloadLink(content string) string {
	match := downloadLinkPattern.FindStringSubmatch(content)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// extractStreamFromVixHTML parses window.masterPlaylist and builds the signed playlist URL.
func extractStreamFromVixHTML(vixHTML, vixEmbedURL string) string {
	var embedU *url.URL
	if vixEmbedURL != "" {
		embedU, _ = url.Parse(vixEmbedURL)
	}

	if uMatch := masterPlaylistURLRegex.FindStringSubmatch(vixHTML); len(uMatch) > 1 {
		rawURL := strings.ReplaceAll(uMatch[1], `\/`, `/`)
		if parsedURL, err := url.Parse(rawURL); err == nil {
			q := parsedURL.Query()

			if pMatch := masterPlaylistParamsRegex.FindStringSubmatch(vixHTML); len(pMatch) > 1 {
				pairs := paramPairRegex.FindAllStringSubmatch(pMatch[1], -1)
				for _, pair := range pairs {
					if len(pair) >= 3 && pair[2] != "" {
						q.Set(pair[1], pair[2])
					}
				}
			}

			// Fallback: token/expires from embed URL if missing from masterPlaylist.params
			if embedU != nil {
				if q.Get("token") == "" && embedU.Query().Get("token") != "" {
					q.Set("token", embedU.Query().Get("token"))
				}
				if q.Get("expires") == "" && embedU.Query().Get("expires") != "" {
					q.Set("expires", embedU.Query().Get("expires"))
				}
			}

			// Check if Full HD (canPlayFHD) is available / requested
			isFHD := false
			if embedU != nil {
				canPlay := embedU.Query().Get("canPlayFHD")
				if canPlay != "" && canPlay != "0" && canPlay != "false" {
					isFHD = true
				}
			}
			if !isFHD && canPlayFHDRegex.MatchString(vixHTML) {
				isFHD = true
			}

			if isFHD {
				q.Set("h", "1")
			}

			parsedURL.RawQuery = q.Encode()
			return parsedURL.String()
		}
	}

	// Fallback to window.streams = [{"url": "https:\/\/vixcloud.cc\/playlist\/..."}]
	if sMatch := streamsURLRegex.FindStringSubmatch(vixHTML); len(sMatch) > 1 {
		raw := strings.ReplaceAll(sMatch[1], `\/`, `/`)
		if parsed, err := url.Parse(raw); err == nil {
			q := parsed.Query()
			if embedU != nil {
				if q.Get("token") == "" && embedU.Query().Get("token") != "" {
					q.Set("token", embedU.Query().Get("token"))
				}
				if q.Get("expires") == "" && embedU.Query().Get("expires") != "" {
					q.Set("expires", embedU.Query().Get("expires"))
				}
			}
			parsed.RawQuery = q.Encode()
			return parsed.String()
		}
		return raw
	}

	return ""
}

func safeFloat(val string) (float64, bool) {
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
