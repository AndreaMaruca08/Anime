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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Regex patterns matching the Python crawler configuration.
var (
	downloadLinkPattern       = regexp.MustCompile(`window\.downloadUrl\s*=\s*'(https?://[^']+)'`)
	animeNamePattern          = regexp.MustCompile(`/anime/\d+-(.+)$`)
	animeURLPattern           = regexp.MustCompile(`(?:/anime/|/info_api/)(\d+-[^/?#]+)`)
	streamURLPattern          = regexp.MustCompile(`https?://[^\s"'<>]+\.(?:mp4|m3u8)[^\s"'<>]*`)
	playlistURLPattern        = regexp.MustCompile(`https?://[^\s"'<>]*/playlist/\d+[^\s"'<>]*`)
	archivioRecordsPattern    = regexp.MustCompile(`<archivio[^>]*records="([^"]+)"`)
	masterPlaylistURLRegex    = regexp.MustCompile(`window\.masterPlaylist\s*=\s*\{[\s\S]*?url\s*:\s*['"]([^'"]+)['"]`)
	masterPlaylistParamsRegex = regexp.MustCompile(`window\.masterPlaylist\s*=\s*\{[\s\S]*?params\s*:\s*\{([^}]*)\}`)
	paramPairRegex            = regexp.MustCompile(`['"]?([a-zA-Z0-9_]+)['"]?\s*:\s*['"]([^'"]*)['"]`)
	canPlayFHDRegex           = regexp.MustCompile(`window\.canPlayFHD\s*=\s*(true|1)`)
	streamsURLRegex           = regexp.MustCompile(`window\.streams\s*=\s*\[[\s\S]*?['"]url['"]\s*:\s*['"]([^'"]+)['"]`)
)

const (
	defaultAnimeUnityHost = "www.animeunity.to"
	defaultBatchSize      = 120 // Range span allowed by the API is < 120 (e.g. 119)
	defaultRangeSpan      = 119
)

// UnityError represents an error occurring during AnimeUnity scraping.
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
	BaseURL     string
	HTTP        *http.Client
	UserAgent   string
	Concurrency int
}

var _ Provider = (*AnimeUnityClient)(nil)

// NewAnimeUnityClient creates and configures a new AnimeUnityClient.
func NewAnimeUnityClient() *AnimeUnityClient {
	return &AnimeUnityClient{
		BaseURL: "https://" + defaultAnimeUnityHost,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
		UserAgent:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:132.0) Gecko/20100101 Firefox/132.0",
		Concurrency: 8,
	}
}

// Name returns the provider name.
func (c *AnimeUnityClient) Name() string {
	return "AnimeUnity"
}

// --- Helper Functions translated from Python crawler_utils ---

func validateURL(rawURL string) string {
	return strings.TrimRight(rawURL, "/")
}

func extractHostDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return defaultAnimeUnityHost
	}
	return parsed.Host
}

func extractNameFromTitleTag(titleText string) string {
	if strings.Contains(titleText, "AnimeUnity ~") {
		parts := strings.Split(titleText, "AnimeUnity ~")
		if len(parts) > 1 {
			sub := strings.Split(parts[1], "Streaming")
			return strings.TrimSpace(sub[0])
		}
	}
	res := strings.ReplaceAll(titleText, "AnimeUnity", "")
	res = strings.ReplaceAll(res, "~", "")
	return strings.TrimSpace(res)
}

