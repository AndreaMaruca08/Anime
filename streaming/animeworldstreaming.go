package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Error struct {
	Message    string
	StatusCode int
}

func (e *Error) Error() string { return "AnimeWorldError: " + e.Message }

// AnimeWorldClient is an alias to Client.
type AnimeWorldClient = Client

func NewAnimeWorldClient() *AnimeWorldClient {
	return NewClient()
}

// ---------- Client ----------

type Client struct {
	BaseURL     string
	HTTP        *http.Client
	UserAgent   string
	Concurrency int // richieste parallele per risolvere gli stream
}

var _ Provider = (*Client)(nil)

func (c *Client) Name() string {
	return "AnimeWorld"
}

func NewClient() *Client {
	return &Client{
		BaseURL:     "https://www.animeworld.ac",
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		UserAgent:   "free_anime/1.0",
		Concurrency: 5,
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	body, err := c.get(ctx, c.BaseURL+"/search?"+url.Values{"keyword": {query}}.Encode())
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	seen := map[string]bool{}

	// Se la struttura contiene .film-list .item, estraiamo direttamente da ciascun item
	items := doc.Find(".film-list .item")
	if items.Length() > 0 {
		items.Each(func(_ int, item *goquery.Selection) {
			nameLink := item.Find("a.name").First()
			posterLink := item.Find("a.poster").First()

			href, ok := nameLink.Attr("href")
			if !ok || href == "" {
				href, ok = posterLink.Attr("href")
			}
			if !ok || !strings.Contains(href, "/play/") {
				return
			}

			u := c.abs(href)
			if seen[u] {
				return
			}
			seen[u] = true

			name := strings.TrimSpace(nameLink.Text())
			if name == "" {
				name = strings.TrimSpace(posterLink.Find("img").AttrOr("alt", ""))
			}
			if name == "" {
				name = strings.TrimSpace(nameLink.AttrOr("data-jtitle", ""))
			}

			cover := ""
			if src, ok := posterLink.Find("img").First().Attr("src"); ok {
				cover = c.abs(src)
			}

			if name != "" {
				results = append(results, SearchResult{Name: name, URL: u, Cover: cover})
			}
		})
		return results, nil
	}

	// Fallback generico per altre possibili pagine
	doc.Find("a").Each(func(_ int, a *goquery.Selection) {
		href, ok := a.Attr("href")
		if !ok || !strings.Contains(href, "/play/") {
			return
		}
		u := c.abs(href)
		if seen[u] {
			return
		}

		name := strings.TrimSpace(a.Find("h2, h3, .title, .name").First().Text())
		if name == "" {
			name = strings.TrimSpace(a.Find("img").AttrOr("alt", ""))
		}
		if name == "" {
			name = strings.TrimSpace(a.AttrOr("title", ""))
		}
		if name == "" || strings.EqualFold(name, "dub") || strings.EqualFold(name, "sub") {
			return
		}

		seen[u] = true
		cover := ""
		if src, ok := a.Find("img").First().Attr("src"); ok {
			cover = c.abs(src)
		}
		results = append(results, SearchResult{Name: name, URL: u, Cover: cover})
	})
	return results, nil
}

func (c *Client) GetAnime(ctx context.Context, pageURL string) (*Anime, error) {
	body, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(doc.Find("#anime-title").First().Text())
	if title == "" {
		return nil, &Error{Message: "anime non trovato o pagina non valida"}
	}
	if strings.Contains(body, "Errore 404") {
		return nil, &Error{Message: "anime non trovato", StatusCode: 404}
	}

	info := map[string]any{}
	row := doc.Find(".info .row").First()
	labels := row.Find("dt")
	values := row.Find("dd")
	for i := 0; i < labels.Length() && i < values.Length(); i++ {
		key := strings.TrimSpace(strings.ReplaceAll(labels.Eq(i).Text(), ":", ""))
		val := strings.TrimSpace(values.Eq(i).Text())
		if strings.Contains(val, ",") {
			parts := strings.Split(val, ",")
			for j := range parts {
				parts[j] = strings.TrimSpace(parts[j])
			}
			info[key] = parts
		} else {
			info[key] = val
		}
	}

	cover := ""
	if src, ok := doc.Find("#thumbnail-watch img").First().Attr("src"); ok {
		cover = c.abs(src)
	}
	return &Anime{
		Name:        title,
		URL:         c.abs(pageURL),
		Cover:       cover,
		Description: strings.TrimSpace(doc.Find(".desc").First().Text()),
		Info:        info,
	}, nil
}

func (c *Client) GetEpisodes(ctx context.Context, anime *Anime, numbers ...string) ([]Episode, error) {
	body, err := c.get(ctx, anime.URL)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	if doc.Find("li.episode > a").Length() == 0 {
		return nil, &Error{Message: "episodi non disponibili per " + anime.Name}
	}

	allowed := map[string]bool{}
	for _, n := range numbers {
		allowed[n] = true
	}

	var order []string
	byID := map[string]*Episode{}

	doc.Find("[class*='server'][data-name]").Each(func(_ int, server *goquery.Selection) {
		serverID := -1
		if n, err := strconv.Atoi(server.AttrOr("data-name", "")); err == nil {
			serverID = n
		}
		serverName := strings.TrimSpace(server.Find(".server-name").First().Text())
		if serverName == "" {
			serverName = server.AttrOr("data-server", fmt.Sprintf("Server %d", serverID))
		}
		server.Find("li.episode > a").Each(func(_ int, a *goquery.Selection) {
			number, ok1 := a.Attr("data-episode-num")
			epID, ok2 := a.Attr("data-episode-id")
			dataID, ok3 := a.Attr("data-id")
			href, ok4 := a.Attr("href")
			if !ok1 || !ok2 || !ok3 || !ok4 {
				return
			}
			if len(allowed) > 0 && !allowed[number] {
				return
			}
			ep, exists := byID[epID]
			if !exists {
				ep = &Episode{Number: number, ID: epID}
				byID[epID] = ep
				order = append(order, epID)
			}
			ep.Links = append(ep.Links, EpisodeLink{
				ID: dataID, URL: c.abs(href), ServerID: serverID, ServerName: serverName,
			})
		})
	})

	episodes := make([]Episode, 0, len(order))
	for _, id := range order {
		episodes = append(episodes, *byID[id])
	}
	c.resolveStreams(ctx, episodes)
	return episodes, nil
}

func (c *Client) resolveStreams(ctx context.Context, episodes []Episode) {
	conc := c.Concurrency
	if conc < 1 {
		conc = 1
	}
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	for i := range episodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(ep *Episode) {
			defer wg.Done()
			defer func() { <-sem }()
			var lastErr error
			for j := range ep.Links {
				u, err := c.streamURL(ctx, ep.Links[j].ID)
				if err != nil {
					lastErr = err
					continue
				}
				ep.Links[j].StreamURL = u
				if ep.StreamURL == "" {
					ep.StreamURL = u
				}
			}
			if ep.StreamURL == "" {
				if lastErr == nil {
					lastErr = &Error{Message: "nessun server disponibile per l'episodio " + ep.Number}
				}
				ep.Err = lastErr
			}
		}(&episodes[i])
	}
	wg.Wait()
}

