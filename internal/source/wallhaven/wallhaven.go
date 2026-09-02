// Package wallhaven implements a termbg source.Source backed by the
// wallhaven.cc public API (https://wallhaven.cc/help/api). It supports
// passing through arbitrary search filters supported by that API (q,
// categories, purity, sorting, order, topRange, atleast, resolutions,
// ratios, colors, seed, ...) via a raw query-parameter map, and caches
// downloaded wallpapers locally so repeated Next() calls (e.g. after a
// restart) don't always require a network round trip.
package wallhaven

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/kmaltcev/termbg/internal/source"
)

func init() {
	source.Register("wallhaven", newFromConfig)
}

const envAPIKeyVar = "TERMBG_WALLHAVEN_API_KEY"

// apiBase is a var (not const) so tests can point it at a local
// httptest server instead of the real wallhaven.cc API.
var apiBase = "https://wallhaven.cc/api/v1/search"

// Source fetches wallpapers from wallhaven.cc matching a set of search
// parameters, downloads them, and serves them as local file paths.
type Source struct {
	params   url.Values
	apiKey   string
	cacheDir string
	client   *http.Client

	mu      sync.Mutex
	results []wallpaper
	idx     int
}

type wallpaper struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type searchResponse struct {
	Data []wallpaper `json:"data"`
	Meta searchMeta  `json:"meta"`
}

type searchMeta struct {
	LastPage int `json:"last_page"`
}

func newFromConfig(cfg map[string]any) (source.Source, error) {
	params := url.Values{}
	if raw, ok := cfg["params"].(map[string]any); ok {
		for k, v := range raw {
			params.Set(k, fmt.Sprintf("%v", v))
		}
	}
	// Backward/alternate config shape: map[string]string.
	if raw, ok := cfg["params"].(map[string]string); ok {
		for k, v := range raw {
			params.Set(k, v)
		}
	}

	apiKey := os.Getenv(envAPIKeyVar)
	if apiKey == "" {
		apiKey, _ = cfg["api_key"].(string)
	}

	cacheDir, _ := cfg["cache_dir"].(string)
	if cacheDir == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("wallhaven source: determining cache dir: %w", err)
		}
		cacheDir = filepath.Join(userCache, "termbg", "wallhaven")
	}

	return New(params, apiKey, cacheDir)
}

// New creates a wallhaven source with the given raw search params (see
// https://wallhaven.cc/help/api), an optional API key (required for
// NSFW purity and to use the account's own default filters), and a
// local directory used to cache downloaded images.
func New(params url.Values, apiKey, cacheDir string) (*Source, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("wallhaven source: creating cache dir %s: %w", cacheDir, err)
	}
	return &Source{
		params:   params,
		apiKey:   apiKey,
		cacheDir: cacheDir,
		client:   &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (s *Source) Name() string { return "wallhaven" }

// Next fetches (if needed) a page of search results matching the
// configured filters and returns the local path to the next wallpaper
// in that page, downloading it first if not already cached.
func (s *Source) Next(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.idx >= len(s.results) {
		if err := s.fetchPage(ctx); err != nil {
			return "", err
		}
		s.idx = 0
	}
	if len(s.results) == 0 {
		return "", fmt.Errorf("wallhaven source: no results for the configured search filters")
	}

	w := s.results[s.idx]
	s.idx++
	return s.download(ctx, w)
}

func (s *Source) fetchPage(ctx context.Context) error {
	first, err := s.search(ctx, 0)
	if err != nil {
		return err
	}

	results := first.Data
	// wallhaven only returns 24 results per page; without paging,
	// every fetch would return the exact same fixed page-1 set (small
	// and, for non-random sort orders like sorting=views, completely
	// deterministic), making filters/tags feel "stuck" on the same
	// handful of images no matter how many times Next() is called.
	// Once we know how many pages actually match the configured
	// filters, jump to a random one so repeated calls explore the
	// full result set instead of only ever recycling page 1.
	if first.Meta.LastPage > 1 {
		page := 1 + rand.Intn(first.Meta.LastPage)
		if page > 1 {
			if other, err := s.search(ctx, page); err == nil && len(other.Data) > 0 {
				results = other.Data
			}
			// If the random page request fails or comes back empty
			// (wallhaven caps how deep pagination actually works),
			// silently fall back to the page-1 results already in
			// hand rather than erroring the whole rotation out.
		}
	}

	s.results = results
	// termbg is typically invoked as a short-lived CLI process, so
	// each Next() call usually starts from a fresh idx=0 on a newly
	// fetched page. Shuffle so that "first result" isn't always the
	// same wallpaper (e.g. sorting=views/date_added returns the exact
	// same ordering on every fetch) — every call should genuinely
	// pick a random wallpaper among the matching results.
	rand.Shuffle(len(s.results), func(i, j int) {
		s.results[i], s.results[j] = s.results[j], s.results[i]
	})
	return nil
}

// search issues a single wallhaven /search request. page == 0 omits
// the "page" query parameter entirely (wallhaven defaults to page 1).
func (s *Source) search(ctx context.Context, page int) (*searchResponse, error) {
	q := url.Values{}
	for k, v := range s.params {
		q[k] = v
	}
	if s.apiKey != "" {
		q.Set("apikey", s.apiKey)
	}
	// A random seed keeps sorting=random pagination stable across
	// pages if the caller ever wants to page forward; harmless
	// otherwise since wallhaven ignores unknown/irrelevant params.
	if q.Get("sorting") == "random" && q.Get("seed") == "" {
		q.Set("seed", randomSeed())
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}

	reqURL := apiBase + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("wallhaven source: building request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wallhaven source: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("wallhaven source: unexpected status %s: %s", resp.Status, body)
	}

	var parsed searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("wallhaven source: decoding response: %w", err)
	}
	return &parsed, nil
}

func (s *Source) download(ctx context.Context, w wallpaper) (string, error) {
	dest := filepath.Join(s.cacheDir, w.ID+filepath.Ext(w.Path))
	if _, err := os.Stat(dest); err == nil {
		return dest, nil // already cached
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.Path, nil)
	if err != nil {
		return "", fmt.Errorf("wallhaven source: building download request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wallhaven source: downloading %s: %w", w.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wallhaven source: unexpected status downloading %s: %s", w.Path, resp.Status)
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("wallhaven source: creating %s: %w", tmp, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", fmt.Errorf("wallhaven source: writing %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("wallhaven source: closing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", fmt.Errorf("wallhaven source: renaming %s: %w", tmp, err)
	}
	return dest, nil
}

func randomSeed() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