func safeFloat(val string) (float64, bool) {
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func episodeInRange(num string, start, end *int) bool {
	n, ok := safeFloat(num)
	if !ok {
		return true // If cannot be compared as float, included by default
	}
	if start != nil && n < float64(*start) {
		return false
	}
	if end != nil && n > float64(*end) {
		return false
	}
	return true
}

func validateEpisodeRange(start, end *int, numEpisodes int) error {
	if start != nil && (*start < 1 || *start > numEpisodes) {
		return &UnityError{Message: fmt.Sprintf("Start episode must be between 1 and %d", numEpisodes)}
	}
	if start != nil && end != nil {
		if *start > *end {
			return &UnityError{Message: "Start episode cannot be greater than end episode"}
		}
		if *end > numEpisodes {
			return &UnityError{Message: fmt.Sprintf("End episode must be between 1 and %d", numEpisodes)}
		}
	}
	return nil
}

func extractDownloadLink(content string) string {
	match := downloadLinkPattern.FindStringSubmatch(content)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// --- Internal HTTP helpers with retry logic ---

func (c *AnimeUnityClient) getHeaders(referer string) http.Header {
	h := http.Header{}
	h.Set("User-Agent", c.UserAgent)
	h.Set("Accept", "application/json, text/plain, text/html, */*")
	h.Set("Accept-Language", "en-US,en;q=0.9,it;q=0.8")
	h.Set("DNT", "1")
	h.Set("Connection", "keep-alive")
	if referer != "" {
		h.Set("Referer", referer)
	}
	return h
}

func (c *AnimeUnityClient) fetchWithRetries(ctx context.Context, targetURL string, referer string, retries int) (string, error) {
	if retries < 1 {
		retries = 3
	}

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return "", err
		}
		req.Header = c.getHeaders(referer)

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))*500+float64(rand.Intn(500))) * time.Millisecond)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return string(body), nil
		}

		lastErr = &UnityError{
			Message:    fmt.Sprintf("request to %s failed with status %d", targetURL, resp.StatusCode),
			StatusCode: resp.StatusCode,
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 500 {
			time.Sleep(time.Duration(math.Pow(2, float64(attempt))*1000+float64(rand.Intn(1000))) * time.Millisecond)
			continue
		}

		// Don't retry client errors like 404
		return "", lastErr
	}

	return "", lastErr
}

// --- Provider Implementation ---

type archivioRecord struct {
	ID            int     `json:"id"`
	Title         *string `json:"title"`
	TitleEng      *string `json:"title_eng"`
	TitleIt       *string `json:"title_it"`
	Slug          string  `json:"slug"`
	ImageURL      string  `json:"imageurl"`
	Plot          string  `json:"plot"`
	Type          string  `json:"type"`
	Status        string  `json:"status"`
	Date          string  `json:"date"`
	Score         string  `json:"score"`
	EpisodesCount int     `json:"episodes_count"`
}

// Search queries the AnimeUnity archive by title.
func (c *AnimeUnityClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("%s/archivio?title=%s", validateURL(c.BaseURL), url.QueryEscape(query))
	body, err := c.fetchWithRetries(ctx, searchURL, c.BaseURL, 3)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	host := extractHostDomain(c.BaseURL)
	var results []SearchResult
	seen := map[string]bool{}

	// Strategy 1: Parse the Vue/Blade <archivio records="[...JSON...]"> component
	var unescapedJSON string
	if recordsAttr, exists := doc.Find("archivio").Attr("records"); exists && recordsAttr != "" {
		unescapedJSON = recordsAttr
	} else if m := archivioRecordsPattern.FindStringSubmatch(body); len(m) > 1 {
		unescapedJSON = html.UnescapeString(m[1])
	}

	if unescapedJSON != "" {
		var records []archivioRecord
		if err := json.Unmarshal([]byte(unescapedJSON), &records); err == nil {
			for _, rec := range records {
				name := ""
				if rec.Title != nil && strings.TrimSpace(*rec.Title) != "" {
					name = strings.TrimSpace(*rec.Title)
				} else if rec.TitleEng != nil && strings.TrimSpace(*rec.TitleEng) != "" {
					name = strings.TrimSpace(*rec.TitleEng)
				} else if rec.TitleIt != nil && strings.TrimSpace(*rec.TitleIt) != "" {
					name = strings.TrimSpace(*rec.TitleIt)
				} else {
					name = strings.Title(strings.ReplaceAll(rec.Slug, "-", " "))
				}

				animeURL := fmt.Sprintf("https://%s/anime/%d-%s", host, rec.ID, rec.Slug)
				if seen[animeURL] {
					continue
				}
				seen[animeURL] = true

				results = append(results, SearchResult{
					Name:  name,
					URL:   animeURL,
					Cover: rec.ImageURL,
				})
			}
			return results, nil
		}
	}

	// Strategy 2: Fallback to HTML anchor elements with /anime/ links
	doc.Find("a[href*='/anime/']").Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok || href == "" {
			return
		}

		u := href
		if !strings.HasPrefix(u, "http") {
			u = fmt.Sprintf("https://%s%s", host, href)
		}
		if seen[u] {
			return
		}

		name := strings.TrimSpace(a.Find(".title, .name, h2, h3").First().Text())
		if name == "" {
			name = strings.TrimSpace(a.AttrOr("title", ""))
		}
		if name == "" {
			name = strings.TrimSpace(a.Find("img").AttrOr("alt", ""))
		}
		if name == "" {
			match := animeNamePattern.FindStringSubmatch(u)
			if len(match) > 1 {
				name = strings.Title(strings.ReplaceAll(match[1], "-", " "))
			}
		}

		cover := ""
		if src, ok := a.Find("img").First().Attr("src"); ok {
			cover = src
		}

		if name != "" {
			seen[u] = true
			results = append(results, SearchResult{
				Name:  name,
				URL:   u,
				Cover: cover,
			})
		}
	})

	return results, nil
}