func (c *Client) streamURL(ctx context.Context, linkID string) (string, error) {
	q := url.Values{"id": {linkID}, "alt": {"0"}}
	body, err := c.get(ctx, c.BaseURL+"/api/episode/info?"+q.Encode())
	if err != nil {
		return "", err
	}
	var decoded any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return "", err
	}
	u := findURL(decoded)
	if u == "" {
		return "", &Error{Message: "il server non ha restituito uno stream valido"}
	}
	return c.abs(u), nil
}

func findURL(v any) string {
	switch t := v.(type) {
	case string:
		if u, err := url.Parse(t); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			return t
		}
	case map[string]any:
		for _, k := range []string{"url", "file", "source", "src", "grabber"} {
			if r := findURL(t[k]); r != "" {
				return r
			}
		}
		for _, item := range t {
			if r := findURL(item); r != "" {
				return r
			}
		}
	case []any:
		for _, item := range t {
			if r := findURL(item); r != "" {
				return r
			}
		}
	}
	return ""
}

var reTrailingSlash = regexp.MustCompile(`/$`)

func (c *Client) get(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", &Error{Message: "errore di rete: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &Error{Message: fmt.Sprintf("richiesta fallita (%d)", resp.StatusCode), StatusCode: resp.StatusCode}
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Client) abs(ref string) string {
	base, err := url.Parse(reTrailingSlash.ReplaceAllString(c.BaseURL, ""))
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(r).String()
}