type infoAPIResponse struct {
	ID            int     `json:"id"`
	Title         *string `json:"title"`
	TitleEng      *string `json:"title_eng"`
	TitleIt       *string `json:"title_it"`
	Slug          string  `json:"slug"`
	Plot          string  `json:"plot"`
	Type          string  `json:"type"`
	Status        string  `json:"status"`
	Date          string  `json:"date"`
	Studio        string  `json:"studio"`
	ImageURL      string  `json:"imageurl"`
	ImageURLCover string  `json:"imageurl_cover"`
	Cover         *string `json:"cover"`
	Score         string  `json:"score"`
	EpisodesCount int     `json:"episodes_count"`
	MalID         any     `json:"mal_id"`
	AnilistID     any     `json:"anilist_id"`
	Genres        any     `json:"genres"`
	Related       any     `json:"related"`
}

// GetAnime extracts metadata for a given anime using AnimeUnity's info_api or HTML fallback.
func (c *AnimeUnityClient) GetAnime(ctx context.Context, pageURL string) (*Anime, error) {
	pageURL = validateURL(pageURL)
	host := extractHostDomain(pageURL)

	match := animeURLPattern.FindStringSubmatch(pageURL)
	var animeID string
	if len(match) > 1 {
		animeID = match[1]
	}

	canonicalPageURL := pageURL
	if animeID != "" {
		canonicalPageURL = fmt.Sprintf("https://%s/anime/%s", host, animeID)
	}

	// First try info_api
	if animeID != "" {
		infoAPIURL := fmt.Sprintf("https://%s/info_api/%s", host, animeID)
		body, err := c.fetchWithRetries(ctx, infoAPIURL, canonicalPageURL, 3)
		if err == nil && body != "" {
			var resp infoAPIResponse
			if err := json.Unmarshal([]byte(body), &resp); err == nil && resp.ID > 0 {
				title := ""
				if resp.Title != nil && strings.TrimSpace(*resp.Title) != "" {
					title = strings.TrimSpace(*resp.Title)
				} else if resp.TitleEng != nil && strings.TrimSpace(*resp.TitleEng) != "" {
					title = strings.TrimSpace(*resp.TitleEng)
				} else if resp.TitleIt != nil && strings.TrimSpace(*resp.TitleIt) != "" {
					title = strings.TrimSpace(*resp.TitleIt)
				} else {
					title = strings.Title(strings.ReplaceAll(resp.Slug, "-", " "))
				}

				cover := resp.ImageURL
				if cover == "" && resp.Cover != nil {
					cover = *resp.Cover
				}
				if cover == "" {
					cover = resp.ImageURLCover
				}

				infoMap := map[string]any{
					"ID":             resp.ID,
					"Type":           resp.Type,
					"Status":         resp.Status,
					"Date":           resp.Date,
					"Studio":         resp.Studio,
					"Score":          resp.Score,
					"EpisodesCount":  resp.EpisodesCount,
					"episodes_count": resp.EpisodesCount,
					"Genres":         resp.Genres,
				}

				return &Anime{
					Name:        title,
					URL:         canonicalPageURL,
					Cover:       cover,
					Description: strings.TrimSpace(resp.Plot),
					Info:        infoMap,
				}, nil
			}
		}
	}

	// Fallback: fetch HTML page directly and parse with BeautifulSoup equivalent (goquery)
	body, err := c.fetchWithRetries(ctx, canonicalPageURL, c.BaseURL, 3)
	if err != nil {
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Extract title using the 4 strategies from Python Crawler.extract_anime_name
	title := ""
	if titleNode := doc.Find("h1.title").First(); titleNode.Length() > 0 {
		title = strings.TrimSpace(titleNode.Text())
	}
	if title == "" {
		if titleTag := doc.Find("title").First(); titleTag.Length() > 0 {
			title = extractNameFromTitleTag(titleTag.Text())
		}
	}
	if title == "" {
		if ogTitle, ok := doc.Find(`meta[property="og:title"]`).First().Attr("content"); ok {
			title = strings.TrimSpace(ogTitle)
		}
	}
	if title == "" && animeID != "" {
		matchName := animeNamePattern.FindStringSubmatch("/anime/" + animeID)
		if len(matchName) > 1 {
			title = strings.Title(strings.ReplaceAll(matchName[1], "-", " "))
		}
	}

	cover := ""
	if ogImg, ok := doc.Find(`meta[property="og:image"]`).First().Attr("content"); ok {
		cover = ogImg
	}

	desc := ""
	if ogDesc, ok := doc.Find(`meta[property="og:description"]`).First().Attr("content"); ok {
		desc = strings.TrimSpace(ogDesc)
	}

	return &Anime{
		Name:        title,
		URL:         canonicalPageURL,
		Cover:       cover,
		Description: desc,
		Info:        map[string]any{},
	}, nil
}

type unityEpisodeInfo struct {
	ID        int    `json:"id"`
	AnimeID   int    `json:"anime_id"`
	Number    string `json:"number"`
	Link      string `json:"link"`
	SCWSID    int    `json:"scws_id"`
	FileName  string `json:"file_name"`
	CreatedAt string `json:"created_at"`
}

type unityEpisodesBatchResponse struct {
	EpisodesCount int                `json:"episodes_count"`
	Episodes      []unityEpisodeInfo `json:"episodes"`
	Error         string             `json:"error"`
}

// GetEpisodes fetches episodes for an anime and resolves their playable stream URLs.
func (c *AnimeUnityClient) GetEpisodes(ctx context.Context, anime *Anime, numbers ...string) ([]Episode, error) {
	animeURL := validateURL(anime.URL)
	host := extractHostDomain(animeURL)

	match := animeURLPattern.FindStringSubmatch(animeURL)
	if len(match) < 2 {
		return nil, &UnityError{Message: "Formato URL anime non valido: " + animeURL}
	}
	animeID := match[1]
	apiURL := fmt.Sprintf("https://%s/info_api/%s", host, animeID)

	// Retrieve total episodes count
	numEpisodes := 0
	if anime.Info != nil {
		if count, ok := anime.Info["episodes_count"].(int); ok && count > 0 {
			numEpisodes = count
		}
	}

	if numEpisodes == 0 {
		body, err := c.fetchWithRetries(ctx, apiURL, animeURL, 3)
		if err != nil {
			return nil, err
		}
		var meta struct {
			EpisodesCount int `json:"episodes_count"`
		}
		if err := json.Unmarshal([]byte(body), &meta); err == nil && meta.EpisodesCount > 0 {
			numEpisodes = meta.EpisodesCount
		}
	}

	if numEpisodes == 0 {
		numEpisodes = 1
	}

	allowed := map[string]bool{}
	for _, n := range numbers {
		allowed[strings.TrimSpace(n)] = true
	}

	var allEpInfos []unityEpisodeInfo

	// Fast path: if the caller requested a single specific episode number, calculate its batch
	if len(numbers) == 1 {
		if reqNum, err := strconv.Atoi(numbers[0]); err == nil && reqNum >= 1 {
			batchStart := ((reqNum - 1) / defaultRangeSpan) * defaultRangeSpan
			batchEnd := batchStart + defaultRangeSpan
			if batchEnd > numEpisodes+1 {
				batchEnd = numEpisodes + 1
			}

			batchURL := fmt.Sprintf("%s/0?start_range=%d&end_range=%d", apiURL, batchStart, batchEnd)
			body, err := c.fetchWithRetries(ctx, batchURL, animeURL, 3)
			if err == nil {
				var bResp unityEpisodesBatchResponse
				if err := json.Unmarshal([]byte(body), &bResp); err == nil {
					for _, ep := range bResp.Episodes {
						if ep.Number == numbers[0] {
							allEpInfos = append(allEpInfos, ep)
							break
						}
					}
				}
			}
		}
	}

	// If not found via fast path or multiple episodes requested, fetch all batches
	if len(allEpInfos) == 0 {
		episodesBatchURL := fmt.Sprintf("%s/0", apiURL)
		startRange := 0
		endRange := numEpisodes + 1

		type batchRange struct {
			start int
			end   int
		}
		var batches []batchRange
		for bStart := startRange; bStart < endRange; bStart += defaultRangeSpan {
			bEnd := bStart + defaultRangeSpan
			if bEnd > endRange {
				bEnd = endRange
			}
			batches = append(batches, batchRange{start: bStart, end: bEnd})
		}

		batchResults := make([][]unityEpisodeInfo, len(batches))
		conc := c.Concurrency
		if conc < 1 {
			conc = 4
		}
		sem := make(chan struct{}, conc)
		var wg sync.WaitGroup

		for idx, b := range batches {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, rng batchRange) {
				defer wg.Done()
				defer func() { <-sem }()

				target := fmt.Sprintf("%s?start_range=%d&end_range=%d", episodesBatchURL, rng.start, rng.end)
				body, err := c.fetchWithRetries(ctx, target, animeURL, 3)
				if err != nil {
					return
				}
				var bResp unityEpisodesBatchResponse
				if err := json.Unmarshal([]byte(body), &bResp); err == nil && len(bResp.Episodes) > 0 {
					batchResults[i] = bResp.Episodes
				}
			}(idx, b)
		}
		wg.Wait()

		for _, bList := range batchResults {
			allEpInfos = append(allEpInfos, bList...)
		}
	}

	// Filter by allowed numbers if specified
	var episodes []Episode
	for _, info := range allEpInfos {
		epNumStr := info.Number
		if len(allowed) > 0 && !allowed[epNumStr] {
			continue
		}

		epIDStr := strconv.Itoa(info.ID)
		embedURL := fmt.Sprintf("https://%s/embed-url/%s", host, epIDStr)
		episodes = append(episodes, Episode{
			Number: epNumStr,
			ID:     epIDStr,
			Links: []EpisodeLink{
				{
					ID:         epIDStr,
					URL:        embedURL,
					ServerName: "VixCloud",
				},
			},
		})
	}

	if len(episodes) == 0 {
		return nil, &UnityError{Message: "Nessun episodio trovato per " + anime.Name}
	}

	// Concurrently resolve embed and stream URLs
	c.resolveStreams(ctx, host, episodes)

	return episodes, nil
}

// resolveStreams concurrently resolves each episode's embed URL into a playable stream URL.
func (c *AnimeUnityClient) resolveStreams(ctx context.Context, host string, episodes []Episode) {
	conc := c.Concurrency
	if conc < 1 {
		conc = 4
	}
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	for i := range episodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(ep *Episode) {
			defer wg.Done()
			defer func() { <-sem }()

			embedURL := fmt.Sprintf("https://%s/embed-url/%s", host, ep.ID)
			streamURL, err := c.resolveSingleStream(ctx, host, embedURL)
			if err != nil {
				ep.Err = err
				return
			}

			ep.StreamURL = streamURL
			if len(ep.Links) > 0 {
				ep.Links[0].StreamURL = streamURL
			}
		}(&episodes[i])
	}
	wg.Wait()
}

// resolveSingleStream fetches the embed URL and extracts the underlying video / download URL.
func (c *AnimeUnityClient) resolveSingleStream(ctx context.Context, host string, embedURL string) (string, error) {
	// Step 1: GET embed-url endpoint (returns e.g. https://vixcloud.cc/embed/...)
	referer := fmt.Sprintf("https://%s/", host)
	respText, err := c.fetchWithRetries(ctx, embedURL, referer, 3)
	if err != nil {
		return "", err
	}
	vixURL := strings.TrimSpace(respText)

	// If the response is already a direct video URL, return it
	if streamURLPattern.MatchString(vixURL) {
		return vixURL, nil
	}

	// Step 2: GET the VixCloud embed player page
	if strings.HasPrefix(vixURL, "http://") || strings.HasPrefix(vixURL, "https://") {
		vixHTML, err := c.fetchWithRetries(ctx, vixURL, referer, 3)
		if err == nil && vixHTML != "" {
			// Check for window.downloadUrl (direct MP4)
			if dlLink := extractDownloadLink(vixHTML); dlLink != "" {
				return dlLink, nil
			}

			// Check for window.masterPlaylist / window.streams with token, expires & h
			if streamURL := extractStreamFromVixHTML(vixHTML, vixURL); streamURL != "" {
				return streamURL, nil
			}

			// Fallback: search for m3u8 playlist or mp4 in the page scripts
			if m := streamURLPattern.FindString(vixHTML); m != "" {
				return strings.ReplaceAll(m, `\/`, `/`), nil
			}

			// Fallback: check for playlist url
			if p := playlistURLPattern.FindString(vixHTML); p != "" {
				return strings.ReplaceAll(p, `\/`, `/`), nil
			}
		}

		// Fallback to embed player URL itself if extraction couldn't find a direct link
		return vixURL, nil
	}

	return "", &UnityError{Message: "Impossibile trovare uno stream valido per " + embedURL}
}

// extractStreamFromVixHTML parses window.masterPlaylist (or window.streams) and builds the signed playlist URL.
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

	// 2. Fallback to window.streams = [{"url": "https:\/\/vixcloud.cc\/playlist\/..."}]
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
